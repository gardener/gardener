// Copyright (c) 2020 SAP SE or an SAP affiliate company.All rights reserved.This file is licensed under the Apache Software License, v.2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package shoot_test

import (
	"context"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/apis/seedmanagement/encoding"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakeclientmap "github.com/gardener/gardener/pkg/client/kubernetes/clientmap/fake"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/features"
	configv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	. "github.com/gardener/gardener/pkg/gardenlet/controller/shoot"
	gardenerlogger "github.com/gardener/gardener/pkg/logger"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("SeedRegistrationReconciler", func() {
	const (
		name      = "test"
		namespace = "garden"
	)

	var (
		ctx context.Context
		c   client.Client

		reconciler reconcile.Reconciler
		request    reconcile.Request

		newShoot            func(string) *gardencorev1beta1.Shoot
		expectedManagedSeed *seedmanagementv1alpha1.ManagedSeed
	)

	BeforeEach(func() {
		ctx = context.Background()
		c = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()
		clientMap := fakeclientmap.NewClientMap().AddRuntimeClient(keys.ForGarden(), c)

		newShoot = func(useAsSeed string) *gardencorev1beta1.Shoot {
			var annotations map[string]string
			if useAsSeed != "" {
				annotations = map[string]string{
					v1beta1constants.AnnotationShootUseAsSeed: useAsSeed,
				}
			}
			return &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
					Labels: map[string]string{
						"foo": "bar",
					},
					Annotations: annotations,
				},
			}
		}
		request = reconcile.Request{NamespacedName: client.ObjectKey{Namespace: namespace, Name: name}}

		reconciler = NewSeedRegistrationReconciler(clientMap, record.NewFakeRecorder(1), gardenerlogger.NewNopLogger())
	})

	AfterEach(func() {
		managedSeed := &seedmanagementv1alpha1.ManagedSeed{}
		err := c.Get(ctx, request.NamespacedName, managedSeed)
		if expectedManagedSeed == nil {
			Expect(err).To(BeNotFoundError())
		} else {
			Expect(err).NotTo(HaveOccurred())

			// fake client sets metadata
			managedSeed.ResourceVersion = ""
			managedSeed.SetGroupVersionKind(schema.GroupVersionKind{})

			// fake client does some back and forth marshalling on get, which removes the trailing newline in the config's
			// raw extension, that the json marshaller put there earlier
			// hence, add back the trailing newline in order to compare successfully
			if gardenlet := managedSeed.Spec.Gardenlet; gardenlet != nil {
				if len(gardenlet.Config.Raw) != 0 {
					gardenlet.Config.Raw = append(gardenlet.Config.Raw, '\n')
				}
			}

			Expect(managedSeed).To(DeepEqual(expectedManagedSeed))
		}
	})

	Describe("#Reconcile", func() {
		It("should do nothing if shoot is gone", func() {
			Expect(reconciler.Reconcile(ctx, request)).To(Equal(reconcile.Result{}))
		})

		Context("reconcile", func() {
			Context("no-gardenlet", func() {
				It("should create the correct ManagedSeed resource, all defaults", func() {
					shoot := newShoot("true,no-gardenlet")
					Expect(c.Create(ctx, shoot)).To(Succeed())

					expectedManagedSeed = &seedmanagementv1alpha1.ManagedSeed{
						ObjectMeta: metav1.ObjectMeta{
							Name:      name,
							Namespace: namespace,
							OwnerReferences: []metav1.OwnerReference{
								*metav1.NewControllerRef(shoot, gardencorev1beta1.SchemeGroupVersion.WithKind("Shoot")),
							},
						},
						Spec: seedmanagementv1alpha1.ManagedSeedSpec{
							Shoot: &seedmanagementv1alpha1.Shoot{
								Name: name,
							},
							SeedTemplate: &gardencorev1beta1.SeedTemplate{
								ObjectMeta: metav1.ObjectMeta{
									Labels: shoot.Labels,
								},
								Spec: gardencorev1beta1.SeedSpec{
									Backup: &gardencorev1beta1.SeedBackup{},
									SecretRef: &corev1.SecretReference{
										Name:      "seed-" + name,
										Namespace: v1beta1constants.GardenNamespace,
									},
									Settings: &gardencorev1beta1.SeedSettings{
										ExcessCapacityReservation: &gardencorev1beta1.SeedSettingExcessCapacityReservation{
											Enabled: true,
										},
										Scheduling: &gardencorev1beta1.SeedSettingScheduling{
											Visible: true,
										},
										ShootDNS: &gardencorev1beta1.SeedSettingShootDNS{
											Enabled: true,
										},
									},
								},
							},
						},
					}

					Expect(reconciler.Reconcile(ctx, request)).To(Equal(reconcile.Result{}))
				})

				It("should create the correct ManagedSeed resource, all custom", func() {
					shoot := newShoot("true,no-gardenlet,disable-dns,disable-capacity-reservation,protected,invisible,minimumVolumeSize=20Gi,apiServer.replicas=2,apiServer.autoscaler.minReplicas=2,apiServer.autoscaler.maxReplicas=5,blockCIDRs=169.254.169.254/32,shootDefaults.pods=100.96.0.0/11,shootDefaults.services=100.64.0.0/13,backup.provider=gcp,backup.region=europe-north1,loadBalancerServices.annotations.foo=bar,ingress.controller.kind=nginx")
					Expect(c.Create(ctx, shoot)).To(Succeed())

					expectedManagedSeed = &seedmanagementv1alpha1.ManagedSeed{
						ObjectMeta: metav1.ObjectMeta{
							Name:      name,
							Namespace: namespace,
							OwnerReferences: []metav1.OwnerReference{
								*metav1.NewControllerRef(shoot, gardencorev1beta1.SchemeGroupVersion.WithKind("Shoot")),
							},
						},
						Spec: seedmanagementv1alpha1.ManagedSeedSpec{
							Shoot: &seedmanagementv1alpha1.Shoot{
								Name: name,
							},
							SeedTemplate: &gardencorev1beta1.SeedTemplate{
								ObjectMeta: metav1.ObjectMeta{
									Labels: shoot.Labels,
								},
								Spec: gardencorev1beta1.SeedSpec{
									Backup: &gardencorev1beta1.SeedBackup{
										Provider: "gcp",
										Region:   pointer.String("europe-north1"),
									},
									Networks: gardencorev1beta1.SeedNetworks{
										ShootDefaults: &gardencorev1beta1.ShootNetworks{
											Pods:     pointer.String("100.96.0.0/11"),
											Services: pointer.String("100.64.0.0/13"),
										},
										BlockCIDRs: []string{"169.254.169.254/32"},
									},
									SecretRef: &corev1.SecretReference{
										Name:      "seed-" + name,
										Namespace: v1beta1constants.GardenNamespace,
									},
									Taints: []gardencorev1beta1.SeedTaint{
										{
											Key: gardencorev1beta1.SeedTaintProtected,
										},
									},
									Volume: &gardencorev1beta1.SeedVolume{
										MinimumSize: quantityPtr(resource.MustParse("20Gi")),
									},
									Settings: &gardencorev1beta1.SeedSettings{
										ExcessCapacityReservation: &gardencorev1beta1.SeedSettingExcessCapacityReservation{
											Enabled: false,
										},
										Scheduling: &gardencorev1beta1.SeedSettingScheduling{
											Visible: false,
										},
										ShootDNS: &gardencorev1beta1.SeedSettingShootDNS{
											Enabled: false,
										},
										LoadBalancerServices: &gardencorev1beta1.SeedSettingLoadBalancerServices{
											Annotations: map[string]string{"foo": "bar"},
										},
									},
									Ingress: &gardencorev1beta1.Ingress{
										Controller: gardencorev1beta1.IngressController{
											Kind: "nginx",
										},
									},
								},
							},
						},
					}

					Expect(reconciler.Reconcile(ctx, request)).To(Equal(reconcile.Result{}))
				})
			})

			Context("gardenlet", func() {
				It("should create the correct ManagedSeed resource, all defaults", func() {
					shoot := newShoot("true")
					Expect(c.Create(ctx, shoot)).To(Succeed())

					expectedManagedSeed = &seedmanagementv1alpha1.ManagedSeed{
						ObjectMeta: metav1.ObjectMeta{
							Name:      name,
							Namespace: namespace,
							OwnerReferences: []metav1.OwnerReference{
								*metav1.NewControllerRef(shoot, gardencorev1beta1.SchemeGroupVersion.WithKind("Shoot")),
							},
						},
						Spec: seedmanagementv1alpha1.ManagedSeedSpec{
							Shoot: &seedmanagementv1alpha1.Shoot{
								Name: name,
							},
							Gardenlet: &seedmanagementv1alpha1.Gardenlet{
								Config: rawExtension(&configv1alpha1.GardenletConfiguration{
									TypeMeta: metav1.TypeMeta{
										APIVersion: "gardenlet.config.gardener.cloud/v1alpha1",
										Kind:       "GardenletConfiguration",
									},
									Resources: &configv1alpha1.ResourcesConfiguration{
										Capacity: corev1.ResourceList{
											gardencorev1beta1.ResourceShoots: resource.MustParse("250"),
										},
									},
									SeedConfig: &configv1alpha1.SeedConfig{
										SeedTemplate: gardencorev1beta1.SeedTemplate{
											ObjectMeta: metav1.ObjectMeta{
												Labels: shoot.Labels,
											},
											Spec: gardencorev1beta1.SeedSpec{
												Backup: &gardencorev1beta1.SeedBackup{},
												Settings: &gardencorev1beta1.SeedSettings{
													ExcessCapacityReservation: &gardencorev1beta1.SeedSettingExcessCapacityReservation{
														Enabled: true,
													},
													Scheduling: &gardencorev1beta1.SeedSettingScheduling{
														Visible: true,
													},
													ShootDNS: &gardencorev1beta1.SeedSettingShootDNS{
														Enabled: true,
													},
												},
											},
										},
									},
								}),
								Bootstrap:       bootstrapPtr(seedmanagementv1alpha1.BootstrapToken),
								MergeWithParent: pointer.Bool(true),
							},
						},
					}

					Expect(reconciler.Reconcile(ctx, request)).To(Equal(reconcile.Result{}))
				})

				It("should create the correct ManagedSeed resource, all custom", func() {
					shoot := newShoot("true,disable-dns,disable-capacity-reservation,protected,invisible,minimumVolumeSize=20Gi,apiServer.replicas=2,apiServer.autoscaler.minReplicas=2,apiServer.autoscaler.maxReplicas=5,blockCIDRs=169.254.169.254/32,shootDefaults.pods=100.96.0.0/11,shootDefaults.services=100.64.0.0/13,backup.provider=gcp,backup.region=europe-north1,use-serviceaccount-bootstrapping,with-secret-ref,featureGates.Logging=false,resources.capacity.shoots=100,loadBalancerServices.annotations.foo=bar,ingress.controller.kind=nginx")
					Expect(c.Create(ctx, shoot)).To(Succeed())

					expectedManagedSeed = &seedmanagementv1alpha1.ManagedSeed{
						ObjectMeta: metav1.ObjectMeta{
							Name:      name,
							Namespace: namespace,
							OwnerReferences: []metav1.OwnerReference{
								*metav1.NewControllerRef(shoot, gardencorev1beta1.SchemeGroupVersion.WithKind("Shoot")),
							},
						},
						Spec: seedmanagementv1alpha1.ManagedSeedSpec{
							Shoot: &seedmanagementv1alpha1.Shoot{
								Name: name,
							},
							Gardenlet: &seedmanagementv1alpha1.Gardenlet{
								Config: rawExtension(&configv1alpha1.GardenletConfiguration{
									TypeMeta: metav1.TypeMeta{
										APIVersion: "gardenlet.config.gardener.cloud/v1alpha1",
										Kind:       "GardenletConfiguration",
									},
									Resources: &configv1alpha1.ResourcesConfiguration{
										Capacity: corev1.ResourceList{
											gardencorev1beta1.ResourceShoots: resource.MustParse("100"),
										},
									},
									FeatureGates: map[string]bool{
										string(features.Logging): false,
									},
									SeedConfig: &configv1alpha1.SeedConfig{
										SeedTemplate: gardencorev1beta1.SeedTemplate{
											ObjectMeta: metav1.ObjectMeta{
												Labels: shoot.Labels,
											},
											Spec: gardencorev1beta1.SeedSpec{
												Backup: &gardencorev1beta1.SeedBackup{
													Provider: "gcp",
													Region:   pointer.String("europe-north1"),
												},
												Networks: gardencorev1beta1.SeedNetworks{
													ShootDefaults: &gardencorev1beta1.ShootNetworks{
														Pods:     pointer.String("100.96.0.0/11"),
														Services: pointer.String("100.64.0.0/13"),
													},
													BlockCIDRs: []string{"169.254.169.254/32"},
												},
												SecretRef: &corev1.SecretReference{
													Name:      "seed-" + name,
													Namespace: v1beta1constants.GardenNamespace,
												},
												Taints: []gardencorev1beta1.SeedTaint{
													{
														Key: gardencorev1beta1.SeedTaintProtected,
													},
												},
												Volume: &gardencorev1beta1.SeedVolume{
													MinimumSize: quantityPtr(resource.MustParse("20Gi")),
												},
												Settings: &gardencorev1beta1.SeedSettings{
													ExcessCapacityReservation: &gardencorev1beta1.SeedSettingExcessCapacityReservation{
														Enabled: false,
													},
													Scheduling: &gardencorev1beta1.SeedSettingScheduling{
														Visible: false,
													},
													ShootDNS: &gardencorev1beta1.SeedSettingShootDNS{
														Enabled: false,
													},
													LoadBalancerServices: &gardencorev1beta1.SeedSettingLoadBalancerServices{
														Annotations: map[string]string{"foo": "bar"},
													},
												},
												Ingress: &gardencorev1beta1.Ingress{
													Controller: gardencorev1beta1.IngressController{
														Kind: "nginx",
													},
												},
											},
										},
									},
								}),
								Bootstrap:       bootstrapPtr(seedmanagementv1alpha1.BootstrapServiceAccount),
								MergeWithParent: pointer.Bool(true),
							},
						},
					}

					Expect(reconciler.Reconcile(ctx, request)).To(Equal(reconcile.Result{}))
				})
			})
		})

		Context("delete", func() {
			It("should delete the ManagedSeed resource", func() {
				shoot := newShoot("")
				Expect(c.Create(ctx, shoot)).To(Succeed())

				// create pre-existing ManagedSeed
				managedSeed := &seedmanagementv1alpha1.ManagedSeed{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name,
						Namespace: namespace,
						OwnerReferences: []metav1.OwnerReference{
							*metav1.NewControllerRef(shoot, gardencorev1beta1.SchemeGroupVersion.WithKind("Shoot")),
						},
					},
				}
				Expect(c.Create(ctx, managedSeed)).To(Succeed())

				Expect(reconciler.Reconcile(ctx, request)).To(Equal(reconcile.Result{}))

				expectedManagedSeed = nil
			})
		})
	})
})

func rawExtension(cfg *configv1alpha1.GardenletConfiguration) runtime.RawExtension {
	re, err := encoding.EncodeGardenletConfiguration(cfg)
	Expect(err).NotTo(HaveOccurred())
	re.Object = nil
	return *re
}

func quantityPtr(v resource.Quantity) *resource.Quantity { return &v }

func bootstrapPtr(v seedmanagementv1alpha1.Bootstrap) *seedmanagementv1alpha1.Bootstrap { return &v }

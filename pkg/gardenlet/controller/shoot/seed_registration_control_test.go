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
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/apis/seedmanagement/encoding"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	mockclientmap "github.com/gardener/gardener/pkg/client/kubernetes/clientmap/mock"
	mockkubernetes "github.com/gardener/gardener/pkg/client/kubernetes/mock"
	"github.com/gardener/gardener/pkg/features"
	configv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	. "github.com/gardener/gardener/pkg/gardenlet/controller/shoot"
	gardenerlogger "github.com/gardener/gardener/pkg/logger"
	mockrecord "github.com/gardener/gardener/pkg/mock/client-go/tools/record"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	name      = "test"
	namespace = "garden"
)

var _ = Describe("DefaultSeedRegistrationControl", func() {
	var (
		ctrl *gomock.Controller

		gardenClient *mockkubernetes.MockInterface
		clientMap    *mockclientmap.MockClientMap
		recorder     *mockrecord.MockEventRecorder
		c            *mockclient.MockClient

		seedRegistrationControl SeedRegistrationControlInterface

		ctx context.Context

		shoot func(string) *gardencorev1beta1.Shoot
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		gardenClient = mockkubernetes.NewMockInterface(ctrl)
		clientMap = mockclientmap.NewMockClientMap(ctrl)
		recorder = mockrecord.NewMockEventRecorder(ctrl)
		c = mockclient.NewMockClient(ctrl)

		clientMap.EXPECT().GetClient(context.TODO(), keys.ForGarden()).Return(gardenClient, nil)
		gardenClient.EXPECT().Client().Return(c).AnyTimes()

		seedRegistrationControl = NewDefaultSeedRegistrationControl(clientMap, recorder, gardenerlogger.NewNopLogger())

		ctx = context.TODO()

		shoot = func(useAsSeed string) *gardencorev1beta1.Shoot {
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
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#Reconcile", func() {
		Context("reconcile", func() {
			Context("no-gardenlet", func() {
				It("should create the correct ManagedSeed resource, all defaults", func() {
					shoot := shoot("true,no-gardenlet")
					shootedSeed, err := gardencorev1beta1helper.ReadShootedSeed(shoot)
					Expect(err).ToNot(HaveOccurred())

					c.EXPECT().Get(ctx, kutil.Key(namespace, name), gomock.AssignableToTypeOf(&seedmanagementv1alpha1.ManagedSeed{})).DoAndReturn(
						func(_ context.Context, _ client.ObjectKey, ms *seedmanagementv1alpha1.ManagedSeed) error {
							return apierrors.NewNotFound(seedmanagementv1alpha1.Resource("managedseed"), name)
						},
					).Times(2)
					c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&seedmanagementv1alpha1.ManagedSeed{})).DoAndReturn(
						func(_ context.Context, ms *seedmanagementv1alpha1.ManagedSeed, _ ...client.CreateOption) error {
							Expect(ms).To(Equal(&seedmanagementv1alpha1.ManagedSeed{
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
							}))
							return nil
						},
					)

					result, err := seedRegistrationControl.Reconcile(ctx, shoot, shootedSeed)
					Expect(err).ToNot(HaveOccurred())
					Expect(result).To(Equal(reconcile.Result{}))
				})

				It("should create the correct ManagedSeed resource, all custom", func() {
					shoot := shoot("true,no-gardenlet,disable-dns,disable-capacity-reservation,protected,invisible,minimumVolumeSize=20Gi,apiServer.replicas=2,apiServer.autoscaler.minReplicas=2,apiServer.autoscaler.maxReplicas=5,blockCIDRs=169.254.169.254/32,shootDefaults.pods=100.96.0.0/11,shootDefaults.services=100.64.0.0/13,backup.provider=gcp,backup.region=europe-north1,loadBalancerServices.annotations.foo=bar,ingress.controller.kind=nginx")
					shootedSeed, err := gardencorev1beta1helper.ReadShootedSeed(shoot)
					Expect(err).ToNot(HaveOccurred())

					c.EXPECT().Get(ctx, kutil.Key(namespace, name), gomock.AssignableToTypeOf(&seedmanagementv1alpha1.ManagedSeed{})).DoAndReturn(
						func(_ context.Context, _ client.ObjectKey, ms *seedmanagementv1alpha1.ManagedSeed) error {
							return apierrors.NewNotFound(seedmanagementv1alpha1.Resource("managedseed"), name)
						},
					).Times(2)
					c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&seedmanagementv1alpha1.ManagedSeed{})).DoAndReturn(
						func(_ context.Context, ms *seedmanagementv1alpha1.ManagedSeed, _ ...client.CreateOption) error {
							Expect(ms).To(Equal(&seedmanagementv1alpha1.ManagedSeed{
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
							}))
							return nil
						},
					)

					result, err := seedRegistrationControl.Reconcile(ctx, shoot, shootedSeed)
					Expect(err).ToNot(HaveOccurred())
					Expect(result).To(Equal(reconcile.Result{}))
				})
			})

			Context("gardenlet", func() {
				It("should create the correct ManagedSeed resource, all defaults", func() {
					shoot := shoot("true")
					shootedSeed, err := gardencorev1beta1helper.ReadShootedSeed(shoot)
					Expect(err).ToNot(HaveOccurred())

					c.EXPECT().Get(ctx, kutil.Key(namespace, name), gomock.AssignableToTypeOf(&seedmanagementv1alpha1.ManagedSeed{})).DoAndReturn(
						func(_ context.Context, _ client.ObjectKey, ms *seedmanagementv1alpha1.ManagedSeed) error {
							return apierrors.NewNotFound(seedmanagementv1alpha1.Resource("managedseed"), name)
						},
					).Times(2)
					c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&seedmanagementv1alpha1.ManagedSeed{})).DoAndReturn(
						func(_ context.Context, ms *seedmanagementv1alpha1.ManagedSeed, _ ...client.CreateOption) error {
							Expect(ms).To(Equal(&seedmanagementv1alpha1.ManagedSeed{
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
							}))
							return nil
						},
					)

					result, err := seedRegistrationControl.Reconcile(ctx, shoot, shootedSeed)
					Expect(err).ToNot(HaveOccurred())
					Expect(result).To(Equal(reconcile.Result{}))
				})

				It("should create the correct ManagedSeed resource, all custom", func() {
					shoot := shoot("true,disable-dns,disable-capacity-reservation,protected,invisible,minimumVolumeSize=20Gi,apiServer.replicas=2,apiServer.autoscaler.minReplicas=2,apiServer.autoscaler.maxReplicas=5,blockCIDRs=169.254.169.254/32,shootDefaults.pods=100.96.0.0/11,shootDefaults.services=100.64.0.0/13,backup.provider=gcp,backup.region=europe-north1,use-serviceaccount-bootstrapping,with-secret-ref,featureGates.Logging=false,resources.capacity.shoots=100,loadBalancerServices.annotations.foo=bar,ingress.controller.kind=nginx")
					shootedSeed, err := gardencorev1beta1helper.ReadShootedSeed(shoot)
					Expect(err).ToNot(HaveOccurred())

					c.EXPECT().Get(ctx, kutil.Key(namespace, name), gomock.AssignableToTypeOf(&seedmanagementv1alpha1.ManagedSeed{})).DoAndReturn(
						func(_ context.Context, _ client.ObjectKey, ms *seedmanagementv1alpha1.ManagedSeed) error {
							return apierrors.NewNotFound(seedmanagementv1alpha1.Resource("managedseed"), name)
						},
					).Times(2)
					c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&seedmanagementv1alpha1.ManagedSeed{})).DoAndReturn(
						func(_ context.Context, ms *seedmanagementv1alpha1.ManagedSeed, _ ...client.CreateOption) error {
							Expect(ms).To(Equal(&seedmanagementv1alpha1.ManagedSeed{
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
							}))
							return nil
						},
					)

					result, err := seedRegistrationControl.Reconcile(ctx, shoot, shootedSeed)
					Expect(err).ToNot(HaveOccurred())
					Expect(result).To(Equal(reconcile.Result{}))
				})
			})
		})

		Context("delete", func() {
			It("should delete the ManagedSeed resource", func() {
				shoot := shoot("")
				shootedSeed, err := gardencorev1beta1helper.ReadShootedSeed(shoot)
				Expect(err).ToNot(HaveOccurred())
				Expect(shootedSeed).To(BeNil())

				c.EXPECT().Get(ctx, kutil.Key(namespace, name), gomock.AssignableToTypeOf(&seedmanagementv1alpha1.ManagedSeed{})).DoAndReturn(
					func(_ context.Context, _ client.ObjectKey, ms *seedmanagementv1alpha1.ManagedSeed) error {
						*ms = seedmanagementv1alpha1.ManagedSeed{
							ObjectMeta: metav1.ObjectMeta{
								Name:      name,
								Namespace: namespace,
								OwnerReferences: []metav1.OwnerReference{
									*metav1.NewControllerRef(shoot, gardencorev1beta1.SchemeGroupVersion.WithKind("Shoot")),
								},
							},
						}
						return nil
					},
				)
				c.EXPECT().Delete(ctx, gomock.AssignableToTypeOf(&seedmanagementv1alpha1.ManagedSeed{})).DoAndReturn(
					func(_ context.Context, ms *seedmanagementv1alpha1.ManagedSeed, _ ...client.DeleteOption) error {
						Expect(ms.Name).To(Equal(name))
						Expect(ms.Namespace).To(Equal(namespace))
						return nil
					},
				)

				result, err := seedRegistrationControl.Reconcile(ctx, shoot, shootedSeed)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))
			})
		})
	})
})

func rawExtension(cfg *configv1alpha1.GardenletConfiguration) runtime.RawExtension {
	re, _ := encoding.EncodeGardenletConfiguration(cfg)
	return *re
}

func quantityPtr(v resource.Quantity) *resource.Quantity { return &v }

func bootstrapPtr(v seedmanagementv1alpha1.Bootstrap) *seedmanagementv1alpha1.Bootstrap { return &v }

// Copyright (c) 2021 SAP SE or an SAP affiliate company.All rights reserved.This file is licensed under the Apache Software License, v.2 except as noted otherwise in the LICENSE file
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

package managedseed

import (
	"context"
	"path/filepath"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	mockclientmap "github.com/gardener/gardener/pkg/client/kubernetes/clientmap/mock"
	mockkubernetes "github.com/gardener/gardener/pkg/client/kubernetes/mock"
	configv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	mockmanagedseed "github.com/gardener/gardener/pkg/gardenlet/controller/managedseed/mock"
	gardenerlogger "github.com/gardener/gardener/pkg/logger"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	dnsv1alpha1 "github.com/gardener/external-dns-management/pkg/apis/dns/v1alpha1"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/component-base/config/v1alpha1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	seedName             = "test-seed"
	secretBindingName    = "test-secret-binding"
	secretName           = "test-secret"
	kubeconfigSecretName = "test.kubeconfig"
	backupSecretName     = "test-backup-secret"
	seedSecretName       = "test-seed-secret"
)

var _ = Describe("Actuator", func() {
	var (
		ctrl *gomock.Controller

		gardenClient      *mockkubernetes.MockInterface
		seedClient        *mockkubernetes.MockInterface
		shootClient       *mockkubernetes.MockInterface
		clientMap         *mockclientmap.MockClientMap
		vh                *mockmanagedseed.MockValuesHelper
		gc                *mockclient.MockClient
		sec               *mockclient.MockClient
		shc               *mockclient.MockClient
		shootChartApplier *mockkubernetes.MockChartApplier

		actuator Actuator

		ctx context.Context

		managedSeed      *seedmanagementv1alpha1.ManagedSeed
		shoot            *gardencorev1beta1.Shoot
		secretBinding    *gardencorev1beta1.SecretBinding
		secret           *corev1.Secret
		kubeconfigSecret *corev1.Secret
		seed             *gardencorev1beta1.Seed

		seedTemplate *gardencorev1beta1.SeedTemplate
		gardenlet    *seedmanagementv1alpha1.Gardenlet

		mergedDeployment      *seedmanagementv1alpha1.GardenletDeployment
		mergedGardenletConfig *configv1alpha1.GardenletConfiguration
		gardenletChartValues  map[string]interface{}
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		gardenClient = mockkubernetes.NewMockInterface(ctrl)
		seedClient = mockkubernetes.NewMockInterface(ctrl)
		shootClient = mockkubernetes.NewMockInterface(ctrl)
		clientMap = mockclientmap.NewMockClientMap(ctrl)
		vh = mockmanagedseed.NewMockValuesHelper(ctrl)
		gc = mockclient.NewMockClient(ctrl)
		sec = mockclient.NewMockClient(ctrl)
		shc = mockclient.NewMockClient(ctrl)
		shootChartApplier = mockkubernetes.NewMockChartApplier(ctrl)

		gardenClient.EXPECT().Client().Return(gc).AnyTimes()
		gardenClient.EXPECT().RESTConfig().Return(&rest.Config{}).AnyTimes()
		seedClient.EXPECT().Client().Return(sec).AnyTimes()
		shootClient.EXPECT().Client().Return(shc).AnyTimes()
		shootClient.EXPECT().ChartApplier().Return(shootChartApplier).AnyTimes()

		actuator = newActuator(gardenClient, clientMap, vh, gardenerlogger.NewNopLogger())

		ctx = context.TODO()

		managedSeed = &seedmanagementv1alpha1.ManagedSeed{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Spec: seedmanagementv1alpha1.ManagedSeedSpec{
				Shoot: &seedmanagementv1alpha1.Shoot{
					Name: name,
				},
			},
		}
		shoot = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Spec: gardencorev1beta1.ShootSpec{
				SecretBindingName: secretBindingName,
				SeedName:          pointer.StringPtr(seedName),
			},
			Status: gardencorev1beta1.ShootStatus{
				TechnicalID: "shoot--" + namespace + "--" + name,
			},
		}
		secretBinding = &gardencorev1beta1.SecretBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretBindingName,
				Namespace: namespace,
			},
			SecretRef: corev1.SecretReference{
				Name:      secretName,
				Namespace: namespace,
			},
		}
		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretName,
				Namespace: namespace,
			},
			Data: map[string][]byte{
				"foo": []byte("bar"),
			},
		}
		kubeconfigSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      kubeconfigSecretName,
				Namespace: namespace,
			},
			Data: map[string][]byte{
				"kubeconfig": []byte("kubeconfig"),
			},
		}
		seed = &gardencorev1beta1.Seed{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
				Labels: map[string]string{
					"foo": "bar",
				},
				Annotations: map[string]string{
					"bar": "baz",
				},
			},
			Spec: gardencorev1beta1.SeedSpec{
				Backup: &gardencorev1beta1.SeedBackup{
					SecretRef: corev1.SecretReference{
						Name:      backupSecretName,
						Namespace: namespace,
					},
				},
				SecretRef: &corev1.SecretReference{
					Name:      seedSecretName,
					Namespace: namespace,
				},
				Settings: &gardencorev1beta1.SeedSettings{
					VerticalPodAutoscaler: &gardencorev1beta1.SeedSettingVerticalPodAutoscaler{
						Enabled: true,
					},
				},
				Ingress: &gardencorev1beta1.Ingress{},
			},
		}

		seedTemplate = &gardencorev1beta1.SeedTemplate{
			ObjectMeta: metav1.ObjectMeta{
				Labels:      seed.Labels,
				Annotations: seed.Annotations,
			},
			Spec: seed.Spec,
		}
		gardenlet = &seedmanagementv1alpha1.Gardenlet{
			Deployment: &seedmanagementv1alpha1.GardenletDeployment{
				ReplicaCount:         pointer.Int32Ptr(1),
				RevisionHistoryLimit: pointer.Int32Ptr(1),
				Image: &seedmanagementv1alpha1.Image{
					PullPolicy: pullPolicyPtr(corev1.PullIfNotPresent),
				},
				VPA: pointer.BoolPtr(true),
			},
			Config: runtime.RawExtension{
				Object: &configv1alpha1.GardenletConfiguration{
					TypeMeta: metav1.TypeMeta{
						APIVersion: configv1alpha1.SchemeGroupVersion.String(),
						Kind:       "GardenletConfiguration",
					},
					SeedConfig: &configv1alpha1.SeedConfig{
						SeedTemplate: gardencorev1beta1.SeedTemplate{
							ObjectMeta: metav1.ObjectMeta{
								Labels:      seed.Labels,
								Annotations: seed.Annotations,
							},
							Spec: seed.Spec,
						},
					},
				},
			},
			Bootstrap:       bootstrapPtr(seedmanagementv1alpha1.BootstrapToken),
			MergeWithParent: pointer.BoolPtr(true),
		}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	var (
		expectCreateGardenNamespace = func() {
			shc.EXPECT().Get(ctx, kutil.Key(v1beta1constants.GardenNamespace), gomock.AssignableToTypeOf(&corev1.Namespace{})).DoAndReturn(
				func(_ context.Context, _ client.ObjectKey, _ *corev1.Namespace) error {
					return apierrors.NewNotFound(corev1.Resource("namespace"), name)
				},
			)
			shc.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&corev1.Namespace{})).DoAndReturn(
				func(_ context.Context, ns *corev1.Namespace) error {
					Expect(ns.Name).To(Equal(v1beta1constants.GardenNamespace))
					return nil
				},
			)
		}
		expectEnsureGardenNamespaceDeleted = func() {
			// Delete garden namespace
			shc.EXPECT().Delete(ctx, gomock.AssignableToTypeOf(&corev1.Namespace{})).DoAndReturn(
				func(_ context.Context, ns *corev1.Namespace) error {
					Expect(ns.Name).To(Equal(v1beta1constants.GardenNamespace))
					return nil
				},
			)

			// Check if garden namespace still exists
			shc.EXPECT().Get(ctx, kutil.Key(v1beta1constants.GardenNamespace), gomock.AssignableToTypeOf(&corev1.Namespace{})).DoAndReturn(
				func(_ context.Context, _ client.ObjectKey, _ *corev1.Namespace) error {
					return apierrors.NewNotFound(corev1.Resource("namespace"), v1beta1constants.GardenNamespace)
				},
			)
		}

		expectCheckSeedSpec = func() {
			// Check if the shoot namespace in the seed contains a vpa-admission-controller deployment
			sec.EXPECT().Get(ctx, kutil.Key(shoot.Status.TechnicalID, "vpa-admission-controller"), gomock.AssignableToTypeOf(&appsv1.Deployment{})).DoAndReturn(
				func(_ context.Context, _ client.ObjectKey, _ *appsv1.Deployment) error {
					return apierrors.NewNotFound(appsv1.Resource("deployment"), "vpa-admission-controller")
				},
			)

			// Check if the shoot namespace in the seed contains an ingress DNSEntry
			sec.EXPECT().Get(ctx, kutil.Key(shoot.Status.TechnicalID, common.ShootDNSIngressName), gomock.AssignableToTypeOf(&dnsv1alpha1.DNSEntry{})).DoAndReturn(
				func(_ context.Context, _ client.ObjectKey, _ *dnsv1alpha1.DNSEntry) error {
					return apierrors.NewNotFound(dnsv1alpha1.Resource("dnsentry"), common.ShootDNSIngressName)
				},
			)
		}

		expectCreateSeedSecrets = func() {
			// Get shoot secret
			gc.EXPECT().Get(ctx, kutil.Key(namespace, secretBindingName), gomock.AssignableToTypeOf(&gardencorev1beta1.SecretBinding{})).DoAndReturn(
				func(_ context.Context, _ client.ObjectKey, sb *gardencorev1beta1.SecretBinding) error {
					*sb = *secretBinding
					return nil
				},
			)
			gc.EXPECT().Get(ctx, kutil.Key(namespace, secretName), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(
				func(_ context.Context, _ client.ObjectKey, s *corev1.Secret) error {
					*s = *secret
					return nil
				},
			)

			// Create backup secret
			gc.EXPECT().Get(ctx, kutil.Key(namespace, backupSecretName), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(
				func(_ context.Context, _ client.ObjectKey, _ *corev1.Secret) error {
					return apierrors.NewNotFound(corev1.Resource("secret"), backupSecretName)
				},
			).Times(2)
			gc.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(
				func(_ context.Context, s *corev1.Secret) error {
					Expect(s).To(Equal(&corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      backupSecretName,
							Namespace: namespace,
							OwnerReferences: []metav1.OwnerReference{
								*metav1.NewControllerRef(managedSeed, seedmanagementv1alpha1.SchemeGroupVersion.WithKind("ManagedSeed")),
							},
						},
						Data: secret.Data,
						Type: corev1.SecretTypeOpaque,
					}))
					return nil
				},
			)

			// Create seed secret
			gc.EXPECT().Get(ctx, kutil.Key(namespace, kubeconfigSecretName), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(
				func(_ context.Context, _ client.ObjectKey, s *corev1.Secret) error {
					*s = *kubeconfigSecret
					return nil
				},
			)
			gc.EXPECT().Get(ctx, kutil.Key(namespace, seedSecretName), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(
				func(_ context.Context, _ client.ObjectKey, _ *corev1.Secret) error {
					return apierrors.NewNotFound(corev1.Resource("secret"), seedSecretName)
				},
			)
			gc.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(
				func(_ context.Context, s *corev1.Secret) error {
					Expect(s).To(Equal(&corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      seedSecretName,
							Namespace: namespace,
							OwnerReferences: []metav1.OwnerReference{
								*metav1.NewControllerRef(managedSeed, seedmanagementv1alpha1.SchemeGroupVersion.WithKind("ManagedSeed")),
							},
						},
						Data: map[string][]byte{
							"foo":        []byte("bar"),
							"kubeconfig": []byte("kubeconfig"),
						},
						Type: corev1.SecretTypeOpaque,
					}))
					return nil
				},
			)
		}

		expectEnsureSeedSecretsDeleted = func() {
			// Delete backup secret
			gc.EXPECT().Get(ctx, kutil.Key(namespace, backupSecretName), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(
				func(_ context.Context, _ client.ObjectKey, s *corev1.Secret) error {
					*s = corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      seedSecretName,
							Namespace: namespace,
							OwnerReferences: []metav1.OwnerReference{
								*metav1.NewControllerRef(managedSeed, seedmanagementv1alpha1.SchemeGroupVersion.WithKind("ManagedSeed")),
							},
						},
					}
					return nil
				},
			)
			gc.EXPECT().Delete(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(
				func(_ context.Context, s *corev1.Secret) error {
					Expect(s.Name).To(Equal(backupSecretName))
					Expect(s.Namespace).To(Equal(namespace))
					return nil
				},
			)

			// Delete seed secret
			gc.EXPECT().Delete(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(
				func(_ context.Context, s *corev1.Secret) error {
					Expect(s.Name).To(Equal(seedSecretName))
					Expect(s.Namespace).To(Equal(namespace))
					return nil
				},
			)

			// Check if the backup secret still exists
			gc.EXPECT().Get(ctx, kutil.Key(namespace, backupSecretName), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(
				func(_ context.Context, _ client.ObjectKey, _ *corev1.Secret) error {
					return apierrors.NewNotFound(corev1.Resource("secret"), backupSecretName)
				},
			)

			// Check if the seed secret still exists
			gc.EXPECT().Get(ctx, kutil.Key(namespace, seedSecretName), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(
				func(_ context.Context, _ client.ObjectKey, _ *corev1.Secret) error {
					return apierrors.NewNotFound(corev1.Resource("secret"), seedSecretName)
				},
			)
		}

		expectCreateSeed = func() {
			gc.EXPECT().Get(ctx, kutil.Key(name), gomock.AssignableToTypeOf(&gardencorev1beta1.Seed{})).DoAndReturn(
				func(_ context.Context, _ client.ObjectKey, _ *gardencorev1beta1.Seed) error {
					return apierrors.NewNotFound(gardencorev1beta1.Resource("seed"), name)
				},
			)
			gc.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&gardencorev1beta1.Seed{})).DoAndReturn(
				func(_ context.Context, s *gardencorev1beta1.Seed) error {
					Expect(s).To(Equal(&gardencorev1beta1.Seed{
						ObjectMeta: metav1.ObjectMeta{
							Name: name,
							Labels: utils.MergeStringMaps(seed.Labels, map[string]string{
								v1beta1constants.GardenRole: v1beta1constants.GardenRoleSeed,
							}),
							Annotations: seed.Annotations,
							OwnerReferences: []metav1.OwnerReference{
								*metav1.NewControllerRef(managedSeed, seedmanagementv1alpha1.SchemeGroupVersion.WithKind("ManagedSeed")),
							},
						},
						Spec: seed.Spec,
					}))
					return nil
				},
			)
		}

		expectEnsureSeedDeleted = func() {
			// Delete the seed
			gc.EXPECT().Delete(ctx, gomock.AssignableToTypeOf(&gardencorev1beta1.Seed{})).DoAndReturn(
				func(_ context.Context, s *gardencorev1beta1.Seed) error {
					Expect(s.Name).To(Equal(name))
					return nil
				},
			)

			// Check if the seed still exists
			gc.EXPECT().Get(ctx, kutil.Key(name), gomock.AssignableToTypeOf(&gardencorev1beta1.Seed{})).DoAndReturn(
				func(_ context.Context, _ client.ObjectKey, s *gardencorev1beta1.Seed) error {
					return apierrors.NewNotFound(gardencorev1beta1.Resource("seed"), name)
				},
			)
		}

		expectMergeWithParent = func() {
			mergedDeployment = managedSeed.Spec.Gardenlet.Deployment.DeepCopy()
			mergedDeployment.Image = &seedmanagementv1alpha1.Image{
				Repository: pointer.StringPtr("repository"),
				Tag:        pointer.StringPtr("tag"),
				PullPolicy: pullPolicyPtr(corev1.PullIfNotPresent),
			}

			mergedGardenletConfig = managedSeed.Spec.Gardenlet.Config.Object.(*configv1alpha1.GardenletConfiguration).DeepCopy()
			mergedGardenletConfig.GardenClientConnection = &configv1alpha1.GardenClientConnection{
				ClientConnectionConfiguration: v1alpha1.ClientConnectionConfiguration{
					Kubeconfig: "kubeconfig",
				},
			}
			mergedGardenletConfig.SeedSelector = &metav1.LabelSelector{}

			vh.EXPECT().MergeGardenletDeployment(managedSeed.Spec.Gardenlet.Deployment, shoot).Return(mergedDeployment, nil)
			vh.EXPECT().MergeGardenletConfiguration(managedSeed.Spec.Gardenlet.Config.Object).Return(mergedGardenletConfig, nil)
		}

		expectPrepareGardenClientConnection = func() {
			// Check if kubeconfig secret exists
			shc.EXPECT().Get(ctx, kutil.Key(v1beta1constants.GardenNamespace, gardenletKubeconfigSecretName), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(
				func(_ context.Context, _ client.ObjectKey, _ *corev1.Secret) error {
					return apierrors.NewNotFound(corev1.Resource("secret"), gardenletKubeconfigSecretName)
				},
			)

			// Create bootstrap token secret
			gc.EXPECT().Get(ctx, kutil.Key(metav1.NamespaceSystem, "bootstrap-token-9f86d0"), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(
				func(_ context.Context, _ client.ObjectKey, _ *corev1.Secret) error {
					return apierrors.NewNotFound(corev1.Resource("secret"), "bootstrap-token-9f86d0")
				},
			).Times(3)
			gc.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(
				func(_ context.Context, s *corev1.Secret) error {
					Expect(s.Name).To(Equal("bootstrap-token-9f86d0"))
					Expect(s.Namespace).To(Equal(metav1.NamespaceSystem))
					Expect(s.Type).To(Equal(corev1.SecretTypeBootstrapToken))
					Expect(s.Data).To(HaveKeyWithValue("token-id", []byte("9f86d0")))
					Expect(s.Data).To(HaveKey("token-secret"))
					Expect(s.Data).To(HaveKeyWithValue("usage-bootstrap-signing", []byte("true")))
					Expect(s.Data).To(HaveKeyWithValue("usage-bootstrap-authentication", []byte("true")))
					return nil
				},
			)
		}

		expectGetGardenletChartValues = func(withBootstrap bool) {
			gardenletChartValues = map[string]interface{}{"foo": "bar"}

			vh.EXPECT().GetGardenletChartValues(mergedDeployment, gomock.AssignableToTypeOf(&configv1alpha1.GardenletConfiguration{}), gomock.AssignableToTypeOf("")).DoAndReturn(
				func(_ *seedmanagementv1alpha1.GardenletDeployment, gc *configv1alpha1.GardenletConfiguration, _ string) (map[string]interface{}, error) {
					if withBootstrap {
						Expect(gc.GardenClientConnection.Kubeconfig).To(Equal(""))
						Expect(gc.GardenClientConnection.KubeconfigSecret).To(Equal(&corev1.SecretReference{
							Name:      gardenletKubeconfigSecretName,
							Namespace: v1beta1constants.GardenNamespace,
						}))
						Expect(gc.GardenClientConnection.BootstrapKubeconfig).To(Equal(&corev1.SecretReference{
							Name:      gardenletKubeconfigBootstrapSecretName,
							Namespace: v1beta1constants.GardenNamespace,
						}))
					} else {
						Expect(gc.GardenClientConnection.Kubeconfig).To(Equal("kubeconfig"))
						Expect(gc.GardenClientConnection.KubeconfigSecret).To(BeNil())
						Expect(gc.GardenClientConnection.BootstrapKubeconfig).To(BeNil())
					}
					Expect(gc.SeedConfig.SeedTemplate).To(Equal(gardencorev1beta1.SeedTemplate{
						ObjectMeta: metav1.ObjectMeta{
							Name:        name,
							Labels:      seed.Labels,
							Annotations: seed.Annotations,
						},
						Spec: seed.Spec,
					}))
					Expect(gc.SeedSelector).To(BeNil())
					return gardenletChartValues, nil
				},
			)
		}

		expectApplyGardenletChart = func() {
			shootChartApplier.EXPECT().Apply(ctx, filepath.Join(common.ChartPath, "gardener", "gardenlet"), v1beta1constants.GardenNamespace, "gardenlet", kubernetes.Values(gardenletChartValues)).Return(nil)
		}

		expectDeleteGardenletChart = func() {
			shootChartApplier.EXPECT().Delete(ctx, filepath.Join(common.ChartPath, "gardener", "gardenlet"), v1beta1constants.GardenNamespace, "gardenlet", kubernetes.Values(gardenletChartValues)).Return(nil)
		}
	)

	Describe("#Reconcile", func() {
		Context("seed template", func() {
			BeforeEach(func() {
				managedSeed.Spec.SeedTemplate = seedTemplate
			})

			It("should reconcile the ManagedSeed creation or update", func() {
				clientMap.EXPECT().GetClient(ctx, keys.ForShoot(shoot)).Return(shootClient, nil)
				clientMap.EXPECT().GetClient(ctx, keys.ForSeedWithName(seedName)).Return(seedClient, nil)

				expectCreateGardenNamespace()
				expectCheckSeedSpec()
				expectCreateSeedSecrets()
				expectCreateSeed()

				err := actuator.Reconcile(ctx, managedSeed, shoot)
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("gardenlet", func() {
			BeforeEach(func() {
				managedSeed.Spec.Gardenlet = gardenlet
			})

			It("should reconcile the ManagedSeed creation or update (with bootstrap)", func() {
				clientMap.EXPECT().GetClient(ctx, keys.ForShoot(shoot)).Return(shootClient, nil)
				clientMap.EXPECT().GetClient(ctx, keys.ForSeedWithName(seedName)).Return(seedClient, nil)

				expectCreateGardenNamespace()
				expectCheckSeedSpec()
				expectCreateSeedSecrets()
				expectMergeWithParent()
				expectPrepareGardenClientConnection()
				expectGetGardenletChartValues(true)
				expectApplyGardenletChart()

				err := actuator.Reconcile(ctx, managedSeed, shoot)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should reconcile the ManagedSeed creation or update (without bootstrap)", func() {
				managedSeed.Spec.Gardenlet.Bootstrap = bootstrapPtr(seedmanagementv1alpha1.BootstrapNone)

				clientMap.EXPECT().GetClient(ctx, keys.ForShoot(shoot)).Return(shootClient, nil)
				clientMap.EXPECT().GetClient(ctx, keys.ForSeedWithName(seedName)).Return(seedClient, nil)

				expectCreateGardenNamespace()
				expectCheckSeedSpec()
				expectCreateSeedSecrets()
				expectMergeWithParent()
				expectGetGardenletChartValues(false)
				expectApplyGardenletChart()

				err := actuator.Reconcile(ctx, managedSeed, shoot)
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})

	Describe("#Delete", func() {
		Context("seed template", func() {
			BeforeEach(func() {
				managedSeed.Spec.SeedTemplate = seedTemplate
			})

			It("should reconcile the ManagedSeed deletion", func() {
				clientMap.EXPECT().GetClient(ctx, keys.ForShoot(shoot)).Return(shootClient, nil)

				expectEnsureSeedDeleted()
				expectEnsureSeedSecretsDeleted()
				expectEnsureGardenNamespaceDeleted()

				err := actuator.Delete(ctx, managedSeed, shoot)
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("gardenlet", func() {
			BeforeEach(func() {
				managedSeed.Spec.Gardenlet = gardenlet
			})

			It("should reconcile the ManagedSeed deletion", func() {
				clientMap.EXPECT().GetClient(ctx, keys.ForShoot(shoot)).Return(shootClient, nil)

				expectEnsureSeedDeleted()
				expectEnsureSeedSecretsDeleted()
				expectMergeWithParent()
				expectPrepareGardenClientConnection()
				expectGetGardenletChartValues(true)
				expectDeleteGardenletChart()
				expectEnsureGardenNamespaceDeleted()

				err := actuator.Delete(ctx, managedSeed, shoot)
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})
})

func pullPolicyPtr(v corev1.PullPolicy) *corev1.PullPolicy { return &v }

func bootstrapPtr(v seedmanagementv1alpha1.Bootstrap) *seedmanagementv1alpha1.Bootstrap { return &v }

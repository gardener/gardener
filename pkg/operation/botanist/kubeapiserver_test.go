// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package botanist_test

import (
	"context"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakeclientmap "github.com/gardener/gardener/pkg/client/kubernetes/clientmap/fake"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/features"
	gardenletfeatures "github.com/gardener/gardener/pkg/gardenlet/features"
	"github.com/gardener/gardener/pkg/operation"
	. "github.com/gardener/gardener/pkg/operation/botanist"
	"github.com/gardener/gardener/pkg/operation/botanist/component/kubeapiserver"
	mockkubeapiserver "github.com/gardener/gardener/pkg/operation/botanist/component/kubeapiserver/mock"
	gardenpkg "github.com/gardener/gardener/pkg/operation/garden"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"
	"github.com/gardener/gardener/pkg/utils/test"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/component-base/featuregate"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("KubeAPIServer", func() {
	var (
		ctrl *gomock.Controller

		gc              client.Client
		k8sGardenClient kubernetes.Interface
		c               client.Client
		k8sSeedClient   kubernetes.Interface
		botanist        *Botanist
		kubeAPIServer   *mockkubeapiserver.MockInterface

		ctx              = context.TODO()
		projectNamespace = "garden-my-project"
		shootNamespace   = "shoot--foo--bar"
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		gc = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()
		k8sGardenClient = fake.NewClientSetBuilder().WithClient(gc).Build()

		c = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		k8sSeedClient = fake.NewClientSetBuilder().WithClient(c).Build()

		kubeAPIServer = mockkubeapiserver.NewMockInterface(ctrl)
		botanist = &Botanist{
			Operation: &operation.Operation{
				K8sGardenClient: k8sGardenClient,
				K8sSeedClient:   k8sSeedClient,
				Garden:          &gardenpkg.Garden{},
				Shoot: &shootpkg.Shoot{
					SeedNamespace: shootNamespace,
					Components: &shootpkg.Components{
						ControlPlane: &shootpkg.ControlPlane{
							KubeAPIServer: kubeAPIServer,
						},
					},
				},
			},
		}
		botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{})
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#DefaultKubeAPIServer", func() {
		Describe("AutoscalingConfig", func() {
			DescribeTable("should have the expected autoscaling config",
				func(prepTest func(), featureGate *featuregate.Feature, value *bool, expectedConfig kubeapiserver.AutoscalingConfig) {
					if prepTest != nil {
						prepTest()
					}

					if featureGate != nil && value != nil {
						defer test.WithFeatureGate(gardenletfeatures.FeatureGate, *featureGate, *value)()
					}

					kubeAPIServer, err := botanist.DefaultKubeAPIServer(ctx)
					Expect(err).NotTo(HaveOccurred())
					Expect(kubeAPIServer.GetValues().Autoscaling).To(Equal(expectedConfig))
				},

				Entry("default behaviour, HVPA is disabled",
					nil,
					featureGatePtr(features.HVPA), pointer.Bool(false),
					kubeapiserver.AutoscalingConfig{
						HVPAEnabled:               false,
						MinReplicas:               1,
						MaxReplicas:               4,
						UseMemoryMetricForHvpaHPA: false,
						ScaleDownDisabledForHvpa:  false,
					},
				),
				Entry("default behaviour, HVPA is enabled",
					nil,
					featureGatePtr(features.HVPA), pointer.Bool(true),
					kubeapiserver.AutoscalingConfig{
						HVPAEnabled:               true,
						MinReplicas:               1,
						MaxReplicas:               4,
						UseMemoryMetricForHvpaHPA: false,
						ScaleDownDisabledForHvpa:  false,
					},
				),
				Entry("shoot purpose production",
					func() {
						botanist.Shoot.Purpose = gardencorev1beta1.ShootPurposeProduction
					},
					nil, nil,
					kubeapiserver.AutoscalingConfig{
						HVPAEnabled:               false,
						MinReplicas:               2,
						MaxReplicas:               4,
						UseMemoryMetricForHvpaHPA: false,
						ScaleDownDisabledForHvpa:  false,
					},
				),
				Entry("shoot disables scale down",
					func() {
						botanist.Shoot.GetInfo().Annotations = map[string]string{"alpha.control-plane.scaling.shoot.gardener.cloud/scale-down-disabled": "true"}
					},
					nil, nil,
					kubeapiserver.AutoscalingConfig{
						HVPAEnabled:               false,
						MinReplicas:               4,
						MaxReplicas:               4,
						UseMemoryMetricForHvpaHPA: false,
						ScaleDownDisabledForHvpa:  true,
					},
				),
				Entry("shoot is a managed seed and HVPAForShootedSeed is disabled",
					func() {
						botanist.ManagedSeed = &seedmanagementv1alpha1.ManagedSeed{}
					},
					featureGatePtr(features.HVPAForShootedSeed), pointer.Bool(false),
					kubeapiserver.AutoscalingConfig{
						HVPAEnabled:               false,
						MinReplicas:               1,
						MaxReplicas:               4,
						UseMemoryMetricForHvpaHPA: true,
						ScaleDownDisabledForHvpa:  false,
					},
				),
				Entry("shoot is a managed seed and HVPAForShootedSeed is enabled",
					func() {
						botanist.ManagedSeed = &seedmanagementv1alpha1.ManagedSeed{}
					},
					featureGatePtr(features.HVPAForShootedSeed), pointer.Bool(true),
					kubeapiserver.AutoscalingConfig{
						HVPAEnabled:               true,
						MinReplicas:               1,
						MaxReplicas:               4,
						UseMemoryMetricForHvpaHPA: true,
						ScaleDownDisabledForHvpa:  false,
					},
				),
				Entry("shoot is a managed seed w/ APIServer settings and HVPAForShootedSeed is enabled",
					func() {
						botanist.ManagedSeed = &seedmanagementv1alpha1.ManagedSeed{}
						botanist.ManagedSeedAPIServer = &helper.ShootedSeedAPIServer{
							Autoscaler: &helper.ShootedSeedAPIServerAutoscaler{
								MinReplicas: pointer.Int32(16),
								MaxReplicas: 32,
							},
							Replicas: pointer.Int32(24),
						}
					},
					featureGatePtr(features.HVPAForShootedSeed), pointer.Bool(true),
					kubeapiserver.AutoscalingConfig{
						HVPAEnabled:               true,
						MinReplicas:               16,
						MaxReplicas:               32,
						UseMemoryMetricForHvpaHPA: true,
						ScaleDownDisabledForHvpa:  false,
					},
				),
				Entry("shoot is a managed seed w/ APIServer settings and HVPAForShootedSeed is disabled",
					func() {
						botanist.ManagedSeed = &seedmanagementv1alpha1.ManagedSeed{}
						botanist.ManagedSeedAPIServer = &helper.ShootedSeedAPIServer{
							Autoscaler: &helper.ShootedSeedAPIServerAutoscaler{
								MinReplicas: pointer.Int32(16),
								MaxReplicas: 32,
							},
							Replicas: pointer.Int32(24),
						}
					},
					featureGatePtr(features.HVPAForShootedSeed), pointer.Bool(false),
					kubeapiserver.AutoscalingConfig{
						HVPAEnabled:               false,
						MinReplicas:               16,
						MaxReplicas:               32,
						Replicas:                  pointer.Int32(24),
						UseMemoryMetricForHvpaHPA: true,
						ScaleDownDisabledForHvpa:  false,
					},
				),
			)
		})

		Describe("OIDCConfig", func() {
			DescribeTable("should have the expected OIDC config",
				func(prepTest func(), expectedConfig *gardencorev1beta1.OIDCConfig) {
					if prepTest != nil {
						prepTest()
					}

					kubeAPIServer, err := botanist.DefaultKubeAPIServer(ctx)
					Expect(err).NotTo(HaveOccurred())
					Expect(kubeAPIServer.GetValues().OIDC).To(Equal(expectedConfig))
				},

				Entry("KubeAPIServerConfig is nil",
					nil,
					nil,
				),
				Entry("OIDCConfig is nil",
					func() {
						botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
							Spec: gardencorev1beta1.ShootSpec{
								Kubernetes: gardencorev1beta1.Kubernetes{
									KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{},
								},
							},
						})
					},
					nil,
				),
				Entry("OIDCConfig is not nil",
					func() {
						botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
							Spec: gardencorev1beta1.ShootSpec{
								Kubernetes: gardencorev1beta1.Kubernetes{
									KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{
										OIDCConfig: &gardencorev1beta1.OIDCConfig{},
									},
								},
							},
						})
					},
					&gardencorev1beta1.OIDCConfig{},
				),
			)
		})

		Describe("ServiceAccountConfig", func() {
			var (
				signingKey       = []byte("some-key")
				signingKeySecret *corev1.Secret
			)

			BeforeEach(func() {
				signingKeySecret = &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-secret",
						Namespace: projectNamespace,
					},
					Data: map[string][]byte{"signing-key": signingKey},
				}
			})

			DescribeTable("should have the expected ServiceAccountConfig config",
				func(prepTest func(), expectedConfig *kubeapiserver.ServiceAccountConfig, errMatcher gomegatypes.GomegaMatcher) {
					if prepTest != nil {
						prepTest()
					}

					kubeAPIServer, err := botanist.DefaultKubeAPIServer(ctx)
					Expect(err).To(errMatcher)
					if kubeAPIServer != nil {
						Expect(kubeAPIServer.GetValues().ServiceAccountConfig).To(Equal(expectedConfig))
					}
				},

				Entry("KubeAPIServerConfig is nil",
					nil,
					nil,
					Not(HaveOccurred()),
				),
				Entry("ServiceAccountConfig is nil",
					func() {
						botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
							Spec: gardencorev1beta1.ShootSpec{
								Kubernetes: gardencorev1beta1.Kubernetes{
									KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{},
								},
							},
						})
					},
					nil,
					Not(HaveOccurred()),
				),
				Entry("SigningKeySecret is nil",
					func() {
						botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
							Spec: gardencorev1beta1.ShootSpec{
								Kubernetes: gardencorev1beta1.Kubernetes{
									KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{
										ServiceAccountConfig: &gardencorev1beta1.ServiceAccountConfig{},
									},
								},
							},
						})
					},
					&kubeapiserver.ServiceAccountConfig{},
					Not(HaveOccurred()),
				),
				Entry("SigningKeySecret is provided but secret is missing",
					func() {
						botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
							ObjectMeta: metav1.ObjectMeta{
								Namespace: projectNamespace,
							},
							Spec: gardencorev1beta1.ShootSpec{
								Kubernetes: gardencorev1beta1.Kubernetes{
									KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{
										ServiceAccountConfig: &gardencorev1beta1.ServiceAccountConfig{
											SigningKeySecret: &corev1.LocalObjectReference{
												Name: signingKeySecret.Name,
											},
										},
									},
								},
							},
						})
					},
					nil,
					MatchError(ContainSubstring("not found")),
				),
				Entry("SigningKeySecret is provided but secret does not have correct data field",
					func() {
						signingKeySecret.Data = nil
						Expect(gc.Create(ctx, signingKeySecret)).To(Succeed())

						botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
							ObjectMeta: metav1.ObjectMeta{
								Namespace: projectNamespace,
							},
							Spec: gardencorev1beta1.ShootSpec{
								Kubernetes: gardencorev1beta1.Kubernetes{
									KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{
										ServiceAccountConfig: &gardencorev1beta1.ServiceAccountConfig{
											SigningKeySecret: &corev1.LocalObjectReference{
												Name: signingKeySecret.Name,
											},
										},
									},
								},
							},
						})
					},
					nil,
					MatchError(ContainSubstring("no signing key in secret")),
				),
				Entry("SigningKeySecret is provided and secret is compliant",
					func() {
						Expect(gc.Create(ctx, signingKeySecret)).To(Succeed())

						botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
							ObjectMeta: metav1.ObjectMeta{
								Namespace: projectNamespace,
							},
							Spec: gardencorev1beta1.ShootSpec{
								Kubernetes: gardencorev1beta1.Kubernetes{
									KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{
										ServiceAccountConfig: &gardencorev1beta1.ServiceAccountConfig{
											SigningKeySecret: &corev1.LocalObjectReference{
												Name: signingKeySecret.Name,
											},
										},
									},
								},
							},
						})
					},
					&kubeapiserver.ServiceAccountConfig{
						SigningKey: signingKey,
					},
					Not(HaveOccurred()),
				),
			)
		})

		Describe("SNIConfig", func() {
			DescribeTable("should have the expected SNI config",
				func(prepTest func(), featureGate *featuregate.Feature, value *bool, expectedConfig kubeapiserver.SNIConfig) {
					if prepTest != nil {
						prepTest()
					}

					if featureGate != nil && value != nil {
						defer test.WithFeatureGate(gardenletfeatures.FeatureGate, *featureGate, *value)()
					}

					kubeAPIServer, err := botanist.DefaultKubeAPIServer(ctx)
					Expect(err).NotTo(HaveOccurred())
					Expect(kubeAPIServer.GetValues().SNI).To(Equal(expectedConfig))
				},

				Entry("SNI disabled",
					nil,
					featureGatePtr(features.APIServerSNI), pointer.Bool(false),
					kubeapiserver.SNIConfig{
						PodMutatorEnabled: false,
					},
				),
				Entry("SNI enabled but no need for internal DNS",
					func() {
						botanist.Shoot.DisableDNS = true
					},
					featureGatePtr(features.APIServerSNI), pointer.Bool(true),
					kubeapiserver.SNIConfig{
						PodMutatorEnabled: false,
					},
				),
				Entry("SNI enabled but no need for external DNS",
					func() {
						botanist.Shoot.DisableDNS = false
						botanist.Garden.InternalDomain = &gardenpkg.Domain{}
						botanist.Shoot.GetInfo().Spec.DNS = nil
					},
					featureGatePtr(features.APIServerSNI), pointer.Bool(true),
					kubeapiserver.SNIConfig{
						PodMutatorEnabled: false,
					},
				),
				Entry("SNI and both DNS enabled",
					func() {
						botanist.Shoot.DisableDNS = false
						botanist.Garden.InternalDomain = &gardenpkg.Domain{}
						botanist.Shoot.ExternalDomain = &gardenpkg.Domain{}
						botanist.Shoot.ExternalClusterDomain = pointer.StringPtr("some-domain")
						botanist.Shoot.GetInfo().Spec.DNS = &gardencorev1beta1.DNS{
							Domain:    pointer.StringPtr("some-domain"),
							Providers: []gardencorev1beta1.DNSProvider{{}},
						}
					},
					featureGatePtr(features.APIServerSNI), pointer.Bool(true),
					kubeapiserver.SNIConfig{
						PodMutatorEnabled: true,
					},
				),
				Entry("SNI and both DNS enabled but pod injector disabled via annotation",
					func() {
						botanist.Shoot.DisableDNS = false
						botanist.Garden.InternalDomain = &gardenpkg.Domain{}
						botanist.Shoot.ExternalDomain = &gardenpkg.Domain{}
						botanist.Shoot.ExternalClusterDomain = pointer.StringPtr("some-domain")
						botanist.Shoot.GetInfo().Spec.DNS = &gardencorev1beta1.DNS{
							Domain:    pointer.StringPtr("some-domain"),
							Providers: []gardencorev1beta1.DNSProvider{{}},
						}
						botanist.Shoot.GetInfo().Annotations = map[string]string{"alpha.featuregates.shoot.gardener.cloud/apiserver-sni-pod-injector": "disable"}
					},
					featureGatePtr(features.APIServerSNI), pointer.Bool(true),
					kubeapiserver.SNIConfig{
						PodMutatorEnabled: false,
					},
				),
			)
		})
	})

	Describe("#DeployKubeAPIServer", func() {
		DescribeTable("should correctly set the autoscaling replicas",
			func(prepTest func(), autoscalingConfig kubeapiserver.AutoscalingConfig, expectedReplicas int32) {
				if prepTest != nil {
					prepTest()
				}

				oldGetDeployKubeAPIServerFunc := GetLegacyDeployKubeAPIServerFunc
				defer func() { GetLegacyDeployKubeAPIServerFunc = oldGetDeployKubeAPIServerFunc }()
				GetLegacyDeployKubeAPIServerFunc = func(*Botanist) func(context.Context) error {
					return func(context.Context) error { return nil }
				}

				kubeAPIServer.EXPECT().GetValues().DoAndReturn(func() kubeapiserver.Values {
					return kubeapiserver.Values{Autoscaling: autoscalingConfig}
				})
				kubeAPIServer.EXPECT().SetAutoscalingReplicas(&expectedReplicas)
				kubeAPIServer.EXPECT().Deploy(ctx)

				Expect(botanist.DeployKubeAPIServer(ctx)).To(Succeed())
			},

			Entry("no change due to already set",
				nil,
				kubeapiserver.AutoscalingConfig{Replicas: pointer.Int32(1)},
				int32(1),
			),
			Entry("use minReplicas because deployment does not exist",
				nil,
				kubeapiserver.AutoscalingConfig{MinReplicas: 2},
				int32(2),
			),
			Entry("use deployment replicas because they are greater than 0",
				func() {
					Expect(c.Create(ctx, &appsv1.Deployment{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "kube-apiserver",
							Namespace: shootNamespace,
						},
						Spec: appsv1.DeploymentSpec{
							Replicas: pointer.Int32(3),
						},
					})).To(Succeed())
				},
				kubeapiserver.AutoscalingConfig{},
				int32(3),
			),
			Entry("use 0 because shoot is hibernated and deployment is already scaled down",
				func() {
					botanist.Shoot.HibernationEnabled = true
					Expect(c.Create(ctx, &appsv1.Deployment{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "kube-apiserver",
							Namespace: shootNamespace,
						},
						Spec: appsv1.DeploymentSpec{
							Replicas: pointer.Int32(0),
						},
					})).To(Succeed())
				},
				kubeapiserver.AutoscalingConfig{},
				int32(0),
			),
		)
	})

	Describe("#DeleteKubeAPIServer", func() {
		It("should properly invalidate the client and destroy the component", func() {
			clientMap := fakeclientmap.NewClientMap().AddClient(keys.ForShoot(botanist.Shoot.GetInfo()), k8sSeedClient)
			botanist.ClientMap = clientMap

			shootClient, err := botanist.ClientMap.GetClient(ctx, keys.ForShoot(botanist.Shoot.GetInfo()))
			Expect(err).NotTo(HaveOccurred())
			Expect(shootClient).To(Equal(k8sSeedClient))

			k8sShootClient := fake.NewClientSetBuilder().WithClient(c).Build()
			botanist.K8sShootClient = k8sShootClient

			kubeAPIServer.EXPECT().Destroy(ctx)

			Expect(botanist.DeleteKubeAPIServer(ctx)).To(Succeed())

			shootClient, err = clientMap.GetClient(ctx, keys.ForShoot(botanist.Shoot.GetInfo()))
			Expect(err).To(MatchError("clientSet for key \"/\" not found"))
			Expect(shootClient).To(BeNil())

			Expect(botanist.K8sShootClient).To(BeNil())
		})
	})

	Describe("#ScaleKubeAPIServerToOne", func() {
		deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "kube-apiserver", Namespace: shootNamespace}}

		It("should scale the KAPI deployment", func() {
			Expect(c.Create(ctx, deployment)).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(Succeed())

			Expect(botanist.ScaleKubeAPIServerToOne(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(Succeed())
			Expect(deployment.Spec.Replicas).To(Equal(pointer.Int32(1)))
		})
	})
})

func featureGatePtr(f featuregate.Feature) *featuregate.Feature {
	return &f
}

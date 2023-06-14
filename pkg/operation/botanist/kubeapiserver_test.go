// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package botanist

import (
	"context"
	"net"

	"github.com/Masterminds/semver"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/component-base/featuregate"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakeclientmap "github.com/gardener/gardener/pkg/client/kubernetes/clientmap/fake"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/component/kubeapiserver"
	mockkubeapiserver "github.com/gardener/gardener/pkg/component/kubeapiserver/mock"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/operation"
	"github.com/gardener/gardener/pkg/operation/garden"
	seedpkg "github.com/gardener/gardener/pkg/operation/seed"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("KubeAPIServer", func() {
	var (
		ctrl *gomock.Controller

		gardenClient  client.Client
		seedClient    client.Client
		seedClientSet kubernetes.Interface
		sm            secretsmanager.Interface
		botanist      *Botanist
		kubeAPIServer *mockkubeapiserver.MockInterface

		ctx                   = context.TODO()
		projectNamespace      = "garden-foo"
		seedNamespace         = "shoot--foo--bar"
		shootName             = "bar"
		internalClusterDomain = "internal.foo.bar.com"
		externalClusterDomain = "external.foo.bar.com"
		podNetwork            *net.IPNet
		serviceNetwork        *net.IPNet
		seedVersion           = "1.22.0"
		apiServerNetwork      = net.ParseIP("10.0.4.1")
		podNetworkCIDR        = "10.0.1.0/24"
		serviceNetworkCIDR    = "10.0.2.0/24"
		nodeNetworkCIDR       = "10.0.3.0/24"
		apiServerClusterIP    = "1.2.3.4"
		apiServerAddress      = "5.6.7.8"
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		gardenClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()
		seedClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		seedClientSet = fake.NewClientSetBuilder().WithClient(seedClient).WithVersion(seedVersion).Build()

		var err error
		_, podNetwork, err = net.ParseCIDR(podNetworkCIDR)
		Expect(err).NotTo(HaveOccurred())
		_, serviceNetwork, err = net.ParseCIDR(serviceNetworkCIDR)
		Expect(err).NotTo(HaveOccurred())

		sm = fakesecretsmanager.New(seedClient, seedNamespace)

		By("Create secrets managed outside of this function for whose secretsmanager.Get() will be called")
		Expect(seedClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ca", Namespace: seedNamespace}})).To(Succeed())
		Expect(seedClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "user-kubeconfig", Namespace: seedNamespace}})).To(Succeed())

		kubeAPIServer = mockkubeapiserver.NewMockInterface(ctrl)
		botanist = &Botanist{
			Operation: &operation.Operation{
				GardenClient:   gardenClient,
				SeedClientSet:  seedClientSet,
				SecretsManager: sm,
				Garden:         &garden.Garden{},
				Seed: &seedpkg.Seed{
					KubernetesVersion: semver.MustParse(seedVersion),
				},
				Shoot: &shootpkg.Shoot{
					SeedNamespace: seedNamespace,
					Components: &shootpkg.Components{
						ControlPlane: &shootpkg.ControlPlane{
							KubeAPIServer: kubeAPIServer,
						},
					},
					InternalClusterDomain: internalClusterDomain,
					ExternalClusterDomain: &externalClusterDomain,
					Networks: &shootpkg.Networks{
						APIServer: apiServerNetwork,
						Pods:      podNetwork,
						Services:  serviceNetwork,
					},
					PSPDisabled:       false,
					KubernetesVersion: semver.MustParse("1.22.1"),
				},
				ImageVector: imagevector.ImageVector{
					{Name: "kube-apiserver"},
					{Name: "vpn-shoot-client"},
					{Name: "alpine"},
				},
				APIServerAddress:   apiServerAddress,
				APIServerClusterIP: apiServerClusterIP,
			},
		}

		botanist.Shoot.SetInfo(&gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      shootName,
				Namespace: projectNamespace,
			},
			Spec: gardencorev1beta1.ShootSpec{
				DNS: &gardencorev1beta1.DNS{
					Domain: &externalClusterDomain,
				},
				Networking: &gardencorev1beta1.Networking{
					Nodes: &nodeNetworkCIDR,
				},
				Kubernetes: gardencorev1beta1.Kubernetes{
					Version: "1.22.0",
				},
				Provider: gardencorev1beta1.Provider{
					Workers: []gardencorev1beta1.Worker{
						{Minimum: 2, Maximum: 20},
						{Minimum: 2, Maximum: 20},
					},
				},
			},
			Status: gardencorev1beta1.ShootStatus{
				TechnicalID: seedNamespace,
			},
		})
		botanist.SetShootState(&gardencorev1beta1.ShootState{})

		botanist.Seed.SetInfo(&gardencorev1beta1.Seed{
			Spec: gardencorev1beta1.SeedSpec{
				Ingress: &gardencorev1beta1.Ingress{
					Domain: "foo.bar.local",
				},
			},
		})
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#DefaultKubeAPIServer", func() {
		Describe("AutoscalingConfig", func() {
			DescribeTable("should have the expected autoscaling config",
				func(prepTest func(), featureGates map[featuregate.Feature]bool, expectedConfig kubeapiserver.AutoscalingConfig) {
					if prepTest != nil {
						prepTest()
					}

					for featureGate, value := range featureGates {
						defer test.WithFeatureGate(features.DefaultFeatureGate, featureGate, value)()
					}

					kubeAPIServer, err := botanist.DefaultKubeAPIServer(ctx)
					Expect(err).NotTo(HaveOccurred())
					Expect(kubeAPIServer.GetValues().Autoscaling).To(Equal(expectedConfig))
				},

				Entry("default behaviour, HVPA is disabled",
					nil,
					map[featuregate.Feature]bool{features.HVPA: false},
					kubeapiserver.AutoscalingConfig{
						APIServerResources:        resourcesRequirementsForKubeAPIServer(4, ""),
						HVPAEnabled:               false,
						MinReplicas:               1,
						MaxReplicas:               4,
						UseMemoryMetricForHvpaHPA: false,
						ScaleDownDisabledForHvpa:  false,
					},
				),
				Entry("default behaviour, HVPA is enabled",
					nil,
					map[featuregate.Feature]bool{features.HVPA: true},
					kubeapiserver.AutoscalingConfig{
						APIServerResources:        resourcesRequirementsForKubeAPIServer(40, ""),
						HVPAEnabled:               true,
						MinReplicas:               1,
						MaxReplicas:               4,
						UseMemoryMetricForHvpaHPA: false,
						ScaleDownDisabledForHvpa:  false,
					},
				),
				Entry("default behaviour, HVPA is enabled and DisableScalingClassesForShoots is enabled",
					nil,
					map[featuregate.Feature]bool{
						features.HVPA:                           true,
						features.DisableScalingClassesForShoots: true,
					},
					kubeapiserver.AutoscalingConfig{
						APIServerResources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("500m"),
								corev1.ResourceMemory: resource.MustParse("1Gi"),
							},
						},
						HVPAEnabled:               true,
						MinReplicas:               1,
						MaxReplicas:               4,
						UseMemoryMetricForHvpaHPA: false,
						ScaleDownDisabledForHvpa:  false,
					},
				),
				Entry("default behaviour, HVPA is disabled and DisableScalingClassesForShoots is enabled",
					nil,
					map[featuregate.Feature]bool{
						features.HVPA:                           false,
						features.DisableScalingClassesForShoots: true,
					},
					kubeapiserver.AutoscalingConfig{
						APIServerResources:        resourcesRequirementsForKubeAPIServer(4, ""),
						HVPAEnabled:               false,
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
					nil,
					kubeapiserver.AutoscalingConfig{
						APIServerResources:        resourcesRequirementsForKubeAPIServer(4, ""),
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
					nil,
					kubeapiserver.AutoscalingConfig{
						APIServerResources:        resourcesRequirementsForKubeAPIServer(4, ""),
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
					map[featuregate.Feature]bool{features.HVPAForShootedSeed: false},
					kubeapiserver.AutoscalingConfig{
						APIServerResources:        resourcesRequirementsForKubeAPIServer(4, ""),
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
					map[featuregate.Feature]bool{features.HVPAForShootedSeed: true},
					kubeapiserver.AutoscalingConfig{
						APIServerResources:        resourcesRequirementsForKubeAPIServer(40, ""),
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
						botanist.ManagedSeedAPIServer = &helper.ManagedSeedAPIServer{
							Autoscaler: &helper.ManagedSeedAPIServerAutoscaler{
								MinReplicas: pointer.Int32(16),
								MaxReplicas: 32,
							},
							Replicas: pointer.Int32(24),
						}
					},
					map[featuregate.Feature]bool{features.HVPAForShootedSeed: true},
					kubeapiserver.AutoscalingConfig{
						APIServerResources:        resourcesRequirementsForKubeAPIServer(40, ""),
						HVPAEnabled:               true,
						MinReplicas:               16,
						MaxReplicas:               32,
						UseMemoryMetricForHvpaHPA: true,
						ScaleDownDisabledForHvpa:  false,
					},
				),
				Entry("shoot is a managed seed w/ APIServer settings and HVPAForShootedSeed is enabled and DisableScalingClassesForShoots is enabled",
					func() {
						botanist.ManagedSeed = &seedmanagementv1alpha1.ManagedSeed{}
						botanist.ManagedSeedAPIServer = &helper.ManagedSeedAPIServer{
							Autoscaler: &helper.ManagedSeedAPIServerAutoscaler{
								MinReplicas: pointer.Int32(16),
								MaxReplicas: 32,
							},
							Replicas: pointer.Int32(24),
						}
					},
					map[featuregate.Feature]bool{
						features.HVPAForShootedSeed:             true,
						features.DisableScalingClassesForShoots: true,
					},
					kubeapiserver.AutoscalingConfig{
						APIServerResources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("500m"),
								corev1.ResourceMemory: resource.MustParse("1Gi"),
							},
						},
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
						botanist.ManagedSeedAPIServer = &helper.ManagedSeedAPIServer{
							Autoscaler: &helper.ManagedSeedAPIServerAutoscaler{
								MinReplicas: pointer.Int32(16),
								MaxReplicas: 32,
							},
							Replicas: pointer.Int32(24),
						}
					},
					map[featuregate.Feature]bool{features.HVPAForShootedSeed: false},
					kubeapiserver.AutoscalingConfig{
						APIServerResources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("1750m"),
								corev1.ResourceMemory: resource.MustParse("2Gi"),
							},
						},
						HVPAEnabled:               false,
						MinReplicas:               16,
						MaxReplicas:               32,
						Replicas:                  pointer.Int32(24),
						UseMemoryMetricForHvpaHPA: true,
						ScaleDownDisabledForHvpa:  false,
					},
				),
				Entry("shoot is a managed seed w/ APIServer settings and HVPAForShootedSeed is disabled and DisableScalingClassesForShoots is enabled",
					func() {
						botanist.ManagedSeed = &seedmanagementv1alpha1.ManagedSeed{}
						botanist.ManagedSeedAPIServer = &helper.ManagedSeedAPIServer{
							Autoscaler: &helper.ManagedSeedAPIServerAutoscaler{
								MinReplicas: pointer.Int32(16),
								MaxReplicas: 32,
							},
							Replicas: pointer.Int32(24),
						}
					},
					map[featuregate.Feature]bool{
						features.HVPAForShootedSeed:             false,
						features.DisableScalingClassesForShoots: true,
					},
					kubeapiserver.AutoscalingConfig{
						APIServerResources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("1750m"),
								corev1.ResourceMemory: resource.MustParse("2Gi"),
							},
						},
						HVPAEnabled:               false,
						MinReplicas:               16,
						MaxReplicas:               32,
						Replicas:                  pointer.Int32(24),
						UseMemoryMetricForHvpaHPA: true,
						ScaleDownDisabledForHvpa:  false,
					},
				),
				Entry("shoot enables HA control planes",
					func() {
						botanist.Shoot.GetInfo().Spec.ControlPlane = &gardencorev1beta1.ControlPlane{
							HighAvailability: &gardencorev1beta1.HighAvailability{
								FailureTolerance: gardencorev1beta1.FailureTolerance{},
							},
						}
					},
					nil,
					kubeapiserver.AutoscalingConfig{
						APIServerResources:        resourcesRequirementsForKubeAPIServer(4, ""),
						HVPAEnabled:               false,
						MinReplicas:               3,
						MaxReplicas:               4,
						UseMemoryMetricForHvpaHPA: false,
						ScaleDownDisabledForHvpa:  false,
					},
				),
			)
		})
	})

	DescribeTable("#resourcesRequirementsForKubeAPIServer",
		func(nodes int, storageClass, expectedCPURequest, expectedMemoryRequest string) {
			Expect(resourcesRequirementsForKubeAPIServer(int32(nodes), storageClass)).To(Equal(
				corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse(expectedCPURequest),
						corev1.ResourceMemory: resource.MustParse(expectedMemoryRequest),
					},
				}))
		},

		// nodes tests
		Entry("nodes <= 2", 2, "", "800m", "800Mi"),
		Entry("nodes <= 10", 10, "", "1000m", "1100Mi"),
		Entry("nodes <= 50", 50, "", "1200m", "1600Mi"),
		Entry("nodes <= 100", 100, "", "2500m", "5200Mi"),
		Entry("nodes > 100", 1000, "", "3000m", "5200Mi"),

		// scaling class tests
		Entry("scaling class small", -1, "small", "800m", "800Mi"),
		Entry("scaling class medium", -1, "medium", "1000m", "1100Mi"),
		Entry("scaling class large", -1, "large", "1200m", "1600Mi"),
		Entry("scaling class xlarge", -1, "xlarge", "2500m", "5200Mi"),
		Entry("scaling class 2xlarge", -1, "2xlarge", "3000m", "5200Mi"),

		// scaling class always decides if provided
		Entry("nodes > 100, scaling class small", 100, "small", "800m", "800Mi"),
		Entry("nodes <= 100, scaling class medium", 100, "medium", "1000m", "1100Mi"),
		Entry("nodes <= 50, scaling class large", 50, "large", "1200m", "1600Mi"),
		Entry("nodes <= 10, scaling class xlarge", 10, "xlarge", "2500m", "5200Mi"),
		Entry("nodes <= 2, scaling class 2xlarge", 2, "2xlarge", "3000m", "5200Mi"),
	)

	Describe("#DeployKubeAPIServer", func() {
		Describe("SNIConfig", func() {

			var secret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "wildcard-secret",
					Namespace: seedNamespace,
					Labels: map[string]string{
						"gardener.cloud/role": "controlplane-cert",
					},
				},
			}

			DescribeTable("should have the expected SNI config",
				func(prepTest func(), expectedConfig kubeapiserver.SNIConfig) {
					if prepTest != nil {
						prepTest()
					}

					kubeAPIServer.EXPECT().GetValues()
					kubeAPIServer.EXPECT().SetAutoscalingReplicas(gomock.Any())
					kubeAPIServer.EXPECT().SetSNIConfig(expectedConfig)
					kubeAPIServer.EXPECT().SetETCDEncryptionConfig(gomock.Any())
					kubeAPIServer.EXPECT().SetExternalHostname(gomock.Any())
					kubeAPIServer.EXPECT().SetExternalServer(gomock.Any())
					kubeAPIServer.EXPECT().SetServerCertificateConfig(gomock.Any())
					kubeAPIServer.EXPECT().SetServiceAccountConfig(gomock.Any())
					kubeAPIServer.EXPECT().Deploy(ctx)

					Expect(botanist.DeployKubeAPIServer(ctx)).To(Succeed())
				},

				Entry("no need for internal DNS",
					func() {},
					kubeapiserver.SNIConfig{
						Enabled: false,
					},
				),
				Entry("no need for external DNS",
					func() {
						botanist.Shoot.GetInfo().Spec.DNS.Providers = []gardencorev1beta1.DNSProvider{{Type: pointer.String("unmanaged")}}
						botanist.Shoot.ExternalClusterDomain = nil
						botanist.Garden.InternalDomain = &gardenerutils.Domain{}
					},
					kubeapiserver.SNIConfig{
						Enabled: false,
					},
				),
				Entry("both DNS needed",
					func() {
						botanist.Garden.InternalDomain = &gardenerutils.Domain{}
						botanist.Shoot.ExternalDomain = &gardenerutils.Domain{}
						botanist.Shoot.ExternalClusterDomain = pointer.String("some-domain")
						botanist.Shoot.GetInfo().Spec.DNS = &gardencorev1beta1.DNS{
							Domain:    pointer.String("some-domain"),
							Providers: []gardencorev1beta1.DNSProvider{{}},
						}
					},
					kubeapiserver.SNIConfig{
						Enabled:          true,
						AdvertiseAddress: apiServerClusterIP,
					},
				),
				Entry("Control plane wildcard certificate available",
					func() {
						botanist.ControlPlaneWildcardCert = secret
					},
					kubeapiserver.SNIConfig{
						Enabled: false,
						TLS: []kubeapiserver.TLSSNIConfig{
							{
								SecretName:     &secret.Name,
								DomainPatterns: []string{"api-foo--bar.foo.bar.local"},
							},
						},
					},
				),
			)
		})

		It("should sync the kubeconfig to the garden project namespace when enableStaticTokenKubeconfig is set to true", func() {
			kubeAPIServer.EXPECT().GetValues()
			kubeAPIServer.EXPECT().SetAutoscalingReplicas(gomock.Any())
			kubeAPIServer.EXPECT().SetSNIConfig(gomock.Any())
			kubeAPIServer.EXPECT().SetETCDEncryptionConfig(gomock.Any())
			kubeAPIServer.EXPECT().SetExternalHostname(gomock.Any())
			kubeAPIServer.EXPECT().SetExternalServer(gomock.Any())
			kubeAPIServer.EXPECT().SetServerCertificateConfig(gomock.Any())
			kubeAPIServer.EXPECT().SetServiceAccountConfig(gomock.Any())
			kubeAPIServer.EXPECT().Deploy(ctx)

			Expect(gardenClient.Get(ctx, kubernetesutils.Key(projectNamespace, shootName+".kubeconfig"), &corev1.Secret{})).To(BeNotFoundError())

			Expect(botanist.DeployKubeAPIServer(ctx)).To(Succeed())

			kubeconfigSecret := &corev1.Secret{}
			Expect(gardenClient.Get(ctx, kubernetesutils.Key(projectNamespace, shootName+".kubeconfig"), kubeconfigSecret)).To(Succeed())
			Expect(kubeconfigSecret.Annotations).To(HaveKeyWithValue("url", "https://api."+externalClusterDomain))
			Expect(kubeconfigSecret.Labels).To(HaveKeyWithValue("gardener.cloud/role", "kubeconfig"))
			Expect(kubeconfigSecret.Data).To(And(
				HaveKey("ca.crt"),
				HaveKeyWithValue("data-for", []byte("user-kubeconfig")),
			))
		})

		It("should not sync the kubeconfig to garden project namespace when enableStaticTokenKubeconfig is set to false", func() {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      shootName + ".kubeconfig",
					Namespace: projectNamespace,
				},
			}
			Expect(gardenClient.Create(ctx, secret)).To(Succeed())

			Expect(gardenClient.Get(ctx, kubernetesutils.Key(projectNamespace, shootName+".kubeconfig"), &corev1.Secret{})).To(Succeed())

			kubeAPIServer.EXPECT().GetValues()
			kubeAPIServer.EXPECT().SetAutoscalingReplicas(gomock.Any())
			kubeAPIServer.EXPECT().SetSNIConfig(gomock.Any())
			kubeAPIServer.EXPECT().SetETCDEncryptionConfig(gomock.Any())
			kubeAPIServer.EXPECT().SetExternalHostname(gomock.Any())
			kubeAPIServer.EXPECT().SetExternalServer(gomock.Any())
			kubeAPIServer.EXPECT().SetServerCertificateConfig(gomock.Any())
			kubeAPIServer.EXPECT().SetServiceAccountConfig(gomock.Any())
			kubeAPIServer.EXPECT().Deploy(ctx)

			shootCopy := botanist.Shoot.GetInfo().DeepCopy()
			shootCopy.Spec.Kubernetes = gardencorev1beta1.Kubernetes{
				KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{
					ServiceAccountConfig: &gardencorev1beta1.ServiceAccountConfig{
						Issuer:          pointer.String("issuer"),
						AcceptedIssuers: []string{"issuer1", "issuer2"},
					},
				},
				EnableStaticTokenKubeconfig: pointer.Bool(false),
			}
			botanist.Shoot.SetInfo(shootCopy)

			Expect(botanist.DeployKubeAPIServer(ctx)).To(Succeed())

			Expect(gardenClient.Get(ctx, kubernetesutils.Key(projectNamespace, shootName+".kubeconfig"), &corev1.Secret{})).To(BeNotFoundError())
		})
	})

	Describe("#DeleteKubeAPIServer", func() {
		It("should properly invalidate the client and destroy the component", func() {
			clientMap := fakeclientmap.NewClientMap().AddClient(keys.ForShoot(botanist.Shoot.GetInfo()), seedClientSet)
			botanist.ShootClientMap = clientMap

			shootClient, err := botanist.ShootClientMap.GetClient(ctx, keys.ForShoot(botanist.Shoot.GetInfo()))
			Expect(err).NotTo(HaveOccurred())
			Expect(shootClient).To(Equal(seedClientSet))

			botanist.ShootClientSet = fake.NewClientSetBuilder().WithClient(seedClient).Build()

			kubeAPIServer.EXPECT().Destroy(ctx)

			Expect(botanist.DeleteKubeAPIServer(ctx)).To(Succeed())

			shootClient, err = clientMap.GetClient(ctx, keys.ForShoot(botanist.Shoot.GetInfo()))
			Expect(err).To(MatchError(`clientSet for key "` + botanist.Shoot.GetInfo().Namespace + `/` + botanist.Shoot.GetInfo().Name + `" not found`))
			Expect(shootClient).To(BeNil())

			Expect(botanist.ShootClientSet).To(BeNil())
		})
	})

	Describe("#ScaleKubeAPIServerToOne", func() {
		deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "kube-apiserver", Namespace: seedNamespace}}

		It("should scale the KAPI deployment", func() {
			Expect(seedClient.Create(ctx, deployment)).To(Succeed())
			Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(Succeed())

			Expect(botanist.ScaleKubeAPIServerToOne(ctx)).To(Succeed())

			Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(Succeed())
			Expect(deployment.Spec.Replicas).To(Equal(pointer.Int32(1)))
		})
	})
})

func featureGatePtr(f featuregate.Feature) *featuregate.Feature {
	return &f
}

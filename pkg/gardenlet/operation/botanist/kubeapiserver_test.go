// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"net"
	"time"

	"github.com/Masterminds/semver/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"go.uber.org/mock/gomock"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/component-base/featuregate"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakeclientmap "github.com/gardener/gardener/pkg/client/kubernetes/clientmap/fake"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/component/apiserver"
	kubeapiserver "github.com/gardener/gardener/pkg/component/kubernetes/apiserver"
	mockkubeapiserver "github.com/gardener/gardener/pkg/component/kubernetes/apiserver/mock"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	"github.com/gardener/gardener/pkg/gardenlet/operation/garden"
	seedpkg "github.com/gardener/gardener/pkg/gardenlet/operation/seed"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
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
		podNetworks           []net.IPNet
		serviceNetwork        *net.IPNet
		serviceNetworks       []net.IPNet
		seedVersion           = "1.26.0"
		apiServerNetwork      = []net.IP{net.ParseIP("10.0.4.1")}
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
		podNetworks = []net.IPNet{*podNetwork}
		_, serviceNetwork, err = net.ParseCIDR(serviceNetworkCIDR)
		Expect(err).NotTo(HaveOccurred())
		serviceNetworks = []net.IPNet{*serviceNetwork}

		sm = fakesecretsmanager.New(seedClient, seedNamespace)

		By("Create secrets managed outside of this function for which secretsmanager.Get() will be called")
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
						Pods:      podNetworks,
						Services:  serviceNetworks,
					},
					KubernetesVersion: semver.MustParse("1.26.1"),
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
					Version: "1.26.0",
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
		botanist.Shoot.SetShootState(&gardencorev1beta1.ShootState{})

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
				func(prepTest func(), featureGates map[featuregate.Feature]bool, expectedConfig apiserver.AutoscalingConfig) {
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

				Entry("default behaviour, HVPA is disabled, VPAAndHPAForAPIServer is enabled",
					nil,
					map[featuregate.Feature]bool{
						features.HVPA:                  false,
						features.VPAAndHPAForAPIServer: true,
					},
					apiserver.AutoscalingConfig{
						Mode: apiserver.AutoscalingModeVPAAndHPA,
						APIServerResources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("250m"),
								corev1.ResourceMemory: resource.MustParse("500Mi"),
							},
						},
						MinReplicas:               2,
						MaxReplicas:               6,
						UseMemoryMetricForHvpaHPA: false,
						ScaleDownDisabled:         false,
					},
				),
				Entry("default behaviour, HVPA is enabled, VPAAndHPAForAPIServer is enabled",
					nil,
					map[featuregate.Feature]bool{
						features.HVPA:                  true,
						features.VPAAndHPAForAPIServer: true,
					},
					apiserver.AutoscalingConfig{
						Mode: apiserver.AutoscalingModeVPAAndHPA,
						APIServerResources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("250m"),
								corev1.ResourceMemory: resource.MustParse("500Mi"),
							},
						},
						MinReplicas:               2,
						MaxReplicas:               6,
						UseMemoryMetricForHvpaHPA: false,
						ScaleDownDisabled:         false,
					},
				),
				Entry("shoot disables scale down, HVPA is enabled, VPAAndHPAForAPIServer is enabled",
					func() {
						botanist.Shoot.GetInfo().Annotations = map[string]string{"alpha.control-plane.scaling.shoot.gardener.cloud/scale-down-disabled": "true"}
					},
					map[featuregate.Feature]bool{
						features.HVPA:                  true,
						features.VPAAndHPAForAPIServer: true,
					},
					apiserver.AutoscalingConfig{
						Mode: apiserver.AutoscalingModeVPAAndHPA,
						APIServerResources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("250m"),
								corev1.ResourceMemory: resource.MustParse("500Mi"),
							},
						},
						MinReplicas:               4,
						MaxReplicas:               6,
						UseMemoryMetricForHvpaHPA: false,
						ScaleDownDisabled:         true,
					},
				),
				Entry("shoot is a managed seed and HVPAForShootedSeed is disabled, VPAAndHPAForAPIServer is enabled",
					func() {
						botanist.ManagedSeed = &seedmanagementv1alpha1.ManagedSeed{}
					},
					map[featuregate.Feature]bool{
						features.HVPAForShootedSeed:    false,
						features.VPAAndHPAForAPIServer: true,
					},
					apiserver.AutoscalingConfig{
						Mode: apiserver.AutoscalingModeVPAAndHPA,
						APIServerResources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("250m"),
								corev1.ResourceMemory: resource.MustParse("500Mi"),
							},
						},
						MinReplicas:               2,
						MaxReplicas:               6,
						UseMemoryMetricForHvpaHPA: true,
						ScaleDownDisabled:         false,
					},
				),
				Entry("shoot is a managed seed w/ APIServer settings and HVPAForShootedSeed is disabled, VPAAndHPAForAPIServer is enabled",
					func() {
						botanist.ManagedSeed = &seedmanagementv1alpha1.ManagedSeed{}
						botanist.ManagedSeedAPIServer = &helper.ManagedSeedAPIServer{
							Autoscaler: &helper.ManagedSeedAPIServerAutoscaler{
								MinReplicas: ptr.To[int32](16),
								MaxReplicas: 32,
							},
							Replicas: ptr.To[int32](24),
						}
					},
					map[featuregate.Feature]bool{
						features.HVPAForShootedSeed:    false,
						features.VPAAndHPAForAPIServer: true,
					},
					apiserver.AutoscalingConfig{
						Mode: apiserver.AutoscalingModeVPAAndHPA,
						APIServerResources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("250m"),
								corev1.ResourceMemory: resource.MustParse("500Mi"),
							},
						},
						MinReplicas:               16,
						MaxReplicas:               32,
						UseMemoryMetricForHvpaHPA: true,
						ScaleDownDisabled:         false,
					},
				),
				Entry("shoot enables HA control planes, VPAAndHPAForAPIServer is enabled",
					func() {
						botanist.Shoot.GetInfo().Spec.ControlPlane = &gardencorev1beta1.ControlPlane{
							HighAvailability: &gardencorev1beta1.HighAvailability{
								FailureTolerance: gardencorev1beta1.FailureTolerance{},
							},
						}
					},
					map[featuregate.Feature]bool{
						features.VPAAndHPAForAPIServer: true,
					},
					apiserver.AutoscalingConfig{
						Mode: apiserver.AutoscalingModeVPAAndHPA,
						APIServerResources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("250m"),
								corev1.ResourceMemory: resource.MustParse("500Mi"),
							},
						},
						MinReplicas:               3,
						MaxReplicas:               6,
						UseMemoryMetricForHvpaHPA: false,
						ScaleDownDisabled:         false,
					},
				),
			)
		})
	})

	DescribeTable("#resourcesRequirementsForKubeAPIServerInBaselineMode",
		func(nodes int, expectedCPURequest, expectedMemoryRequest string) {
			Expect(resourcesRequirementsForKubeAPIServerInBaselineMode(int32(nodes))).To(Equal(
				corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse(expectedCPURequest),
						corev1.ResourceMemory: resource.MustParse(expectedMemoryRequest),
					},
				}))
		},

		Entry("nodes <= 2", 2, "800m", "800Mi"),
		Entry("nodes <= 10", 10, "1000m", "1100Mi"),
		Entry("nodes <= 50", 50, "1200m", "1600Mi"),
		Entry("nodes <= 100", 100, "2500m", "5200Mi"),
		Entry("nodes > 100", 1000, "3000m", "5200Mi"),
	)

	Describe("#DeployKubeAPIServer", func() {
		Describe("SNIConfig", func() {
			secret := &corev1.Secret{
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
					kubeAPIServer.EXPECT().SetNodeNetworkCIDRs(gomock.Any())
					kubeAPIServer.EXPECT().SetServiceNetworkCIDRs(gomock.Any())
					kubeAPIServer.EXPECT().SetPodNetworkCIDRs(gomock.Any())
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
						botanist.Shoot.GetInfo().Spec.DNS.Providers = []gardencorev1beta1.DNSProvider{{Type: ptr.To("unmanaged")}}
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
						botanist.Shoot.ExternalClusterDomain = ptr.To("some-domain")
						botanist.Shoot.GetInfo().Spec.DNS = &gardencorev1beta1.DNS{
							Domain:    ptr.To("some-domain"),
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

		Describe("ServiceAccountConfig", func() {
			DescribeTable("should have the expected ServiceAccount config",
				func(prepTest func(), expectedConfig kubeapiserver.ServiceAccountConfig) {
					if prepTest != nil {
						prepTest()
					}

					kubeAPIServer.EXPECT().GetValues()
					kubeAPIServer.EXPECT().SetAutoscalingReplicas(gomock.Any())
					kubeAPIServer.EXPECT().SetSNIConfig(gomock.Any())
					kubeAPIServer.EXPECT().SetETCDEncryptionConfig(gomock.Any())
					kubeAPIServer.EXPECT().SetExternalHostname(gomock.Any())
					kubeAPIServer.EXPECT().SetExternalServer(gomock.Any())
					kubeAPIServer.EXPECT().SetNodeNetworkCIDRs(gomock.Any())
					kubeAPIServer.EXPECT().SetPodNetworkCIDRs(gomock.Any())
					kubeAPIServer.EXPECT().SetServiceNetworkCIDRs(gomock.Any())
					kubeAPIServer.EXPECT().SetServerCertificateConfig(gomock.Any())
					kubeAPIServer.EXPECT().SetServiceAccountConfig(expectedConfig)
					kubeAPIServer.EXPECT().Deploy(ctx)

					Expect(botanist.DeployKubeAPIServer(ctx)).To(Succeed())
				},

				Entry("should default the issuer",
					func() {
						DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.ShootManagedIssuer, true))
					},
					kubeapiserver.ServiceAccountConfig{
						Issuer: "https://api.internal.foo.bar.com",
					},
				),
				Entry("should set configuration correctly",
					func() {
						botanist.Shoot.GetInfo().Spec.Kubernetes.KubeAPIServer = &gardencorev1beta1.KubeAPIServerConfig{
							ServiceAccountConfig: &gardencorev1beta1.ServiceAccountConfig{
								Issuer:                ptr.To("foo"),
								ExtendTokenExpiration: ptr.To(false),
								MaxTokenExpiration:    &metav1.Duration{Duration: time.Second},
								AcceptedIssuers:       []string{"aa", "bb"},
							},
						}
						DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.ShootManagedIssuer, true))
					},
					kubeapiserver.ServiceAccountConfig{
						Issuer:                "foo",
						ExtendTokenExpiration: ptr.To(false),
						MaxTokenExpiration:    &metav1.Duration{Duration: time.Second},
						AcceptedIssuers:       []string{"aa", "bb", "https://api.internal.foo.bar.com"},
					},
				),
				Entry("should set managed issuer configuration",
					func() {
						botanist.Garden = &garden.Garden{
							Project: &gardencorev1beta1.Project{
								ObjectMeta: metav1.ObjectMeta{
									Name: "test",
								},
							},
						}
						botanist.Shoot.ServiceAccountIssuerHostname = ptr.To("foo.bar.example.cloud")
						botanist.Shoot.GetInfo().ObjectMeta.UID = "some-uuid"
						botanist.Shoot.GetInfo().Annotations = map[string]string{
							"authentication.gardener.cloud/issuer": "managed",
						}
						botanist.Shoot.GetInfo().Spec.Kubernetes.KubeAPIServer = &gardencorev1beta1.KubeAPIServerConfig{
							ServiceAccountConfig: &gardencorev1beta1.ServiceAccountConfig{
								ExtendTokenExpiration: ptr.To(false),
								MaxTokenExpiration:    &metav1.Duration{Duration: time.Second},
								AcceptedIssuers:       []string{"aa", "bb"},
							},
						}
						DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.ShootManagedIssuer, true))
					},
					kubeapiserver.ServiceAccountConfig{
						Issuer:                "https://foo.bar.example.cloud/projects/test/shoots/some-uuid/issuer",
						ExtendTokenExpiration: ptr.To(false),
						MaxTokenExpiration:    &metav1.Duration{Duration: time.Second},
						AcceptedIssuers:       []string{"aa", "bb", "https://api.internal.foo.bar.com"},
						JWKSURI:               ptr.To("https://foo.bar.example.cloud/projects/test/shoots/some-uuid/issuer/jwks"),
					},
				),
				Entry("should not set managed issuer configuration because ShootManagedIssuer feature gate is disabled",
					func() {
						botanist.Garden = &garden.Garden{
							Project: &gardencorev1beta1.Project{
								ObjectMeta: metav1.ObjectMeta{
									Name: "test",
								},
							},
						}
						botanist.Shoot.ServiceAccountIssuerHostname = ptr.To("foo.bar.example.cloud")
						botanist.Shoot.GetInfo().ObjectMeta.UID = "some-uuid"
						botanist.Shoot.GetInfo().Annotations = map[string]string{
							"authentication.gardener.cloud/issuer": "managed",
						}
						botanist.Shoot.GetInfo().Spec.Kubernetes.KubeAPIServer = &gardencorev1beta1.KubeAPIServerConfig{
							ServiceAccountConfig: &gardencorev1beta1.ServiceAccountConfig{
								ExtendTokenExpiration: ptr.To(false),
								MaxTokenExpiration:    &metav1.Duration{Duration: time.Second},
								AcceptedIssuers:       []string{"aa", "bb"},
							},
						}
						DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.ShootManagedIssuer, false))
					},
					kubeapiserver.ServiceAccountConfig{
						Issuer:                "https://api.internal.foo.bar.com",
						ExtendTokenExpiration: ptr.To(false),
						MaxTokenExpiration:    &metav1.Duration{Duration: time.Second},
						AcceptedIssuers:       []string{"aa", "bb"},
					},
				),
			)

			It("should return error because shoot wants managed issuer, but issuer hostname is not configured", func() {
				DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.ShootManagedIssuer, true))
				botanist.Garden = &garden.Garden{
					Project: &gardencorev1beta1.Project{
						ObjectMeta: metav1.ObjectMeta{
							Name: "test",
						},
					},
				}
				botanist.Shoot.ServiceAccountIssuerHostname = nil
				botanist.Shoot.GetInfo().ObjectMeta.UID = "some-uuid"
				botanist.Shoot.GetInfo().Annotations = map[string]string{
					"authentication.gardener.cloud/issuer": "managed",
				}
				botanist.Shoot.GetInfo().Spec.Kubernetes.KubeAPIServer = &gardencorev1beta1.KubeAPIServerConfig{
					ServiceAccountConfig: &gardencorev1beta1.ServiceAccountConfig{},
				}

				err := botanist.DeployKubeAPIServer(ctx)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("shoot requires managed issuer, but gardener does not have shoot service account hostname configured"))
			})
		})

		It("should sync the kubeconfig to the garden project namespace when enableStaticTokenKubeconfig is set to true", func() {
			kubeAPIServer.EXPECT().GetValues()
			kubeAPIServer.EXPECT().SetAutoscalingReplicas(gomock.Any())
			kubeAPIServer.EXPECT().SetSNIConfig(gomock.Any())
			kubeAPIServer.EXPECT().SetETCDEncryptionConfig(gomock.Any())
			kubeAPIServer.EXPECT().SetExternalHostname(gomock.Any())
			kubeAPIServer.EXPECT().SetExternalServer(gomock.Any())
			kubeAPIServer.EXPECT().SetNodeNetworkCIDRs(gomock.Any())
			kubeAPIServer.EXPECT().SetPodNetworkCIDRs(gomock.Any())
			kubeAPIServer.EXPECT().SetServiceNetworkCIDRs(gomock.Any())
			kubeAPIServer.EXPECT().SetServerCertificateConfig(gomock.Any())
			kubeAPIServer.EXPECT().SetServiceAccountConfig(gomock.Any())
			kubeAPIServer.EXPECT().Deploy(ctx)

			Expect(gardenClient.Get(ctx, client.ObjectKey{Namespace: projectNamespace, Name: shootName + ".kubeconfig"}, &corev1.Secret{})).To(BeNotFoundError())

			Expect(botanist.DeployKubeAPIServer(ctx)).To(Succeed())

			kubeconfigSecret := &corev1.Secret{}
			Expect(gardenClient.Get(ctx, client.ObjectKey{Namespace: projectNamespace, Name: shootName + ".kubeconfig"}, kubeconfigSecret)).To(Succeed())
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

			Expect(gardenClient.Get(ctx, client.ObjectKey{Namespace: projectNamespace, Name: shootName + ".kubeconfig"}, &corev1.Secret{})).To(Succeed())

			kubeAPIServer.EXPECT().GetValues()
			kubeAPIServer.EXPECT().SetAutoscalingReplicas(gomock.Any())
			kubeAPIServer.EXPECT().SetSNIConfig(gomock.Any())
			kubeAPIServer.EXPECT().SetETCDEncryptionConfig(gomock.Any())
			kubeAPIServer.EXPECT().SetExternalHostname(gomock.Any())
			kubeAPIServer.EXPECT().SetExternalServer(gomock.Any())
			kubeAPIServer.EXPECT().SetNodeNetworkCIDRs(gomock.Any())
			kubeAPIServer.EXPECT().SetPodNetworkCIDRs(gomock.Any())
			kubeAPIServer.EXPECT().SetServiceNetworkCIDRs(gomock.Any())
			kubeAPIServer.EXPECT().SetServerCertificateConfig(gomock.Any())
			kubeAPIServer.EXPECT().SetServiceAccountConfig(gomock.Any())
			kubeAPIServer.EXPECT().Deploy(ctx)

			shootCopy := botanist.Shoot.GetInfo().DeepCopy()
			shootCopy.Spec.Kubernetes = gardencorev1beta1.Kubernetes{
				KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{
					ServiceAccountConfig: &gardencorev1beta1.ServiceAccountConfig{
						Issuer:          ptr.To("issuer"),
						AcceptedIssuers: []string{"issuer1", "issuer2"},
					},
				},
				EnableStaticTokenKubeconfig: ptr.To(false),
			}
			botanist.Shoot.SetInfo(shootCopy)

			Expect(botanist.DeployKubeAPIServer(ctx)).To(Succeed())

			Expect(gardenClient.Get(ctx, client.ObjectKey{Namespace: projectNamespace, Name: shootName + ".kubeconfig"}, &corev1.Secret{})).To(BeNotFoundError())
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

		It("should scale the deployment", func() {
			Expect(seedClient.Create(ctx, deployment)).To(Succeed())
			Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(Succeed())

			Expect(botanist.ScaleKubeAPIServerToOne(ctx)).To(Succeed())

			Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(Succeed())
			Expect(deployment.Spec.Replicas).To(PointTo(Equal(int32(1))))
		})
	})
})

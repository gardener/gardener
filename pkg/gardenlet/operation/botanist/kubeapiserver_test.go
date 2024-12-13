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
	apiserverv1beta1 "k8s.io/apiserver/pkg/apis/apiserver/v1beta1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakeclientmap "github.com/gardener/gardener/pkg/client/kubernetes/clientmap/fake"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/client/kubernetes/fake"
	"github.com/gardener/gardener/pkg/component/apiserver"
	kubeapiserver "github.com/gardener/gardener/pkg/component/kubernetes/apiserver"
	mockkubeapiserver "github.com/gardener/gardener/pkg/component/kubernetes/apiserver/mock"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	"github.com/gardener/gardener/pkg/gardenlet/operation/garden"
	seedpkg "github.com/gardener/gardener/pkg/gardenlet/operation/seed"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
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
		controlPlaneNamespace = "shoot--foo--bar"
		shootName             = "bar"
		internalClusterDomain = "internal.foo.bar.com"
		externalClusterDomain = "external.foo.bar.com"
		podNetwork            *net.IPNet
		podNetworks           []net.IPNet
		serviceNetwork        *net.IPNet
		serviceNetworks       []net.IPNet
		seedVersion           = "1.31.1"
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

		sm = fakesecretsmanager.New(seedClient, controlPlaneNamespace)

		By("Create secrets managed outside of this function for which secretsmanager.Get() will be called")
		Expect(seedClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ca", Namespace: controlPlaneNamespace}})).To(Succeed())
		Expect(seedClient.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "user-kubeconfig", Namespace: controlPlaneNamespace}})).To(Succeed())

		kubeAPIServer = mockkubeapiserver.NewMockInterface(ctrl)
		botanist = &Botanist{
			Operation: &operation.Operation{
				Config: &gardenletconfigv1alpha1.GardenletConfiguration{
					SNI: &gardenletconfigv1alpha1.SNI{
						Ingress: &gardenletconfigv1alpha1.SNIIngress{
							Namespace:   ptr.To(v1beta1constants.DefaultSNIIngressNamespace),
							ServiceName: ptr.To(v1beta1constants.DefaultSNIIngressServiceName),
							Labels: map[string]string{
								v1beta1constants.LabelApp: v1beta1constants.DefaultIngressGatewayAppLabelValue,
								"istio":                   "ingressgateway",
							},
						},
					},
				},
				GardenClient:   gardenClient,
				SeedClientSet:  seedClientSet,
				SecretsManager: sm,
				Garden:         &garden.Garden{},
				Seed: &seedpkg.Seed{
					KubernetesVersion: semver.MustParse(seedVersion),
				},
				Shoot: &shootpkg.Shoot{
					ControlPlaneNamespace: controlPlaneNamespace,
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
					KubernetesVersion: semver.MustParse("1.31.1"),
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
					Version: "1.31.1",
				},
				Provider: gardencorev1beta1.Provider{
					Workers: []gardencorev1beta1.Worker{
						{Minimum: 2, Maximum: 20},
						{Minimum: 2, Maximum: 20},
					},
				},
			},
			Status: gardencorev1beta1.ShootStatus{
				TechnicalID: controlPlaneNamespace,
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
				func(prepTest func(), expectedConfig apiserver.AutoscalingConfig) {
					if prepTest != nil {
						prepTest()
					}

					kubeAPIServer, err := botanist.DefaultKubeAPIServer(ctx)
					Expect(err).NotTo(HaveOccurred())
					Expect(kubeAPIServer.GetValues().Autoscaling).To(Equal(expectedConfig))
				},

				Entry("default behaviour",
					nil,
					apiserver.AutoscalingConfig{
						APIServerResources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("250m"),
								corev1.ResourceMemory: resource.MustParse("500Mi"),
							},
						},
						MinReplicas:       2,
						MaxReplicas:       6,
						ScaleDownDisabled: false,
					},
				),
				Entry("shoot disables scale down",
					func() {
						botanist.Shoot.GetInfo().Annotations = map[string]string{"alpha.control-plane.scaling.shoot.gardener.cloud/scale-down-disabled": "true"}
					},
					apiserver.AutoscalingConfig{
						APIServerResources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("250m"),
								corev1.ResourceMemory: resource.MustParse("500Mi"),
							},
						},
						MinReplicas:       4,
						MaxReplicas:       6,
						ScaleDownDisabled: true,
					},
				),
				Entry("shoot is a managed seed",
					func() {
						botanist.ManagedSeed = &seedmanagementv1alpha1.ManagedSeed{}
					},
					apiserver.AutoscalingConfig{
						APIServerResources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("250m"),
								corev1.ResourceMemory: resource.MustParse("500Mi"),
							},
						},
						MinReplicas:       2,
						MaxReplicas:       6,
						ScaleDownDisabled: false,
					},
				),
				Entry("shoot is a managed seed w/ APIServer settings",
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
					apiserver.AutoscalingConfig{
						APIServerResources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("250m"),
								corev1.ResourceMemory: resource.MustParse("500Mi"),
							},
						},
						MinReplicas:       16,
						MaxReplicas:       32,
						ScaleDownDisabled: false,
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
					apiserver.AutoscalingConfig{
						APIServerResources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("250m"),
								corev1.ResourceMemory: resource.MustParse("500Mi"),
							},
						},
						MinReplicas:       3,
						MaxReplicas:       6,
						ScaleDownDisabled: false,
					},
				),
			)
		})
	})

	Describe("#DeployKubeAPIServer", func() {
		Describe("SNIConfig", func() {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "wildcard-secret",
					Namespace: controlPlaneNamespace,
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

					Expect(botanist.DeployKubeAPIServer(ctx, false)).To(Succeed())
				},

				Entry("no need for internal DNS",
					func() {},
					kubeapiserver.SNIConfig{
						Enabled:                      false,
						IstioIngressGatewayNamespace: "istio-ingress",
					},
				),
				Entry("no need for external DNS",
					func() {
						botanist.Shoot.GetInfo().Spec.DNS.Providers = []gardencorev1beta1.DNSProvider{{Type: ptr.To("unmanaged")}}
						botanist.Shoot.ExternalClusterDomain = nil
						botanist.Garden.InternalDomain = &gardenerutils.Domain{}
					},
					kubeapiserver.SNIConfig{
						Enabled:                      false,
						IstioIngressGatewayNamespace: "istio-ingress",
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
						Enabled:                      true,
						AdvertiseAddress:             apiServerClusterIP,
						IstioIngressGatewayNamespace: "istio-ingress",
					},
				),
				Entry("Control plane wildcard certificate available",
					func() {
						botanist.ControlPlaneWildcardCert = secret
					},
					kubeapiserver.SNIConfig{
						Enabled:                      false,
						IstioIngressGatewayNamespace: "istio-ingress",
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

					Expect(botanist.DeployKubeAPIServer(ctx, false)).To(Succeed())
				},

				Entry("should default the issuer",
					func() {},
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
					},
					kubeapiserver.ServiceAccountConfig{
						Issuer:                "https://foo.bar.example.cloud/projects/test/shoots/some-uuid/issuer",
						ExtendTokenExpiration: ptr.To(false),
						MaxTokenExpiration:    &metav1.Duration{Duration: time.Second},
						AcceptedIssuers:       []string{"aa", "bb", "https://api.internal.foo.bar.com"},
						JWKSURI:               ptr.To("https://foo.bar.example.cloud/projects/test/shoots/some-uuid/issuer/jwks"),
					},
				),
			)

			It("should return error because shoot wants managed issuer, but issuer hostname is not configured", func() {
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

				err := botanist.DeployKubeAPIServer(ctx, false)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("shoot requires managed issuer, but gardener does not have shoot service account hostname configured"))
			})
		})

		It("should append the node-agent-authorizer webhook configuration if it is enabled", func() {
			expectedKubeconfig := []byte(`apiVersion: v1
clusters:
- cluster:
    server: https://gardener-resource-manager/webhooks/auth/nodeagent
  name: authorization-webhook
contexts:
- context:
    cluster: authorization-webhook
    user: authorization-webhook
  name: authorization-webhook
current-context: authorization-webhook
kind: Config
preferences: {}
users:
- name: authorization-webhook
  user: {}
`)

			expectedAuthorizationWebhook := kubeapiserver.AuthorizationWebhook{
				Name:       "node-agent-authorizer",
				Kubeconfig: expectedKubeconfig,
				WebhookConfiguration: apiserverv1beta1.WebhookConfiguration{
					AuthorizedTTL:                            metav1.Duration{Duration: 1 * time.Nanosecond},
					UnauthorizedTTL:                          metav1.Duration{Duration: 1 * time.Nanosecond},
					Timeout:                                  metav1.Duration{Duration: 10 * time.Second},
					FailurePolicy:                            "Deny",
					SubjectAccessReviewVersion:               "v1",
					MatchConditionSubjectAccessReviewVersion: "v1",
					MatchConditions: []apiserverv1beta1.WebhookMatchCondition{{
						Expression: "'gardener.cloud:node-agents' in request.groups",
					}},
				},
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
			kubeAPIServer.EXPECT().SetServiceAccountConfig(gomock.Any())
			kubeAPIServer.EXPECT().AppendAuthorizationWebhook(expectedAuthorizationWebhook)
			kubeAPIServer.EXPECT().Deploy(ctx)

			Expect(botanist.DeployKubeAPIServer(ctx, true)).To(Succeed())
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

			kubeAPIServer.EXPECT().SetSNIConfig(gomock.Any())
			kubeAPIServer.EXPECT().Destroy(ctx)

			Expect(botanist.DeleteKubeAPIServer(ctx)).To(Succeed())

			shootClient, err = clientMap.GetClient(ctx, keys.ForShoot(botanist.Shoot.GetInfo()))
			Expect(err).To(MatchError(`clientSet for key "` + botanist.Shoot.GetInfo().Namespace + `/` + botanist.Shoot.GetInfo().Name + `" not found`))
			Expect(shootClient).To(BeNil())

			Expect(botanist.ShootClientSet).To(BeNil())
		})
	})

	Describe("#ScaleKubeAPIServerToOne", func() {
		deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "kube-apiserver", Namespace: controlPlaneNamespace}}

		It("should scale the deployment", func() {
			Expect(seedClient.Create(ctx, deployment)).To(Succeed())
			Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(Succeed())

			kubeAPIServer.EXPECT().SetAutoscalingReplicas(gomock.Any())

			Expect(botanist.ScaleKubeAPIServerToOne(ctx)).To(Succeed())

			Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(Succeed())
			Expect(deployment.Spec.Replicas).To(PointTo(Equal(int32(1))))
		})
	})
})

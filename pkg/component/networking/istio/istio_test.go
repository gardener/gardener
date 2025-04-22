// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package istio_test

import (
	"context"
	"os"
	"strconv"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	istionetworkingv1alpha3 "istio.io/client-go/pkg/apis/networking/v1alpha3"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/chartrenderer"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/component/networking/istio"
	"github.com/gardener/gardener/pkg/features"
	gardenletfeatures "github.com/gardener/gardener/pkg/gardenlet/features"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("istiod", func() {
	const (
		deployNS        = "test"
		deployNSIngress = "test-ingress"
	)

	var (
		ctx                           context.Context
		c                             client.Client
		istiod                        Interface
		igw                           []IngressGatewayValues
		igwAnnotations                map[string]string
		labels                        map[string]string
		networkLabels                 map[string]string
		expectAPIServerTLSTermination bool

		managedResourceIstioName   string
		managedResourceIstio       *resourcesv1alpha1.ManagedResource
		managedResourceIstioSecret *corev1.Secret

		managedResourceIstioSystemName   string
		managedResourceIstioSystem       *resourcesv1alpha1.ManagedResource
		managedResourceIstioSystemSecret *corev1.Secret

		renderer chartrenderer.Interface

		minReplicas = 2
		maxReplicas = 9

		externalTrafficPolicy corev1.ServiceExternalTrafficPolicy

		istiodService = func() string {
			data, _ := os.ReadFile("./test_charts/istiod_service.yaml")
			return string(data)
		}

		istioClusterRole = func(i int) string {
			data, _ := os.ReadFile("./test_charts/istio_clusterrole.yaml")
			return strings.Split(string(data), "---\n")[i]
		}

		istiodClusterRoleBinding = func(i int) string {
			data, _ := os.ReadFile("./test_charts/istio_clusterrolebinding.yaml")
			return strings.Split(string(data), "---\n")[i]
		}

		istiodDestinationRule = func() string {
			data, _ := os.ReadFile("./test_charts/istiod_destinationrule.yaml")
			return string(data)
		}

		istiodPodDisruptionBudget = func() string {
			data, _ := os.ReadFile("./test_charts/istiod_poddisruptionbudget.yaml")
			return string(data)
		}

		istiodRole = func() string {
			data, _ := os.ReadFile("./test_charts/istiod_role.yaml")
			return string(data)
		}

		istiodRoleBinding = func() string {
			data, _ := os.ReadFile("./test_charts/istiod_rolebinding.yaml")
			return string(data)
		}

		istiodServiceAccount = func() string {
			data, _ := os.ReadFile("./test_charts/istiod_serviceaccount.yaml")
			return string(data)
		}

		istiodAutoscale = func() string {
			data, _ := os.ReadFile("./test_charts/istiod_autoscale.yaml")
			return string(data)
		}

		istiodValidationWebhook = func() string {
			data, _ := os.ReadFile("./test_charts/istiod_validationwebhook.yaml")
			return string(data)
		}

		istiodConfigMap = func() string {
			data, _ := os.ReadFile("./test_charts/istiod_configmap.yaml")
			return string(data)
		}

		istiodDeployment = func(checksum string) string {
			data, _ := os.ReadFile("./test_charts/istiod_deployment.yaml")
			return strings.ReplaceAll(string(data), "<CHECKSUM>", checksum)
		}

		istiodServiceMonitor = func() string {
			data, _ := os.ReadFile("./test_charts/istiod_servicemonitor.yaml")
			return string(data)
		}

		istioIngressAutoscaler = func(min *int, max *int) string {
			data, _ := os.ReadFile("./test_charts/ingress_autoscaler.yaml")
			str := strings.ReplaceAll(string(data), "<MIN_REPLICAS>", strconv.Itoa(ptr.Deref(min, 2)))
			str = strings.ReplaceAll(str, "<MAX_REPLICAS>", strconv.Itoa(ptr.Deref(max, 9)))
			return str
		}

		istioIngressEnvoyVPNFilter = func(i int) string {
			data, _ := os.ReadFile("./test_charts/ingress_vpn_envoy_filter.yaml")
			return strings.Split(string(data), "---\n")[i]
		}

		istioIngressEnvoyFilter = func() string {
			data, _ := os.ReadFile("./test_charts/ingress_envoyfilter.yaml")
			return string(data)
		}

		istioIngressVPNGateway = func() string {
			data, _ := os.ReadFile("./test_charts/ingress_vpn_gateway.yaml")
			return string(data)
		}

		istioIngressPodDisruptionBudget = func() string {
			data, _ := os.ReadFile("./test_charts/ingress_poddisruptionbudget.yaml")
			return string(data)
		}

		istioIngressRole = func() string {
			data, _ := os.ReadFile("./test_charts/ingress_role.yaml")
			return string(data)
		}

		istioIngressRoleBinding = func() string {
			data, _ := os.ReadFile("./test_charts/ingress_rolebinding.yaml")
			return string(data)
		}

		istioIngressService = func() string {
			data, _ := os.ReadFile("./test_charts/ingress_service.yaml")
			return string(data)
		}

		istioIngressServiceDualStack = func() string {
			data, _ := os.ReadFile("./test_charts/ingress_service_dualstack.yaml")
			return string(data)
		}

		istioIngressServiceDualStackETP = func() string {
			data, _ := os.ReadFile("./test_charts/ingress_service_dualstack_etp.yaml")
			return string(data)
		}

		istioIngressServiceETPCluster = func() string {
			data, _ := os.ReadFile("./test_charts/ingress_service_etp_cluster.yaml")
			return string(data)
		}

		istioIngressServiceETPLocal = func() string {
			data, _ := os.ReadFile("./test_charts/ingress_service_etp_local.yaml")
			return string(data)
		}

		istioIngressServiceAccount = func() string {
			data, _ := os.ReadFile("./test_charts/ingress_serviceaccount.yaml")
			return string(data)
		}

		istioIngressDeployment = func(replicas *int) string {
			data, _ := os.ReadFile("./test_charts/ingress_deployment.yaml")
			return strings.ReplaceAll(string(data), "<REPLICAS>", strconv.Itoa(ptr.Deref(replicas, 2)))
		}

		istioIngressServiceMonitor = func() string {
			data, _ := os.ReadFile("./test_charts/ingress_servicemonitor.yaml")
			return string(data)
		}

		istioProxyProtocolEnvoyFilter = func() string {
			data, _ := os.ReadFile("./test_charts/proxyprotocol_envoyfilter.yaml")
			return string(data)
		}

		istioProxyProtocolEnvoyFilterDual = func() string {
			data, _ := os.ReadFile("./test_charts/proxyprotocol_envoyfilter_dual_proxy_protocol.yaml")
			return string(data)
		}

		istioProxyProtocolEnvoyFilterSNI = func() string {
			data, _ := os.ReadFile("./test_charts/proxyprotocol_envoyfilter_sni.yaml")
			return string(data)
		}

		istioProxyProtocolEnvoyFilterVPN = func() string {
			data, _ := os.ReadFile("./test_charts/proxyprotocol_envoyfilter_vpn.yaml")
			return string(data)
		}

		istioProxyProtocolGateway = func() string {
			data, _ := os.ReadFile("./test_charts/proxyprotocol_gateway.yaml")
			return string(data)
		}

		istioProxyProtocolVirtualService = func() string {
			data, _ := os.ReadFile("./test_charts/proxyprotocol_virtualservice.yaml")
			return string(data)
		}

		istioAPIServerTLSTerminationEnvoyFilter = func() string {
			data, _ := os.ReadFile("./test_charts/apiserver_tls_termination.yaml")
			return string(data)
		}

		istioStripTrailingDotEnvoyFilter = func() string {
			data, _ := os.ReadFile("./test_charts/strip_trailing_dot_envoyfilter.yaml")
			return string(data)
		}
	)

	BeforeEach(func() {
		ctx = context.Background()
		igwAnnotations = map[string]string{"foo": "bar"}
		labels = map[string]string{"foo": "bar"}
		networkLabels = map[string]string{"to-target": "allowed"}
		expectAPIServerTLSTermination = false

		c = fake.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		renderer = chartrenderer.NewWithServerVersion(&version.Info{GitVersion: "v1.31.1"})

		gardenletfeatures.RegisterFeatureGates()

		igw = makeIngressGateway(deployNSIngress, igwAnnotations, labels, networkLabels)

		istiod = NewIstio(
			c,
			renderer,
			Values{
				Istiod: IstiodValues{
					Enabled:           true,
					Image:             "foo/bar",
					Namespace:         deployNS,
					PriorityClassName: v1beta1constants.PriorityClassNameSeedSystemCritical,
					TrustDomain:       "foo.local",
					Zones:             []string{"a", "b", "c"},
				},
				IngressGateway: igw,
			},
		)

		managedResourceIstioName = "istio"
		managedResourceIstio = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      managedResourceIstioName,
				Namespace: deployNS,
			},
		}
		managedResourceIstioSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "managedresource-" + managedResourceIstio.Name,
				Namespace: deployNS,
			},
		}

		managedResourceIstioSystemName = "istio-system"
		managedResourceIstioSystem = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      managedResourceIstioSystemName,
				Namespace: deployNS,
			},
		}
		managedResourceIstioSystemSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "managedresource-" + managedResourceIstioSystem.Name,
				Namespace: deployNS,
			},
		}
	})

	Describe("#Deploy", func() {
		JustBeforeEach(func() {
			Expect(istiod.Deploy(ctx)).ToNot(HaveOccurred(), "istiod deploy succeeds")

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceIstio), managedResourceIstio)).To(Succeed())
			managedResourceIstioSecret.Name = managedResourceIstio.Spec.SecretRefs[0].Name
		})

		It("deploys istiod namespace", func() {
			actualNS := &corev1.Namespace{}

			Expect(c.Get(ctx, client.ObjectKey{Name: deployNS}, actualNS)).ToNot(HaveOccurred())

			Expect(actualNS.Labels).To(And(
				HaveKeyWithValue("istio-operator-managed", "Reconcile"),
				HaveKeyWithValue("istio-injection", "disabled"),
				HaveKeyWithValue("pod-security.kubernetes.io/enforce", "baseline"),
				HaveKeyWithValue("high-availability-config.resources.gardener.cloud/consider", "true"),
				HaveKeyWithValue("gardener.cloud/role", "istio-system"),
			))
			Expect(actualNS.Annotations).To(And(
				HaveKeyWithValue("high-availability-config.resources.gardener.cloud/zones", "a,b,c"),
			))
		})

		It("deploys istio-ingress namespace", func() {
			actualNS := &corev1.Namespace{}

			Expect(c.Get(ctx, client.ObjectKey{Name: deployNSIngress}, actualNS)).ToNot(HaveOccurred())

			Expect(actualNS.Labels).To(And(
				HaveKeyWithValue("istio-operator-managed", "Reconcile"),
				HaveKeyWithValue("istio-injection", "disabled"),
				HaveKeyWithValue("pod-security.kubernetes.io/enforce", "baseline"),
				HaveKeyWithValue("high-availability-config.resources.gardener.cloud/consider", "true"),
				HaveKeyWithValue("gardener.cloud/role", "istio-ingress"),
			))
			Expect(actualNS.Annotations).To(And(
				HaveKeyWithValue("high-availability-config.resources.gardener.cloud/zones", "a,b,c"),
			))
		})

		checkSuccessfulDeployment := func(minReplicas, maxReplicas *int) {
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceIstio), managedResourceIstio)).To(Succeed())
			expectedMr := &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:            managedResourceIstioName,
					Namespace:       deployNS,
					Labels:          map[string]string{"gardener.cloud/role": "seed-system-component"},
					ResourceVersion: "1",
				},
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					Class: ptr.To("seed"),
					SecretRefs: []corev1.LocalObjectReference{{
						Name: managedResourceIstioSecret.Name,
					}},
					KeepObjects: ptr.To(false),
				},
			}
			utilruntime.Must(references.InjectAnnotations(expectedMr))
			Expect(managedResourceIstio).To(Equal(expectedMr))

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceIstioSecret), managedResourceIstioSecret)).To(Succeed())
			Expect(managedResourceIstioSecret.Type).To(Equal(corev1.SecretTypeOpaque))
			Expect(managedResourceIstioSecret.Immutable).To(Equal(ptr.To(true)))
			Expect(managedResourceIstioSecret.Labels["resources.gardener.cloud/garbage-collectable-reference"]).To(Equal("true"))

			expectedIstioManifests := []string{
				istioIngressAutoscaler(minReplicas, maxReplicas),
				istioIngressRole(),
				istioIngressRoleBinding(),
				istioIngressService(),
				istioIngressServiceAccount(),
				istioIngressDeployment(minReplicas),
				istioIngressServiceMonitor(),
				istioIngressEnvoyFilter(),
			}

			expectedIstioSystemManifests := []string{
				istiodConfigMap(),
				istiodDeployment("1cb4501d4e8d2a8849d21c2aa5e0910c3ea03818bd9b322082fd9c6a8605f097"),
				istiodService(),
				istioClusterRole(0),
				istioClusterRole(1),
				istioClusterRole(2),
				istiodClusterRoleBinding(0),
				istiodClusterRoleBinding(1),
				istiodClusterRoleBinding(2),
				istiodDestinationRule(),
				istiodRole(),
				istiodRoleBinding(),
				istiodServiceAccount(),
				istiodAutoscale(),
				istiodValidationWebhook(),
				istiodServiceMonitor(),
			}

			expectedIstioManifests = append(expectedIstioManifests, istioIngressPodDisruptionBudget())
			expectedIstioSystemManifests = append(expectedIstioSystemManifests, istiodPodDisruptionBudget())

			if expectAPIServerTLSTermination {
				expectedIstioManifests = append(expectedIstioManifests, istioAPIServerTLSTerminationEnvoyFilter())
				expectedIstioManifests = append(expectedIstioManifests, istioStripTrailingDotEnvoyFilter())
			}

			if igw[0].TerminateLoadBalancerProxyProtocol && !igw[0].ProxyProtocolEnabled {
				expectedIstioManifests = append(expectedIstioManifests, istioProxyProtocolEnvoyFilterSNI(), istioProxyProtocolEnvoyFilterVPN())
			}

			if igw[0].TerminateLoadBalancerProxyProtocol && igw[0].ProxyProtocolEnabled {
				expectedIstioManifests = append(expectedIstioManifests, istioProxyProtocolEnvoyFilterDual(), istioProxyProtocolEnvoyFilterSNI(), istioProxyProtocolEnvoyFilterVPN())
			}

			if !igw[0].TerminateLoadBalancerProxyProtocol && igw[0].ProxyProtocolEnabled {
				expectedIstioManifests = append(expectedIstioManifests, istioProxyProtocolEnvoyFilter())
			}

			if igw[0].ProxyProtocolEnabled {
				expectedIstioManifests = append(expectedIstioManifests, istioProxyProtocolGateway(), istioProxyProtocolVirtualService())
			}

			if igw[0].VPNEnabled {
				expectedIstioManifests = append(expectedIstioManifests, istioIngressVPNGateway(), istioIngressEnvoyVPNFilter(0), istioIngressEnvoyVPNFilter(1))
			}

			By("Verify istio resources")
			var err error
			istioManifests, err := test.ExtractManifestsFromManagedResourceData(managedResourceIstioSecret.Data)
			Expect(err).NotTo(HaveOccurred())

			Expect(istioManifests).To(ConsistOf(expectedIstioManifests))

			By("Verify istio-system resources")
			if istiod.GetValues().Istiod.Enabled {
				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceIstioSystem), managedResourceIstioSystem)).To(Succeed())
				managedResourceIstioSystemSecret.Name = managedResourceIstioSystem.Spec.SecretRefs[0].Name

				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceIstioSystemSecret), managedResourceIstioSystemSecret)).To(Succeed())
				Expect(managedResourceIstioSystemSecret.Type).To(Equal(corev1.SecretTypeOpaque))
				Expect(managedResourceIstioSystemSecret.Immutable).To(Equal(ptr.To(true)))
				Expect(managedResourceIstioSystemSecret.Labels["resources.gardener.cloud/garbage-collectable-reference"]).To(Equal("true"))

				istioSystemManifests, err := test.ExtractManifestsFromManagedResourceData(managedResourceIstioSystemSecret.Data)
				Expect(err).NotTo(HaveOccurred())
				Expect(istioSystemManifests).To(ContainElements(expectedIstioSystemManifests))
			} else {
				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceIstioSystem), managedResourceIstioSystem)).To(BeNotFoundError())
			}
		}

		Context("with proxy protocol in apiserver-proxy and with proxy protocol termination", func() {
			BeforeEach(func() {
				igw[0].TerminateLoadBalancerProxyProtocol = true
				igw[0].ProxyProtocolEnabled = true
			})

			It("should successfully deploy all resources", func() {
				checkSuccessfulDeployment(nil, nil)
			})
		})

		Context("without proxy protocol in apiserver-proxy and proxy protocol termination", func() {
			BeforeEach(func() {
				igw[0].TerminateLoadBalancerProxyProtocol = true
				igw[0].ProxyProtocolEnabled = false
			})

			It("should successfully deploy all resources", func() {
				checkSuccessfulDeployment(nil, nil)
			})
		})

		Context("without proxy protocol termination", func() {
			BeforeEach(func() {
				igw[0].TerminateLoadBalancerProxyProtocol = false
			})

			It("should successfully deploy all resources", func() {
				checkSuccessfulDeployment(nil, nil)
			})
		})

		Context("with outdated stats filters", func() {
			var statsFilterNames []string

			BeforeEach(func() {
				statsFilterNames = []string{"tcp-stats-filter-1.11", "stats-filter-1.11", "tcp-stats-filter-1.12", "stats-filter-1.12"}

				for _, ingressGateway := range igw {
					for _, statsFilterName := range statsFilterNames {
						statsFilter := istionetworkingv1alpha3.EnvoyFilter{
							ObjectMeta: metav1.ObjectMeta{
								Name:      statsFilterName,
								Namespace: ingressGateway.Namespace,
							},
						}
						Expect(c.Create(ctx, &statsFilter)).To(Succeed())
					}
				}
			})

			It("should have removed all outdated stats filters", func() {
				for _, ingressGateway := range igw {
					for _, statsFilterName := range statsFilterNames {
						statsFilter := &istionetworkingv1alpha3.EnvoyFilter{
							ObjectMeta: metav1.ObjectMeta{
								Name:      statsFilterName,
								Namespace: ingressGateway.Namespace,
							},
						}
						Expect(c.Get(ctx, client.ObjectKeyFromObject(statsFilter), statsFilter)).To(BeNotFoundError())
					}
				}
			})
		})

		Context("horizontal ingress gateway scaling", func() {
			BeforeEach(func() {
				minReplicas = 3
				maxReplicas = 8
				igw[0].MinReplicas = &minReplicas
				igw[0].MaxReplicas = &maxReplicas
				istiod = NewIstio(
					c,
					renderer,
					Values{
						Istiod: IstiodValues{
							Enabled:           true,
							Image:             "foo/bar",
							Namespace:         deployNS,
							PriorityClassName: v1beta1constants.PriorityClassNameSeedSystemCritical,
							TrustDomain:       "foo.local",
							Zones:             []string{"a", "b", "c"},
						},
						IngressGateway: igw,
					},
				)
			})

			It("should successfully deploy correct autoscaling", func() {
				checkSuccessfulDeployment(&minReplicas, &maxReplicas)
			})
		})

		Context("external traffic policy cluster", func() {
			BeforeEach(func() {
				externalTrafficPolicy = corev1.ServiceExternalTrafficPolicyCluster
				igw[0].ExternalTrafficPolicy = &externalTrafficPolicy
				istiod = NewIstio(
					c,
					renderer,
					Values{
						Istiod: IstiodValues{
							Enabled:     true,
							Image:       "foo/bar",
							Namespace:   deployNS,
							TrustDomain: "foo.local",
							Zones:       []string{"a", "b", "c"},
						},
						IngressGateway: igw,
					},
				)
			})

			It("should successfully deploy correct external traffic policy", func() {
				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceIstioSecret), managedResourceIstioSecret)).To(Succeed())

				var err error
				istioManifests, err := test.ExtractManifestsFromManagedResourceData(managedResourceIstioSecret.Data)
				Expect(err).NotTo(HaveOccurred())

				Expect(istioManifests).To(ContainElement(istioIngressServiceETPCluster()))
			})
		})

		Context("external traffic policy local", func() {
			BeforeEach(func() {
				externalTrafficPolicy = corev1.ServiceExternalTrafficPolicyLocal
				igw[0].ExternalTrafficPolicy = &externalTrafficPolicy
				istiod = NewIstio(
					c,
					renderer,
					Values{
						Istiod: IstiodValues{
							Enabled:     true,
							Image:       "foo/bar",
							Namespace:   deployNS,
							TrustDomain: "foo.local",
							Zones:       []string{"a", "b", "c"},
						},
						IngressGateway: igw,
					},
				)
			})

			It("should successfully deploy correct external traffic policy", func() {
				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceIstioSecret), managedResourceIstioSecret)).To(Succeed())

				var err error
				istioManifests, err := test.ExtractManifestsFromManagedResourceData(managedResourceIstioSecret.Data)
				Expect(err).NotTo(HaveOccurred())

				Expect(istioManifests).To(ContainElement(istioIngressServiceETPLocal()))
			})
		})

		Context("dual stack istio service", func() {
			BeforeEach(func() {
				externalTrafficPolicy = corev1.ServiceExternalTrafficPolicyLocal
				igw[0].ExternalTrafficPolicy = &externalTrafficPolicy
				igw[0].DualStack = true
				istiod = NewIstio(
					c,
					renderer,
					Values{
						Istiod: IstiodValues{
							Enabled:     true,
							Image:       "foo/bar",
							Namespace:   deployNS,
							TrustDomain: "foo.local",
							Zones:       []string{"a", "b", "c"},
						},
						IngressGateway: igw,
					},
				)
			})

			It("should successfully deploy correct dualStack config and traffic policy local", func() {
				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceIstioSecret), managedResourceIstioSecret)).To(Succeed())

				var err error
				istioManifests, err := test.ExtractManifestsFromManagedResourceData(managedResourceIstioSecret.Data)
				Expect(err).NotTo(HaveOccurred())

				Expect(istioManifests).To(ContainElement(istioIngressServiceDualStackETP()))
			})
		})

		Context("dual stack istio service with traffic policy local", func() {
			BeforeEach(func() {
				igw[0].DualStack = true
				istiod = NewIstio(
					c,
					renderer,
					Values{
						Istiod: IstiodValues{
							Enabled:     true,
							Image:       "foo/bar",
							Namespace:   deployNS,
							TrustDomain: "foo.local",
							Zones:       []string{"a", "b", "c"},
						},
						IngressGateway: igw,
					},
				)
			})

			It("should successfully deploy correct dualStack config", func() {
				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceIstioSecret), managedResourceIstioSecret)).To(Succeed())

				var err error
				istioManifests, err := test.ExtractManifestsFromManagedResourceData(managedResourceIstioSecret.Data)
				Expect(err).NotTo(HaveOccurred())

				Expect(istioManifests).To(ContainElement(istioIngressServiceDualStack()))
			})
		})

		Context("VPN disabled", func() {
			BeforeEach(func() {
				for i := range igw {
					igw[i].VPNEnabled = false
				}

				istiod = NewIstio(
					c,
					renderer,
					Values{
						Istiod: IstiodValues{
							Enabled:           true,
							Image:             "foo/bar",
							Namespace:         deployNS,
							PriorityClassName: v1beta1constants.PriorityClassNameSeedSystemCritical,
							TrustDomain:       "foo.local",
							Zones:             []string{"a", "b", "c"},
						},
						IngressGateway: igw,
					},
				)
			})

			It("should successfully deploy all resources", func() {
				checkSuccessfulDeployment(nil, nil)
			})
		})

		Context("Proxy Protocol disabled", func() {
			BeforeEach(func() {
				for i := range igw {
					igw[i].ProxyProtocolEnabled = false
				}

				istiod = NewIstio(
					c,
					renderer,
					Values{
						Istiod: IstiodValues{
							Enabled:           true,
							Image:             "foo/bar",
							Namespace:         deployNS,
							PriorityClassName: v1beta1constants.PriorityClassNameSeedSystemCritical,
							TrustDomain:       "foo.local",
							Zones:             []string{"a", "b", "c"},
						},
						IngressGateway: igw,
					},
				)
			})

			It("should successfully deploy all resources", func() {
				checkSuccessfulDeployment(nil, nil)
			})
		})

		Context("istiod disabled", func() {
			BeforeEach(func() {
				for i := range igw {
					igw[i].ProxyProtocolEnabled = false
				}

				istiod = NewIstio(
					c,
					renderer,
					Values{
						Istiod: IstiodValues{
							Enabled:     false,
							Image:       "foo/bar",
							Namespace:   deployNS,
							TrustDomain: "foo.local",
							Zones:       []string{"a", "b", "c"},
						},
						IngressGateway: igw,
					},
				)
			})

			It("should successfully deploy all resources", func() {
				checkSuccessfulDeployment(nil, nil)
			})
		})

		Context("With IstioTLSTermination feature gate enabled", func() {
			BeforeEach(func() {
				expectAPIServerTLSTermination = true
				DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.IstioTLSTermination, true))
			})

			It("should successfully deploy all resources", func() {
				checkSuccessfulDeployment(nil, nil)
			})
		})

		Context("With IstioTLSTermination feature gate disabled but with shoots still using the feature", func() {
			BeforeEach(func() {
				expectAPIServerTLSTermination = true

				envoyFilter := istionetworkingv1alpha3.EnvoyFilter{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "shoot--foo--bar-istio-tls-termination",
						Namespace: "test-ingress",
					},
				}
				Expect(c.Create(ctx, &envoyFilter)).To(Succeed())
				DeferCleanup(func() { Expect(c.Delete(ctx, &envoyFilter)).To(Succeed()) })
			})

			It("should successfully deploy all resources", func() {
				checkSuccessfulDeployment(nil, nil)
			})
		})
	})

	Describe("#Destroy", func() {
		var (
			oldMrSecret       *corev1.Secret
			oldMrSystemSecret *corev1.Secret
		)

		BeforeEach(func() {
			Expect(istiod.Deploy(ctx)).To(Succeed())
			oldMrSecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "managedresource-" + managedResourceIstio.Name,
					Namespace: deployNS,
				},
			}
			oldMrSystemSecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "managedresource-" + managedResourceIstioSystem.Name,
					Namespace: deployNS,
				},
			}
			Expect(c.Create(ctx, oldMrSecret)).To(Succeed())
			Expect(c.Create(ctx, oldMrSystemSecret)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceIstio), managedResourceIstio)).To(Succeed())
			managedResourceIstioSecret.Name = managedResourceIstio.Spec.SecretRefs[0].Name

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceIstioSystem), managedResourceIstioSystem)).To(Succeed())
			managedResourceIstioSystemSecret.Name = managedResourceIstioSystem.Spec.SecretRefs[0].Name
		})

		It("should successfully destroy all resources", func() {
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceIstio), managedResourceIstio)).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceIstioSecret), managedResourceIstioSecret)).To(Succeed())

			Expect(istiod.Destroy(ctx)).To(Succeed())

			namespace := &corev1.Namespace{}
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceIstio), managedResourceIstio)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceIstioSecret), managedResourceIstioSecret)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceIstioSystem), managedResourceIstioSystem)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceIstioSystemSecret), managedResourceIstioSystemSecret)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(oldMrSecret), oldMrSecret)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(oldMrSystemSecret), oldMrSystemSecret)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKey{Name: deployNS}, namespace)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKey{Name: deployNSIngress}, namespace)).To(BeNotFoundError())
		})

		Context("istiod disabled", func() {
			It("should not destroy istiod resources", func() {
				istiod = NewIstio(
					c,
					renderer,
					Values{
						Istiod: IstiodValues{
							Enabled:     false,
							Image:       "foo/bar",
							Namespace:   deployNS,
							TrustDomain: "foo.local",
						},
						IngressGateway: igw,
					},
				)

				Expect(istiod.Destroy(ctx)).To(Succeed())

				namespace := &corev1.Namespace{}
				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceIstioSystem), managedResourceIstio)).To(Succeed())
				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceIstioSystemSecret), managedResourceIstioSecret)).To(Succeed())
				Expect(c.Get(ctx, client.ObjectKey{Name: deployNS}, namespace)).To(Succeed())
				Expect(c.Get(ctx, client.ObjectKey{Name: deployNSIngress}, namespace)).To(BeNotFoundError())
			})
		})
	})

	Describe("#AddIngressGateway", func() {
		It("should add the given ingress gateway", func() {
			igValues := IngressGatewayValues{
				Namespace: "additition-istio-ingress",
			}

			istiod.AddIngressGateway(igValues)

			igwLen := len(istiod.GetValues().IngressGateway)
			Expect(igwLen).To(Equal(len(igw) + 1))
			Expect(istiod.GetValues().IngressGateway[igwLen-1]).To(Equal(igValues))
		})
	})

	Context("waiting functions", func() {
		var (
			fakeOps *retryfake.Ops
		)

		BeforeEach(func() {
			fakeOps = &retryfake.Ops{MaxAttempts: 1}

			DeferCleanup(test.WithVars(
				&retry.Until, fakeOps.Until,
				&retry.UntilTimeout, fakeOps.UntilTimeout,
			))
		})

		Describe("#Wait", func() {
			It("should fail because reading the ManagedResource fails", func() {
				Expect(istiod.Wait(ctx)).To(MatchError(ContainSubstring("not found")))
			})

			It("should fail because the ManagedResource doesn't become healthy", func() {
				fakeOps.MaxAttempts = 2

				Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceIstioName,
						Namespace:  deployNS,
						Generation: 1,
					},
					Status: resourcesv1alpha1.ManagedResourceStatus{
						ObservedGeneration: 1,
						Conditions: []gardencorev1beta1.Condition{
							{
								Type:   resourcesv1alpha1.ResourcesApplied,
								Status: gardencorev1beta1.ConditionFalse,
							},
							{
								Type:   resourcesv1alpha1.ResourcesHealthy,
								Status: gardencorev1beta1.ConditionFalse,
							},
						},
					},
				})).To(Succeed())

				Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:       managedResourceIstioSystemName,
						Namespace:  deployNS,
						Generation: 1,
					},
					Status: resourcesv1alpha1.ManagedResourceStatus{
						ObservedGeneration: 1,
						Conditions: []gardencorev1beta1.Condition{
							{
								Type:   resourcesv1alpha1.ResourcesApplied,
								Status: gardencorev1beta1.ConditionTrue,
							},
							{
								Type:   resourcesv1alpha1.ResourcesHealthy,
								Status: gardencorev1beta1.ConditionTrue,
							},
						},
					},
				})).To(Succeed())

				Expect(istiod.Wait(ctx)).To(MatchError(ContainSubstring("is not healthy")))
			})

			It("should successfully wait for the managed resource to become healthy", func() {
				fakeOps.MaxAttempts = 2

				for _, mr := range []string{managedResourceIstioName, managedResourceIstioSystemName} {
					Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{
						ObjectMeta: metav1.ObjectMeta{
							Name:       mr,
							Namespace:  deployNS,
							Generation: 1,
						},
						Status: resourcesv1alpha1.ManagedResourceStatus{
							ObservedGeneration: 1,
							Conditions: []gardencorev1beta1.Condition{
								{
									Type:   resourcesv1alpha1.ResourcesApplied,
									Status: gardencorev1beta1.ConditionTrue,
								},
								{
									Type:   resourcesv1alpha1.ResourcesHealthy,
									Status: gardencorev1beta1.ConditionTrue,
								},
							},
						},
					})).To(Succeed())
				}

				Expect(istiod.Wait(ctx)).To(Succeed())
			})
		})

		Describe("#WaitCleanup", func() {
			It("should fail when the wait for the managed resource deletion times out", func() {
				fakeOps.MaxAttempts = 2

				Expect(c.Create(ctx, managedResourceIstio)).To(Succeed())

				Expect(istiod.WaitCleanup(ctx)).To(MatchError(ContainSubstring("still exists")))
			})

			It("should not return an error when it's already removed", func() {
				Expect(istiod.WaitCleanup(ctx)).To(Succeed())
			})
		})
	})
})

func makeIngressGateway(namespace string, annotations, labels map[string]string, networkPolicyLabels map[string]string) []IngressGatewayValues {
	return []IngressGatewayValues{
		{
			Image:               "foo/bar",
			TrustDomain:         "foo.bar",
			IstiodNamespace:     "istio-test-system",
			Annotations:         annotations,
			Labels:              labels,
			NetworkPolicyLabels: networkPolicyLabels,
			Ports: []corev1.ServicePort{
				{Name: "foo", Port: 999, TargetPort: intstr.FromInt32(999)},
			},
			Namespace:                          namespace,
			PriorityClassName:                  v1beta1constants.PriorityClassNameSeedSystemCritical,
			ProxyProtocolEnabled:               true,
			TerminateLoadBalancerProxyProtocol: false,
			VPNEnabled:                         true,
		},
	}
}

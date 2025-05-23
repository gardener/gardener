// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package apiserverexposure_test

import (
	"context"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
	istioapinetworkingv1beta1 "istio.io/api/networking/v1beta1"
	istionetworkingv1alpha3 "istio.io/client-go/pkg/apis/networking/v1alpha3"
	istionetworkingv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	. "github.com/gardener/gardener/pkg/component/kubernetes/apiserverexposure"
	comptest "github.com/gardener/gardener/pkg/component/test"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("#SNI", func() {
	var (
		ctx context.Context
		c   client.Client
		sm  secretsmanager.Interface

		defaultDepWaiter component.DeployWaiter

		apiServerProxyValues        *APIServerProxy
		namespace                   string
		istioLabels                 map[string]string
		istioWildcardLabels         map[string]string
		istioNamespace              string
		istioWildcardNamespace      string
		istioTLSTermination         bool
		hosts                       []string
		hostName                    string
		connectionUpgradeHostName   string
		wildcardConfiguration       *WildcardConfiguration
		wildcardHosts               []string
		wildcardTLSSecret           corev1.Secret
		wildcardIstioIngressGateway *IstioIngressGateway

		expectedDestinationRule                                  *istionetworkingv1beta1.DestinationRule
		expectedGateway                                          *istionetworkingv1beta1.Gateway
		expectedWildcardGateway                                  *istionetworkingv1beta1.Gateway
		expectedVirtualService                                   *istionetworkingv1beta1.VirtualService
		expectedWildcardVirtualService                           *istionetworkingv1beta1.VirtualService
		expectedEnvoyFilterObjectMetaAPIServerProxy              metav1.ObjectMeta
		expectedEnvoyFilterObjectMetaIstioTLSTermination         metav1.ObjectMeta
		expectedWildcardEnvoyFilterObjectMetaIstioTLSTermination metav1.ObjectMeta
		expectedSecretObjectMetaIstioMTLS                        metav1.ObjectMeta
		expectedWildcardSecretObjectMetaIstioMTLS                metav1.ObjectMeta
		expectedSecretObjectMetaIstioTLS                         metav1.ObjectMeta
		expectedManagedResourceSNI                               *resourcesv1alpha1.ManagedResource
		expectedManagedResourceTLSSecrets                        *resourcesv1alpha1.ManagedResource
	)

	BeforeEach(func() {
		ctx = context.TODO()

		s := runtime.NewScheme()
		Expect(corev1.AddToScheme(s)).To(Succeed())
		Expect(resourcesv1alpha1.AddToScheme(s)).To(Succeed())
		Expect(istionetworkingv1beta1.AddToScheme(s)).To(Succeed())
		Expect(istionetworkingv1alpha3.AddToScheme(s)).To(Succeed())
		c = fake.NewClientBuilder().WithScheme(s).Build()

		apiServerProxyValues = &APIServerProxy{
			APIServerClusterIP: "1.1.1.1",
			UseProxyProtocol:   true,
		}
		namespace = "test-namespace"
		istioLabels = map[string]string{"foo": "bar"}
		istioNamespace = "istio-foo"
		istioWildcardLabels = map[string]string{"bar": "foo"}
		istioWildcardNamespace = "istio-bar"
		istioTLSTermination = false
		hosts = []string{"foo.bar"}
		hostName = "kube-apiserver." + namespace + ".svc.cluster.local"
		connectionUpgradeHostName = "kube-apiserver-connection-upgrade." + namespace + ".svc.cluster.local"
		wildcardConfiguration = nil
		wildcardHosts = []string{"foo.wildcard", "bar.wildcard"}
		wildcardTLSSecret = corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "wildcard-tls-secret",
				Namespace: namespace,
			},
		}
		wildcardIstioIngressGateway = &IstioIngressGateway{
			Labels:    istioWildcardLabels,
			Namespace: istioWildcardNamespace,
		}

		sm = fakesecretsmanager.New(c, namespace)

		expectedDestinationRule = &istionetworkingv1beta1.DestinationRule{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kube-apiserver",
				Namespace: namespace,
				Labels: map[string]string{
					"app":  "kubernetes",
					"role": "apiserver",
				},
				ResourceVersion: "1",
			},
			Spec: istioapinetworkingv1beta1.DestinationRule{
				ExportTo: []string{"*"},
				Host:     hostName,
				TrafficPolicy: &istioapinetworkingv1beta1.TrafficPolicy{
					ConnectionPool: &istioapinetworkingv1beta1.ConnectionPoolSettings{
						Tcp: &istioapinetworkingv1beta1.ConnectionPoolSettings_TCPSettings{
							MaxConnections: 5000,
							TcpKeepalive: &istioapinetworkingv1beta1.ConnectionPoolSettings_TCPSettings_TcpKeepalive{
								Time:     &durationpb.Duration{Seconds: 7200},
								Interval: &durationpb.Duration{Seconds: 75},
							},
						},
					},
					LoadBalancer: &istioapinetworkingv1beta1.LoadBalancerSettings{
						LocalityLbSetting: &istioapinetworkingv1beta1.LocalityLoadBalancerSetting{
							Enabled:          &wrapperspb.BoolValue{Value: true},
							FailoverPriority: []string{"topology.kubernetes.io/zone"},
						},
					},
					OutlierDetection: &istioapinetworkingv1beta1.OutlierDetection{
						MinHealthPercent: 0,
					},
					Tls: &istioapinetworkingv1beta1.ClientTLSSettings{
						Mode: istioapinetworkingv1beta1.ClientTLSSettings_DISABLE,
					},
				},
			},
		}
		expectedEnvoyFilterObjectMetaAPIServerProxy = metav1.ObjectMeta{
			Name:      namespace + "-apiserver-proxy",
			Namespace: istioNamespace,
		}
		expectedEnvoyFilterObjectMetaIstioTLSTermination = metav1.ObjectMeta{
			Name:      namespace + "-istio-tls-termination",
			Namespace: istioNamespace,
		}
		expectedWildcardEnvoyFilterObjectMetaIstioTLSTermination = metav1.ObjectMeta{
			Name:      namespace + "-istio-tls-termination",
			Namespace: istioWildcardNamespace,
		}
		expectedSecretObjectMetaIstioMTLS = metav1.ObjectMeta{
			Name:      namespace + "-kube-apiserver-istio-mtls",
			Namespace: istioNamespace,
		}
		expectedWildcardSecretObjectMetaIstioMTLS = metav1.ObjectMeta{
			Name:      namespace + "-kube-apiserver-istio-mtls",
			Namespace: istioWildcardNamespace,
		}
		expectedSecretObjectMetaIstioTLS = metav1.ObjectMeta{
			Name:      namespace + "-kube-apiserver-tls",
			Namespace: istioNamespace,
		}
		expectedGateway = &istionetworkingv1beta1.Gateway{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kube-apiserver",
				Namespace: namespace,
				Labels: map[string]string{
					"app":  "kubernetes",
					"role": "apiserver",
				},
				ResourceVersion: "1",
			},
			Spec: istioapinetworkingv1beta1.Gateway{
				Selector: istioLabels,
				Servers: []*istioapinetworkingv1beta1.Server{{
					Hosts: hosts,
					Port: &istioapinetworkingv1beta1.Port{
						Number:   443,
						Name:     "tls",
						Protocol: "TLS",
					},
					Tls: &istioapinetworkingv1beta1.ServerTLSSettings{
						Mode: istioapinetworkingv1beta1.ServerTLSSettings_PASSTHROUGH,
					},
				}},
			},
		}
		expectedWildcardGateway = &istionetworkingv1beta1.Gateway{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kube-apiserver-wildcard",
				Namespace: namespace,
				Labels: map[string]string{
					"app":  "kubernetes",
					"role": "apiserver",
				},
				ResourceVersion: "1",
			},
			Spec: istioapinetworkingv1beta1.Gateway{
				Selector: istioWildcardLabels,
				Servers: []*istioapinetworkingv1beta1.Server{{
					Hosts: wildcardHosts,
					Port: &istioapinetworkingv1beta1.Port{
						Number:   443,
						Name:     "tls",
						Protocol: "TLS",
					},
					Tls: &istioapinetworkingv1beta1.ServerTLSSettings{
						Mode: istioapinetworkingv1beta1.ServerTLSSettings_PASSTHROUGH,
					},
				}},
			},
		}
		expectedVirtualService = &istionetworkingv1beta1.VirtualService{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kube-apiserver",
				Namespace: namespace,
				Labels: map[string]string{
					"app":  "kubernetes",
					"role": "apiserver",
				},
				ResourceVersion: "1",
			},
			Spec: istioapinetworkingv1beta1.VirtualService{
				ExportTo: []string{"*"},
				Hosts:    hosts,
				Gateways: []string{expectedGateway.Name},
				Tls: []*istioapinetworkingv1beta1.TLSRoute{{
					Match: []*istioapinetworkingv1beta1.TLSMatchAttributes{{
						Port:     443,
						SniHosts: hosts,
					}},
					Route: []*istioapinetworkingv1beta1.RouteDestination{{
						Destination: &istioapinetworkingv1beta1.Destination{
							Host: hostName,
							Port: &istioapinetworkingv1beta1.PortSelector{Number: 443},
						},
					}},
				}},
			},
		}
		expectedWildcardVirtualService = &istionetworkingv1beta1.VirtualService{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kube-apiserver-wildcard",
				Namespace: namespace,
				Labels: map[string]string{
					"app":  "kubernetes",
					"role": "apiserver",
				},
				ResourceVersion: "1",
			},
			Spec: istioapinetworkingv1beta1.VirtualService{
				ExportTo: []string{"*"},
				Hosts:    wildcardHosts,
				Gateways: []string{expectedWildcardGateway.Name},
				Tls: []*istioapinetworkingv1beta1.TLSRoute{{
					Match: []*istioapinetworkingv1beta1.TLSMatchAttributes{{
						Port:     443,
						SniHosts: wildcardHosts,
					}},
					Route: []*istioapinetworkingv1beta1.RouteDestination{{
						Destination: &istioapinetworkingv1beta1.Destination{
							Host: hostName,
							Port: &istioapinetworkingv1beta1.PortSelector{Number: 443},
						},
					}},
				}},
			},
		}
		expectedManagedResourceSNI = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "kube-apiserver-sni",
				Namespace:       namespace,
				ResourceVersion: "1",
				Labels:          map[string]string{"gardener.cloud/role": "seed-system-component"},
			},
			Spec: resourcesv1alpha1.ManagedResourceSpec{
				Class:       ptr.To("seed"),
				KeepObjects: ptr.To(false),
			},
		}
		expectedManagedResourceTLSSecrets = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "istio-tls-secrets",
				Namespace:       namespace,
				ResourceVersion: "1",
				Labels:          map[string]string{"gardener.cloud/role": "seed-system-component"},
			},
			Spec: resourcesv1alpha1.ManagedResourceSpec{
				Class:       ptr.To("seed"),
				KeepObjects: ptr.To(false),
			},
		}
	})

	JustBeforeEach(func() {
		By("Create secrets managed outside of this package for whose secretsmanager.Get() will be called")
		Expect(c.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ca", Namespace: namespace}})).To(Succeed())
		Expect(c.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "ca-client", Namespace: namespace}})).To(Succeed())
		Expect(c.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "kube-apiserver", Namespace: namespace}})).To(Succeed())
		Expect(c.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "kube-apiserver-current", Namespace: namespace}})).To(Succeed())

		defaultDepWaiter = NewSNI(c, v1beta1constants.DeploymentNameKubeAPIServer, namespace, sm, func() *SNIValues {
			val := &SNIValues{
				Hosts:          hosts,
				APIServerProxy: apiServerProxyValues,
				IstioIngressGateway: IstioIngressGateway{
					Namespace: istioNamespace,
					Labels:    istioLabels,
				},
				IstioTLSTermination:   istioTLSTermination,
				WildcardConfiguration: wildcardConfiguration,
			}
			return val
		})
	})

	Describe("#Deploy", func() {
		testFunc := func() {
			Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())

			actualDestinationRule := &istionetworkingv1beta1.DestinationRule{}
			Expect(c.Get(ctx, client.ObjectKey{Namespace: expectedDestinationRule.Namespace, Name: expectedDestinationRule.Name}, actualDestinationRule)).To(Succeed())
			Expect(actualDestinationRule).To(BeComparableTo(expectedDestinationRule, comptest.CmpOptsForDestinationRule()))

			actualGateway := &istionetworkingv1beta1.Gateway{}
			Expect(c.Get(ctx, client.ObjectKey{Namespace: expectedGateway.Namespace, Name: expectedGateway.Name}, actualGateway)).To(Succeed())
			Expect(actualGateway).To(BeComparableTo(expectedGateway, comptest.CmpOptsForGateway()))

			actualVirtualService := &istionetworkingv1beta1.VirtualService{}
			Expect(c.Get(ctx, client.ObjectKey{Namespace: expectedVirtualService.Namespace, Name: expectedVirtualService.Name}, actualVirtualService)).To(Succeed())
			Expect(actualVirtualService).To(BeComparableTo(expectedVirtualService, comptest.CmpOptsForVirtualService()))

			managedResourceIstioTLS := &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Name: "istio-tls-secrets", Namespace: namespace}}
			if istioTLSTermination {
				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceIstioTLS), managedResourceIstioTLS)).To(Succeed())
			} else {
				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceIstioTLS), managedResourceIstioTLS)).To(BeNotFoundError())
			}

			managedResource := &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: expectedManagedResourceSNI.Namespace,
					Name:      expectedManagedResourceSNI.Name,
				},
			}

			if (apiServerProxyValues != nil && apiServerProxyValues.UseProxyProtocol) || istioTLSTermination {
				mrData := validateManagedResourceAndGetData(ctx, c, expectedManagedResourceSNI)

				var envoyFilterObjectsMetas []metav1.ObjectMeta
				for _, mrDataSet := range strings.Split(string(mrData), "---\n") {
					if mrDataSet == "" {
						continue
					}

					managedResourceEnvoyFilter, _, err := kubernetes.ShootCodec.UniversalDecoder().Decode([]byte(mrDataSet), nil, &istionetworkingv1alpha3.EnvoyFilter{})
					Expect(err).ToNot(HaveOccurred())
					Expect(managedResourceEnvoyFilter.GetObjectKind()).To(Equal(&metav1.TypeMeta{Kind: "EnvoyFilter", APIVersion: "networking.istio.io/v1alpha3"}))
					actualEnvoyFilter := managedResourceEnvoyFilter.(*istionetworkingv1alpha3.EnvoyFilter)
					// cannot validate the Spec as there is no meaningful way to unmarshal the data into the Golang structure
					envoyFilterObjectsMetas = append(envoyFilterObjectsMetas, actualEnvoyFilter.ObjectMeta)
				}

				if apiServerProxyValues != nil && apiServerProxyValues.UseProxyProtocol {
					Expect(envoyFilterObjectsMetas).To(ContainElement(expectedEnvoyFilterObjectMetaAPIServerProxy))
				}

				if istioTLSTermination {
					Expect(envoyFilterObjectsMetas).To(ContainElement(expectedEnvoyFilterObjectMetaIstioTLSTermination))

					if wildcardConfiguration != nil && wildcardConfiguration.IstioIngressGateway != nil {
						Expect(envoyFilterObjectsMetas).To(ContainElement(expectedWildcardEnvoyFilterObjectMetaIstioTLSTermination))
					}
				}
			} else {
				Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError(), "should delete EnvoyFilter for apiserver-proxy")
			}

			if istioTLSTermination {
				mrData := validateManagedResourceAndGetData(ctx, c, expectedManagedResourceTLSSecrets)

				var secretObjectsMetas []metav1.ObjectMeta
				for _, mrDataSet := range strings.Split(string(mrData), "---\n") {
					if mrDataSet == "" {
						continue
					}

					managedResourceSecret, _, err := kubernetes.ShootCodec.UniversalDecoder().Decode([]byte(mrDataSet), nil, &corev1.Secret{})
					Expect(err).ToNot(HaveOccurred())
					Expect(managedResourceSecret.GetObjectKind()).To(Equal(&metav1.TypeMeta{Kind: "Secret", APIVersion: "v1"}))
					actualSecret := managedResourceSecret.(*corev1.Secret)
					// cannot validate the Spec as there is no meaningful way to unmarshal the data into the Golang structure
					secretObjectsMetas = append(secretObjectsMetas, actualSecret.ObjectMeta)
				}

				Expect(secretObjectsMetas).To(ContainElement(expectedSecretObjectMetaIstioMTLS))
				Expect(secretObjectsMetas).To(ContainElement(expectedSecretObjectMetaIstioTLS))

				if wildcardConfiguration != nil && wildcardConfiguration.IstioIngressGateway != nil {
					Expect(secretObjectsMetas).To(ContainElement(expectedWildcardSecretObjectMetaIstioMTLS))
				}
			}

			if wildcardConfiguration != nil && wildcardConfiguration.IstioIngressGateway != nil {
				actualWildcardGateway := &istionetworkingv1beta1.Gateway{}
				Expect(c.Get(ctx, client.ObjectKey{Namespace: expectedWildcardGateway.Namespace, Name: expectedWildcardGateway.Name}, actualWildcardGateway)).To(Succeed())
				Expect(actualWildcardGateway).To(BeComparableTo(expectedWildcardGateway, comptest.CmpOptsForGateway()))

				actualWildcardVirtualService := &istionetworkingv1beta1.VirtualService{}
				Expect(c.Get(ctx, client.ObjectKey{Namespace: expectedWildcardVirtualService.Namespace, Name: expectedWildcardVirtualService.Name}, actualWildcardVirtualService)).To(Succeed())
				Expect(actualWildcardVirtualService).To(BeComparableTo(expectedWildcardVirtualService, comptest.CmpOptsForVirtualService()))
			}
		}

		Context("when APIServer Proxy is configured", func() {
			It("should succeed deploying", func() {
				testFunc()
			})
		})

		Context("when APIServer Proxy is not configured", func() {
			BeforeEach(func() {
				apiServerProxyValues = nil

				// create EnvoyFilter to ensure that Deploy deletes it
				envoyFilter := &istionetworkingv1alpha3.EnvoyFilter{
					ObjectMeta: *expectedEnvoyFilterObjectMetaAPIServerProxy.DeepCopy(),
				}
				envoyFilter.ResourceVersion = ""
				Expect(c.Create(ctx, envoyFilter)).To(Succeed())
			})

			It("should succeed deploying", func() {
				testFunc()
			})
		})

		Context("when wildcard certificate is configured", func() {
			BeforeEach(func() {
				wildcardConfiguration = &WildcardConfiguration{
					TLSSecret: wildcardTLSSecret,
					Hosts:     wildcardHosts,
				}

				expectedGateway.Spec.Servers[0].Hosts = append(expectedGateway.Spec.Servers[0].Hosts, wildcardHosts...)
				expectedVirtualService.Spec.Hosts = append(expectedVirtualService.Spec.Hosts, wildcardHosts...)
				expectedVirtualService.Spec.Tls[0].Match[0].SniHosts = append(expectedVirtualService.Spec.Tls[0].Match[0].SniHosts, wildcardHosts...)
			})

			It("should succeed deploying", func() {
				testFunc()
			})
		})

		Context("when wildcard certificate with dedicated gateway is configured", func() {
			BeforeEach(func() {
				wildcardConfiguration = &WildcardConfiguration{
					IstioIngressGateway: wildcardIstioIngressGateway,
					TLSSecret:           wildcardTLSSecret,
					Hosts:               wildcardHosts,
				}
			})

			It("should succeed deploying", func() {
				testFunc()
			})
		})

		Context("when IstioTLSTermination feature gate is true", func() {
			BeforeEach(func() {
				istioTLSTermination = true

				expectedDestinationRule.Spec.TrafficPolicy.LoadBalancer = &istioapinetworkingv1beta1.LoadBalancerSettings{
					LbPolicy: &istioapinetworkingv1beta1.LoadBalancerSettings_Simple{
						Simple: istioapinetworkingv1beta1.LoadBalancerSettings_LEAST_REQUEST,
					},
				}
				expectedDestinationRule.Spec.TrafficPolicy.OutlierDetection = nil
				expectedDestinationRule.Spec.TrafficPolicy.Tls = &istioapinetworkingv1beta1.ClientTLSSettings{
					Mode:           istioapinetworkingv1beta1.ClientTLSSettings_SIMPLE,
					CredentialName: namespace + "-kube-apiserver-istio-mtls",
					Sni:            "kubernetes.default.svc.cluster.local",
				}

				expectedGateway.Spec.Servers[0].Port.Protocol = "HTTPS"
				expectedGateway.Spec.Servers[0].Tls = &istioapinetworkingv1beta1.ServerTLSSettings{
					Mode:           istioapinetworkingv1beta1.ServerTLSSettings_OPTIONAL_MUTUAL,
					CredentialName: namespace + "-kube-apiserver-tls",
				}

				expectedVirtualService.Spec.Tls = nil
				expectedVirtualService.Spec.Http = []*istioapinetworkingv1beta1.HTTPRoute{
					{
						Name: "connection-upgrade",
						Match: []*istioapinetworkingv1beta1.HTTPMatchRequest{
							{
								Headers: map[string]*istioapinetworkingv1beta1.StringMatch{
									"Connection": {MatchType: &istioapinetworkingv1beta1.StringMatch_Exact{Exact: "Upgrade"}},
									"Upgrade":    {},
								},
							},
						},
						Route: []*istioapinetworkingv1beta1.HTTPRouteDestination{
							{
								Destination: &istioapinetworkingv1beta1.Destination{
									Host: connectionUpgradeHostName,
									Port: &istioapinetworkingv1beta1.PortSelector{Number: 443},
								},
							},
						},
					},
					{
						Route: []*istioapinetworkingv1beta1.HTTPRouteDestination{
							{
								Destination: &istioapinetworkingv1beta1.Destination{
									Host: hostName,
									Port: &istioapinetworkingv1beta1.PortSelector{Number: 443},
								},
							},
						},
					},
				}
			})

			It("should succeed deploying", func() {
				testFunc()
			})
		})

		Context("when IstioTLSTermination feature gate is true and wildcard certificate is configured", func() {
			BeforeEach(func() {
				istioTLSTermination = true
				wildcardConfiguration = &WildcardConfiguration{
					TLSSecret: wildcardTLSSecret,
					Hosts:     wildcardHosts,
				}

				expectedDestinationRule.Spec.TrafficPolicy.LoadBalancer = &istioapinetworkingv1beta1.LoadBalancerSettings{
					LbPolicy: &istioapinetworkingv1beta1.LoadBalancerSettings_Simple{
						Simple: istioapinetworkingv1beta1.LoadBalancerSettings_LEAST_REQUEST,
					},
				}
				expectedDestinationRule.Spec.TrafficPolicy.OutlierDetection = nil
				expectedDestinationRule.Spec.TrafficPolicy.Tls = &istioapinetworkingv1beta1.ClientTLSSettings{
					Mode:           istioapinetworkingv1beta1.ClientTLSSettings_SIMPLE,
					CredentialName: namespace + "-kube-apiserver-istio-mtls",
					Sni:            "kubernetes.default.svc.cluster.local",
				}

				expectedGateway.Spec.Servers[0].Port.Protocol = "HTTPS"
				expectedGateway.Spec.Servers[0].Tls = &istioapinetworkingv1beta1.ServerTLSSettings{
					Mode:           istioapinetworkingv1beta1.ServerTLSSettings_OPTIONAL_MUTUAL,
					CredentialName: namespace + "-kube-apiserver-tls",
				}

				expectedGateway.Spec.Servers = append(expectedGateway.Spec.Servers, &istioapinetworkingv1beta1.Server{
					Hosts: wildcardHosts,
					Port: &istioapinetworkingv1beta1.Port{
						Number:   443,
						Name:     "wildcard-tls",
						Protocol: "HTTPS",
					},
					Tls: &istioapinetworkingv1beta1.ServerTLSSettings{
						Mode:           istioapinetworkingv1beta1.ServerTLSSettings_OPTIONAL_MUTUAL,
						CredentialName: namespace + "-kube-apiserver-wildcard-tls",
					},
				})

				expectedVirtualService.Spec.Tls = nil
				expectedVirtualService.Spec.Http = []*istioapinetworkingv1beta1.HTTPRoute{
					{
						Name: "connection-upgrade",
						Match: []*istioapinetworkingv1beta1.HTTPMatchRequest{
							{
								Headers: map[string]*istioapinetworkingv1beta1.StringMatch{
									"Connection": {MatchType: &istioapinetworkingv1beta1.StringMatch_Exact{Exact: "Upgrade"}},
									"Upgrade":    {},
								},
							},
						},
						Route: []*istioapinetworkingv1beta1.HTTPRouteDestination{
							{
								Destination: &istioapinetworkingv1beta1.Destination{
									Host: connectionUpgradeHostName,
									Port: &istioapinetworkingv1beta1.PortSelector{Number: 443},
								},
							},
						},
					},
					{
						Route: []*istioapinetworkingv1beta1.HTTPRouteDestination{
							{
								Destination: &istioapinetworkingv1beta1.Destination{
									Host: hostName,
									Port: &istioapinetworkingv1beta1.PortSelector{Number: 443},
								},
							},
						},
					},
				}

				expectedVirtualService.Spec.Hosts = append(expectedVirtualService.Spec.Hosts, wildcardHosts...)
			})

			It("should succeed deploying", func() {
				testFunc()
			})
		})

		Context("when IstioTLSTermination feature gate is true and wildcard certificate with a dedicated gateway is configured", func() {
			BeforeEach(func() {
				istioTLSTermination = true
				wildcardConfiguration = &WildcardConfiguration{
					IstioIngressGateway: wildcardIstioIngressGateway,
					TLSSecret:           wildcardTLSSecret,
					Hosts:               wildcardHosts,
				}

				expectedDestinationRule.Spec.TrafficPolicy.LoadBalancer = &istioapinetworkingv1beta1.LoadBalancerSettings{
					LbPolicy: &istioapinetworkingv1beta1.LoadBalancerSettings_Simple{
						Simple: istioapinetworkingv1beta1.LoadBalancerSettings_LEAST_REQUEST,
					},
				}
				expectedDestinationRule.Spec.TrafficPolicy.OutlierDetection = nil
				expectedDestinationRule.Spec.TrafficPolicy.Tls = &istioapinetworkingv1beta1.ClientTLSSettings{
					Mode:           istioapinetworkingv1beta1.ClientTLSSettings_SIMPLE,
					CredentialName: namespace + "-kube-apiserver-istio-mtls",
					Sni:            "kubernetes.default.svc.cluster.local",
				}

				expectedGateway.Spec.Servers[0].Port.Protocol = "HTTPS"
				expectedGateway.Spec.Servers[0].Tls = &istioapinetworkingv1beta1.ServerTLSSettings{
					Mode:           istioapinetworkingv1beta1.ServerTLSSettings_OPTIONAL_MUTUAL,
					CredentialName: namespace + "-kube-apiserver-tls",
				}

				expectedWildcardGateway.Spec.Servers[0].Port.Protocol = "HTTPS"
				expectedWildcardGateway.Spec.Servers[0].Port.Name = "wildcard-tls"
				expectedWildcardGateway.Spec.Servers[0].Tls = &istioapinetworkingv1beta1.ServerTLSSettings{
					Mode:           istioapinetworkingv1beta1.ServerTLSSettings_OPTIONAL_MUTUAL,
					CredentialName: namespace + "-kube-apiserver-wildcard-tls",
				}

				expectedVirtualService.Spec.Tls = nil
				expectedVirtualService.Spec.Http = []*istioapinetworkingv1beta1.HTTPRoute{
					{
						Name: "connection-upgrade",
						Match: []*istioapinetworkingv1beta1.HTTPMatchRequest{
							{
								Headers: map[string]*istioapinetworkingv1beta1.StringMatch{
									"Connection": {MatchType: &istioapinetworkingv1beta1.StringMatch_Exact{Exact: "Upgrade"}},
									"Upgrade":    {},
								},
							},
						},
						Route: []*istioapinetworkingv1beta1.HTTPRouteDestination{
							{
								Destination: &istioapinetworkingv1beta1.Destination{
									Host: connectionUpgradeHostName,
									Port: &istioapinetworkingv1beta1.PortSelector{Number: 443},
								},
							},
						},
					},
					{
						Route: []*istioapinetworkingv1beta1.HTTPRouteDestination{
							{
								Destination: &istioapinetworkingv1beta1.Destination{
									Host: hostName,
									Port: &istioapinetworkingv1beta1.PortSelector{Number: 443},
								},
							},
						},
					},
				}

				expectedWildcardVirtualService.Spec.Tls = nil
				expectedWildcardVirtualService.Spec.Http = []*istioapinetworkingv1beta1.HTTPRoute{
					{
						Name: "connection-upgrade",
						Match: []*istioapinetworkingv1beta1.HTTPMatchRequest{
							{
								Headers: map[string]*istioapinetworkingv1beta1.StringMatch{
									"Connection": {MatchType: &istioapinetworkingv1beta1.StringMatch_Exact{Exact: "Upgrade"}},
									"Upgrade":    {},
								},
							},
						},
						Route: []*istioapinetworkingv1beta1.HTTPRouteDestination{
							{
								Destination: &istioapinetworkingv1beta1.Destination{
									Host: connectionUpgradeHostName,
									Port: &istioapinetworkingv1beta1.PortSelector{Number: 443},
								},
							},
						},
					},
					{
						Route: []*istioapinetworkingv1beta1.HTTPRouteDestination{
							{
								Destination: &istioapinetworkingv1beta1.Destination{
									Host: hostName,
									Port: &istioapinetworkingv1beta1.PortSelector{Number: 443},
								},
							},
						},
					},
				}
			})

			It("should succeed deploying", func() {
				testFunc()
			})
		})
	})

	It("should succeed destroying", func() {
		istioTLSTermination = true

		Expect(defaultDepWaiter.Deploy(ctx)).To(Succeed())

		Expect(c.Get(ctx, client.ObjectKey{Namespace: expectedDestinationRule.Namespace, Name: expectedDestinationRule.Name}, &istionetworkingv1beta1.DestinationRule{})).To(Succeed())
		Expect(c.Get(ctx, client.ObjectKey{Namespace: expectedGateway.Namespace, Name: expectedGateway.Name}, &istionetworkingv1beta1.Gateway{})).To(Succeed())
		Expect(c.Get(ctx, client.ObjectKey{Namespace: expectedVirtualService.Namespace, Name: expectedVirtualService.Name}, &istionetworkingv1beta1.VirtualService{})).To(Succeed())
		managedResourceSNI := &resourcesv1alpha1.ManagedResource{}
		Expect(c.Get(ctx, client.ObjectKey{Namespace: expectedManagedResourceSNI.Namespace, Name: expectedManagedResourceSNI.Name}, managedResourceSNI)).To(Succeed())
		managedResourceSNISecretName := managedResourceSNI.Spec.SecretRefs[0].Name
		Expect(c.Get(ctx, client.ObjectKey{Namespace: expectedManagedResourceSNI.Namespace, Name: managedResourceSNISecretName}, &corev1.Secret{})).To(Succeed())
		managedResourceTLS := &resourcesv1alpha1.ManagedResource{}
		Expect(c.Get(ctx, client.ObjectKey{Namespace: expectedManagedResourceTLSSecrets.Namespace, Name: expectedManagedResourceTLSSecrets.Name}, managedResourceTLS)).To(Succeed())
		managedResourceTLSSecretName := managedResourceTLS.Spec.SecretRefs[0].Name
		Expect(c.Get(ctx, client.ObjectKey{Namespace: expectedManagedResourceTLSSecrets.Namespace, Name: managedResourceTLSSecretName}, &corev1.Secret{})).To(Succeed())

		Expect(defaultDepWaiter.Destroy(ctx)).To(Succeed())

		Expect(c.Get(ctx, client.ObjectKey{Namespace: expectedDestinationRule.Namespace, Name: expectedDestinationRule.Name}, &istionetworkingv1beta1.DestinationRule{})).To(BeNotFoundError())
		Expect(c.Get(ctx, client.ObjectKey{Namespace: expectedGateway.Namespace, Name: expectedGateway.Name}, &istionetworkingv1beta1.Gateway{})).To(BeNotFoundError())
		Expect(c.Get(ctx, client.ObjectKey{Namespace: expectedVirtualService.Namespace, Name: expectedVirtualService.Name}, &istionetworkingv1beta1.VirtualService{})).To(BeNotFoundError())
		Expect(c.Get(ctx, client.ObjectKey{Namespace: expectedManagedResourceSNI.Namespace, Name: expectedManagedResourceSNI.Name}, managedResourceSNI)).To(BeNotFoundError())
		Expect(c.Get(ctx, client.ObjectKey{Namespace: expectedManagedResourceSNI.Namespace, Name: managedResourceSNISecretName}, &corev1.Secret{})).To(BeNotFoundError())
		Expect(c.Get(ctx, client.ObjectKey{Namespace: expectedManagedResourceTLSSecrets.Namespace, Name: expectedManagedResourceTLSSecrets.Name}, managedResourceTLS)).To(BeNotFoundError())
		Expect(c.Get(ctx, client.ObjectKey{Namespace: expectedManagedResourceTLSSecrets.Namespace, Name: managedResourceTLSSecretName}, &corev1.Secret{})).To(BeNotFoundError())
	})

	Describe("#Wait", func() {
		It("should succeed because it's not implemented", func() {
			Expect(defaultDepWaiter.Wait(ctx)).To(Succeed())
		})
	})

	Describe("#WaitCleanup", func() {
		It("should succeed because it's not implemented", func() {
			Expect(defaultDepWaiter.WaitCleanup(ctx)).To(Succeed())
		})
	})
})

func validateManagedResourceAndGetData(ctx context.Context, c client.Client, expectedManagedResource *resourcesv1alpha1.ManagedResource) []byte {
	managedResource := &resourcesv1alpha1.ManagedResource{}
	ExpectWithOffset(1, c.Get(ctx, client.ObjectKeyFromObject(expectedManagedResource), managedResource)).To(Succeed())
	expectedManagedResource.Spec.SecretRefs = []corev1.LocalObjectReference{{Name: managedResource.Spec.SecretRefs[0].Name}}
	utilruntime.Must(references.InjectAnnotations(expectedManagedResource))
	ExpectWithOffset(1, managedResource).To(DeepEqual(expectedManagedResource))

	managedResourceSecret := &corev1.Secret{}
	ExpectWithOffset(1, c.Get(ctx, client.ObjectKey{Namespace: expectedManagedResource.Namespace, Name: expectedManagedResource.Spec.SecretRefs[0].Name}, managedResourceSecret)).To(Succeed())
	ExpectWithOffset(1, managedResourceSecret.Type).To(Equal(corev1.SecretTypeOpaque))
	ExpectWithOffset(1, managedResourceSecret.Immutable).To(Equal(ptr.To(true)))
	ExpectWithOffset(1, managedResourceSecret.Labels["resources.gardener.cloud/garbage-collectable-reference"]).To(Equal("true"))
	ExpectWithOffset(1, managedResourceSecret.Data).To(HaveLen(1))
	ExpectWithOffset(1, managedResourceSecret.Data).To(HaveKey("data.yaml.br"))

	mrData, err := test.BrotliDecompression(managedResourceSecret.Data["data.yaml.br"])
	ExpectWithOffset(1, err).NotTo(HaveOccurred())

	return mrData
}

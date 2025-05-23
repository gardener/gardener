// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package apiserverexposure_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
	istioapinetworkingv1beta1 "istio.io/api/networking/v1beta1"
	istionetworkingv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/gardener/gardener/pkg/component"
	. "github.com/gardener/gardener/pkg/component/kubernetes/apiserverexposure"
	comptest "github.com/gardener/gardener/pkg/component/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("#Ingress", func() {
	var (
		ctx context.Context
		c   client.Client

		objKey           client.ObjectKey
		httpsDRKey       client.ObjectKey
		istioLabels      map[string]string
		istioNamespace   string
		namespace        string
		serviceNamespace string
		host             string
		sniHosts         []string
		tlsSecret        string

		expectedPassthroughGateway          *istionetworkingv1beta1.Gateway
		expectedHTTPSBackendGateway         *istionetworkingv1beta1.Gateway
		expectedPassthroughVirtualService   *istionetworkingv1beta1.VirtualService
		expectedHTTPSBackendVirtualService  *istionetworkingv1beta1.VirtualService
		expectedPassthroughDestinationRule  *istionetworkingv1beta1.DestinationRule
		expectedHTTPSBackendDestinationRule *istionetworkingv1beta1.DestinationRule
	)

	BeforeEach(func() {
		ctx = context.TODO()
		s := runtime.NewScheme()
		Expect(corev1.AddToScheme(s)).To(Succeed())
		Expect(networkingv1.AddToScheme(s)).To(Succeed())
		Expect(istionetworkingv1beta1.AddToScheme(s)).To(Succeed())
		c = fake.NewClientBuilder().WithScheme(s).Build()

		istioLabels = map[string]string{"istio": "ingress"}
		istioNamespace = "istio-foo"
		namespace = "bar"
		serviceNamespace = "services"
		objKey = client.ObjectKey{Name: "kube-apiserver-ingress", Namespace: namespace}
		httpsDRKey = client.ObjectKey{Name: objKey.Name, Namespace: serviceNamespace}
		host = "foo.bar.example.com"
		sniHosts = []string{host}
		tlsSecret = "wildcard-tls-secret"

		expectedGateway := &istionetworkingv1beta1.Gateway{
			TypeMeta: metav1.TypeMeta{
				APIVersion: istionetworkingv1beta1.SchemeGroupVersion.String(),
				Kind:       "Gateway",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kube-apiserver-ingress",
				Namespace: namespace,
				Labels: map[string]string{
					"app":  "kubernetes",
					"role": "apiserver",
				},
			},
			Spec: istioapinetworkingv1beta1.Gateway{
				Selector: istioLabels,
				Servers: []*istioapinetworkingv1beta1.Server{{
					Hosts: sniHosts,
					Port: &istioapinetworkingv1beta1.Port{
						Number:   443,
						Name:     "tls",
						Protocol: "foo",
					},
					Tls: &istioapinetworkingv1beta1.ServerTLSSettings{},
				}},
			},
		}

		expectedPassthroughGateway = expectedGateway.DeepCopy()
		expectedPassthroughGateway.Spec.Servers[0].Port.Protocol = "TLS"
		expectedPassthroughGateway.Spec.Servers[0].Tls.Mode = istioapinetworkingv1beta1.ServerTLSSettings_PASSTHROUGH

		expectedHTTPSBackendGateway = expectedGateway.DeepCopy()
		expectedHTTPSBackendGateway.Spec.Servers[0].Port.Protocol = "HTTPS"
		expectedHTTPSBackendGateway.Spec.Servers[0].Tls.Mode = istioapinetworkingv1beta1.ServerTLSSettings_SIMPLE
		expectedHTTPSBackendGateway.Spec.Servers[0].Tls.CredentialName = tlsSecret

		expectedVirtualService := &istionetworkingv1beta1.VirtualService{
			TypeMeta: metav1.TypeMeta{
				APIVersion: istionetworkingv1beta1.SchemeGroupVersion.String(),
				Kind:       "VirtualService",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kube-apiserver-ingress",
				Namespace: namespace,
				Labels: map[string]string{
					"app":  "kubernetes",
					"role": "apiserver",
				},
			},
			Spec: istioapinetworkingv1beta1.VirtualService{
				ExportTo: []string{"*"},
				Hosts:    sniHosts,
				Gateways: []string{"kube-apiserver-ingress"},
				Tls: []*istioapinetworkingv1beta1.TLSRoute{{
					Match: []*istioapinetworkingv1beta1.TLSMatchAttributes{{
						Port:     443,
						SniHosts: sniHosts,
					}},
					Route: []*istioapinetworkingv1beta1.RouteDestination{{
						Destination: &istioapinetworkingv1beta1.Destination{
							Host: "foo.bar.svc.cluster.local",
							Port: &istioapinetworkingv1beta1.PortSelector{Number: 443},
						},
					}},
				}},
			},
		}

		expectedPassthroughVirtualService = expectedVirtualService.DeepCopy()

		expectedHTTPSBackendVirtualService = expectedVirtualService.DeepCopy()
		expectedHTTPSBackendVirtualService.Spec.Tls[0].Route[0].Destination.Host = "foo." + serviceNamespace + ".svc.cluster.local"
		expectedHTTPSBackendVirtualService.Spec.Http = []*istioapinetworkingv1beta1.HTTPRoute{{
			Match: []*istioapinetworkingv1beta1.HTTPMatchRequest{{
				Uri: &istioapinetworkingv1beta1.StringMatch{
					MatchType: &istioapinetworkingv1beta1.StringMatch_Prefix{Prefix: "/"},
				},
			}},
			Route: []*istioapinetworkingv1beta1.HTTPRouteDestination{{
				Destination: &istioapinetworkingv1beta1.Destination{
					Host: "foo." + serviceNamespace + ".svc.cluster.local",
					Port: &istioapinetworkingv1beta1.PortSelector{Number: 443},
				},
			}},
		}}

		expectedDestinationRule := &istionetworkingv1beta1.DestinationRule{
			TypeMeta: metav1.TypeMeta{
				APIVersion: istionetworkingv1beta1.SchemeGroupVersion.String(),
				Kind:       "DestinationRule",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kube-apiserver-ingress",
				Namespace: namespace,
				Labels: map[string]string{
					"app":  "kubernetes",
					"role": "apiserver",
				},
			},
			Spec: istioapinetworkingv1beta1.DestinationRule{
				ExportTo: []string{"*"},
				Host:     "foo.bar.svc.cluster.local",
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
							FailoverPriority: []string{corev1.LabelTopologyZone},
						},
					},
					OutlierDetection: &istioapinetworkingv1beta1.OutlierDetection{
						MinHealthPercent: 0,
					},
					Tls: &istioapinetworkingv1beta1.ClientTLSSettings{},
				},
			},
		}

		expectedPassthroughDestinationRule = expectedDestinationRule.DeepCopy()
		expectedPassthroughDestinationRule.Spec.TrafficPolicy.Tls.Mode = istioapinetworkingv1beta1.ClientTLSSettings_DISABLE

		expectedHTTPSBackendDestinationRule = expectedDestinationRule.DeepCopy()
		expectedHTTPSBackendDestinationRule.Namespace = serviceNamespace
		expectedHTTPSBackendDestinationRule.Spec.Host = "foo." + serviceNamespace + ".svc.cluster.local"
		expectedHTTPSBackendDestinationRule.Spec.TrafficPolicy.Tls.Mode = istioapinetworkingv1beta1.ClientTLSSettings_SIMPLE
	})

	getDeployer := func(serviceNamespace string, tlsSecretName *string) component.Deployer {
		return NewIngress(c, namespace, IngressValues{
			Host: host,
			IstioIngressGatewayLabelsFunc: func() map[string]string {
				return istioLabels
			},
			IstioIngressGatewayNamespaceFunc: func() string {
				return istioNamespace
			},
			ServiceName:      "foo",
			ServiceNamespace: serviceNamespace,
			TLSSecretName:    tlsSecretName,
		})
	}

	Context("Deploy", func() {
		It("should create the expected resources for tls passthrough", func() {
			Expect(getDeployer("", nil).Deploy(ctx)).To(Succeed())

			actualGateway := &istionetworkingv1beta1.Gateway{}
			Expect(c.Get(ctx, objKey, actualGateway)).To(Succeed())
			Expect(actualGateway.Labels).To(DeepEqual(expectedPassthroughGateway.Labels))
			Expect(&actualGateway.Spec).To(BeComparableTo(&expectedPassthroughGateway.Spec, comptest.CmpOptsForGateway()))

			actualVirtualService := &istionetworkingv1beta1.VirtualService{}
			Expect(c.Get(ctx, objKey, actualVirtualService)).To(Succeed())
			Expect(actualVirtualService.Labels).To(DeepEqual(expectedPassthroughVirtualService.Labels))
			Expect(&actualVirtualService.Spec).To(BeComparableTo(&expectedPassthroughVirtualService.Spec, comptest.CmpOptsForVirtualService()))

			actualDestinationRule := &istionetworkingv1beta1.DestinationRule{}
			Expect(c.Get(ctx, objKey, actualDestinationRule)).To(Succeed())
			Expect(actualDestinationRule.Labels).To(DeepEqual(expectedPassthroughDestinationRule.Labels))
			Expect(&actualDestinationRule.Spec).To(BeComparableTo(&expectedPassthroughDestinationRule.Spec, comptest.CmpOptsForDestinationRule()))
		})

		It("should create the expected resources for backend protocol HTTPS", func() {
			Expect(c.Create(ctx, &corev1.Secret{
				TypeMeta: metav1.TypeMeta{
					APIVersion: corev1.SchemeGroupVersion.String(),
					Kind:       "Secret",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      tlsSecret,
					Namespace: "garden",
					Labels:    map[string]string{"gardener.cloud/role": "controlplane-cert"},
				}})).To(Succeed())

			Expect(getDeployer(serviceNamespace, &tlsSecret).Deploy(ctx)).To(Succeed())

			actualGateway := &istionetworkingv1beta1.Gateway{}
			Expect(c.Get(ctx, objKey, actualGateway)).To(Succeed())
			Expect(actualGateway.Labels).To(DeepEqual(expectedHTTPSBackendGateway.Labels))
			Expect(&actualGateway.Spec).To(BeComparableTo(&expectedHTTPSBackendGateway.Spec, comptest.CmpOptsForGateway()))

			actualVirtualService := &istionetworkingv1beta1.VirtualService{}
			Expect(c.Get(ctx, objKey, actualVirtualService)).To(Succeed())
			Expect(actualVirtualService.Labels).To(DeepEqual(expectedHTTPSBackendVirtualService.Labels))
			Expect(&actualVirtualService.Spec).To(BeComparableTo(&expectedHTTPSBackendVirtualService.Spec, comptest.CmpOptsForVirtualService()))

			actualDestinationRule := &istionetworkingv1beta1.DestinationRule{}
			Expect(c.Get(ctx, httpsDRKey, actualDestinationRule)).To(Succeed())
			Expect(actualDestinationRule.Labels).To(DeepEqual(expectedHTTPSBackendDestinationRule.Labels))
			Expect(&actualDestinationRule.Spec).To(BeComparableTo(&expectedHTTPSBackendDestinationRule.Spec, comptest.CmpOptsForDestinationRule()))

			Expect(c.Get(ctx, client.ObjectKey{Name: tlsSecret, Namespace: istioNamespace}, &corev1.Secret{})).To(Succeed())
		})
	})

	Context("Destroy", func() {
		It("should delete the resources for tls passthrough", func() {
			Expect(c.Create(ctx, expectedPassthroughGateway)).To(Succeed())
			Expect(c.Get(ctx, objKey, &istionetworkingv1beta1.Gateway{})).To(Succeed())
			Expect(c.Create(ctx, expectedPassthroughVirtualService)).To(Succeed())
			Expect(c.Get(ctx, objKey, &istionetworkingv1beta1.VirtualService{})).To(Succeed())
			Expect(c.Create(ctx, expectedPassthroughDestinationRule)).To(Succeed())
			Expect(c.Get(ctx, objKey, &istionetworkingv1beta1.DestinationRule{})).To(Succeed())

			Expect(getDeployer("", nil).Destroy(ctx)).To(Succeed())

			Expect(c.Get(ctx, objKey, &istionetworkingv1beta1.Gateway{})).To(BeNotFoundError())
			Expect(c.Get(ctx, objKey, &istionetworkingv1beta1.VirtualService{})).To(BeNotFoundError())
			Expect(c.Get(ctx, objKey, &istionetworkingv1beta1.DestinationRule{})).To(BeNotFoundError())
		})

		It("should delete the resources for backend protocol HTTPS", func() {
			Expect(c.Create(ctx, expectedHTTPSBackendGateway)).To(Succeed())
			Expect(c.Get(ctx, objKey, &istionetworkingv1beta1.Gateway{})).To(Succeed())
			Expect(c.Create(ctx, expectedHTTPSBackendVirtualService)).To(Succeed())
			Expect(c.Get(ctx, objKey, &istionetworkingv1beta1.VirtualService{})).To(Succeed())
			Expect(c.Create(ctx, expectedHTTPSBackendDestinationRule)).To(Succeed())
			Expect(c.Get(ctx, httpsDRKey, &istionetworkingv1beta1.DestinationRule{})).To(Succeed())
			Expect(c.Create(ctx, &corev1.Secret{
				TypeMeta: metav1.TypeMeta{
					APIVersion: corev1.SchemeGroupVersion.String(),
					Kind:       "Secret",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      tlsSecret,
					Namespace: istioNamespace,
				},
			})).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKey{Name: tlsSecret, Namespace: istioNamespace}, &corev1.Secret{})).To(Succeed())

			Expect(getDeployer(serviceNamespace, &tlsSecret).Destroy(ctx)).To(Succeed())

			Expect(c.Get(ctx, objKey, &istionetworkingv1beta1.Gateway{})).To(BeNotFoundError())
			Expect(c.Get(ctx, objKey, &istionetworkingv1beta1.VirtualService{})).To(BeNotFoundError())
			Expect(c.Get(ctx, httpsDRKey, &istionetworkingv1beta1.DestinationRule{})).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKey{Name: tlsSecret, Namespace: istioNamespace}, &corev1.Secret{})).To(BeNotFoundError())
		})
	})
})

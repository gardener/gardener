// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package istio_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	istioapinetworkingv1beta1 "istio.io/api/networking/v1beta1"
	istionetworkingv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/gardener/gardener/pkg/utils/istio"
)

var _ = Describe("Protocol", func() {
	Describe("#DetermineProtocolMode", func() {
		var (
			destinationRule *istionetworkingv1beta1.DestinationRule
			port            corev1.ServicePort
		)

		BeforeEach(func() {
			destinationRule = &istionetworkingv1beta1.DestinationRule{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "ns"},
				Spec: istioapinetworkingv1beta1.DestinationRule{
					Host: "svc.ns.svc.cluster.local",
				},
			}
			port = corev1.ServicePort{Name: "https-main", Port: 443}
		})

		It("should return ProtocolModeNone when no HTTP/2 indicators are present", func() {
			Expect(DetermineProtocolMode(destinationRule, port)).To(Equal(HTTPProtocolPolicyDefault))
		})

		It("should return ProtocolModeExplicitHTTP2 when port name indicates HTTP/2", func() {
			port.Name = "grpc-main"
			Expect(DetermineProtocolMode(destinationRule, port)).To(Equal(HTTPProtocolPolicyExplicitHTTP2))
		})

		It("should return ProtocolModeExplicitHTTP2 when appProtocol indicates HTTP/2", func() {
			port.AppProtocol = new("kubernetes.io/h2c")
			Expect(DetermineProtocolMode(destinationRule, port)).To(Equal(HTTPProtocolPolicyExplicitHTTP2))
		})

		It("should return ProtocolModeUseDownstreamHTTP2 when useClientProtocol is true", func() {
			destinationRule.Spec.TrafficPolicy = &istioapinetworkingv1beta1.TrafficPolicy{
				ConnectionPool: &istioapinetworkingv1beta1.ConnectionPoolSettings{
					Http: &istioapinetworkingv1beta1.ConnectionPoolSettings_HTTPSettings{
						UseClientProtocol: true,
					},
				},
			}
			Expect(DetermineProtocolMode(destinationRule, port)).To(Equal(HTTPProtocolPolicyUseClientProtocol))
		})

		It("should return ProtocolModeExplicitHTTP2 when h2UpgradePolicy is UPGRADE", func() {
			destinationRule.Spec.TrafficPolicy = &istioapinetworkingv1beta1.TrafficPolicy{
				ConnectionPool: &istioapinetworkingv1beta1.ConnectionPoolSettings{
					Http: &istioapinetworkingv1beta1.ConnectionPoolSettings_HTTPSettings{
						H2UpgradePolicy: istioapinetworkingv1beta1.ConnectionPoolSettings_HTTPSettings_UPGRADE,
					},
				},
			}
			Expect(DetermineProtocolMode(destinationRule, port)).To(Equal(HTTPProtocolPolicyExplicitHTTP2))
		})

		It("should prefer useClientProtocol over h2UpgradePolicy", func() {
			destinationRule.Spec.TrafficPolicy = &istioapinetworkingv1beta1.TrafficPolicy{
				ConnectionPool: &istioapinetworkingv1beta1.ConnectionPoolSettings{
					Http: &istioapinetworkingv1beta1.ConnectionPoolSettings_HTTPSettings{
						UseClientProtocol: true,
						H2UpgradePolicy:   istioapinetworkingv1beta1.ConnectionPoolSettings_HTTPSettings_UPGRADE,
					},
				},
			}
			Expect(DetermineProtocolMode(destinationRule, port)).To(Equal(HTTPProtocolPolicyUseClientProtocol))
		})

		It("should prefer top-level traffic policy over port name", func() {
			port.Name = "grpc-main"
			destinationRule.Spec.TrafficPolicy = &istioapinetworkingv1beta1.TrafficPolicy{
				ConnectionPool: &istioapinetworkingv1beta1.ConnectionPoolSettings{
					Http: &istioapinetworkingv1beta1.ConnectionPoolSettings_HTTPSettings{
						UseClientProtocol: true,
					},
				},
			}
			Expect(DetermineProtocolMode(destinationRule, port)).To(Equal(HTTPProtocolPolicyUseClientProtocol))
		})

		It("should use port-level settings over top-level traffic policy", func() {
			destinationRule.Spec.TrafficPolicy = &istioapinetworkingv1beta1.TrafficPolicy{
				ConnectionPool: &istioapinetworkingv1beta1.ConnectionPoolSettings{
					Http: &istioapinetworkingv1beta1.ConnectionPoolSettings_HTTPSettings{
						UseClientProtocol: true,
					},
				},
				PortLevelSettings: []*istioapinetworkingv1beta1.TrafficPolicy_PortTrafficPolicy{
					{
						Port: &istioapinetworkingv1beta1.PortSelector{Number: 443},
						ConnectionPool: &istioapinetworkingv1beta1.ConnectionPoolSettings{
							Http: &istioapinetworkingv1beta1.ConnectionPoolSettings_HTTPSettings{
								H2UpgradePolicy: istioapinetworkingv1beta1.ConnectionPoolSettings_HTTPSettings_UPGRADE,
							},
						},
					},
				},
			}
			Expect(DetermineProtocolMode(destinationRule, port)).To(Equal(HTTPProtocolPolicyExplicitHTTP2))
		})

		It("should ignore port-level settings for non-matching ports", func() {
			destinationRule.Spec.TrafficPolicy = &istioapinetworkingv1beta1.TrafficPolicy{
				ConnectionPool: &istioapinetworkingv1beta1.ConnectionPoolSettings{
					Http: &istioapinetworkingv1beta1.ConnectionPoolSettings_HTTPSettings{
						UseClientProtocol: true,
					},
				},
				PortLevelSettings: []*istioapinetworkingv1beta1.TrafficPolicy_PortTrafficPolicy{
					{
						Port: &istioapinetworkingv1beta1.PortSelector{Number: 8080},
						ConnectionPool: &istioapinetworkingv1beta1.ConnectionPoolSettings{
							Http: &istioapinetworkingv1beta1.ConnectionPoolSettings_HTTPSettings{
								H2UpgradePolicy: istioapinetworkingv1beta1.ConnectionPoolSettings_HTTPSettings_UPGRADE,
							},
						},
					},
				},
			}
			Expect(DetermineProtocolMode(destinationRule, port)).To(Equal(HTTPProtocolPolicyUseClientProtocol))
		})

		It("should fall through to port name when port-level settings have no HTTP config", func() {
			port.Name = "grpc-main"
			destinationRule.Spec.TrafficPolicy = &istioapinetworkingv1beta1.TrafficPolicy{
				PortLevelSettings: []*istioapinetworkingv1beta1.TrafficPolicy_PortTrafficPolicy{
					{
						Port:           &istioapinetworkingv1beta1.PortSelector{Number: 443},
						ConnectionPool: &istioapinetworkingv1beta1.ConnectionPoolSettings{},
					},
				},
			}
			Expect(DetermineProtocolMode(destinationRule, port)).To(Equal(HTTPProtocolPolicyExplicitHTTP2))
		})

		It("should return ProtocolModeNone when traffic policy has empty connection pool", func() {
			destinationRule.Spec.TrafficPolicy = &istioapinetworkingv1beta1.TrafficPolicy{
				ConnectionPool: &istioapinetworkingv1beta1.ConnectionPoolSettings{},
			}
			Expect(DetermineProtocolMode(destinationRule, port)).To(Equal(HTTPProtocolPolicyDefault))
		})

		It("should return ProtocolModeNone when h2UpgradePolicy is DO_NOT_UPGRADE", func() {
			destinationRule.Spec.TrafficPolicy = &istioapinetworkingv1beta1.TrafficPolicy{
				ConnectionPool: &istioapinetworkingv1beta1.ConnectionPoolSettings{
					Http: &istioapinetworkingv1beta1.ConnectionPoolSettings_HTTPSettings{
						H2UpgradePolicy: istioapinetworkingv1beta1.ConnectionPoolSettings_HTTPSettings_DO_NOT_UPGRADE,
					},
				},
			}
			Expect(DetermineProtocolMode(destinationRule, port)).To(Equal(HTTPProtocolPolicyDefault))
		})
	})

	DescribeTable("#IsHTTP2Port",
		func(port corev1.ServicePort, expected bool) {
			Expect(IsHTTP2Port(port)).To(Equal(expected))
		},
		Entry("port name grpc-main", corev1.ServicePort{Name: "grpc-main", Port: 9090}, true),
		Entry("port name http2-server", corev1.ServicePort{Name: "http2-server", Port: 8080}, true),
		Entry("port name grpc-web", corev1.ServicePort{Name: "grpc-web", Port: 8080}, true),
		Entry("port name grpc-web-extra", corev1.ServicePort{Name: "grpc-web-extra", Port: 8080}, true),
		Entry("port name GRPC-WEB (case-insensitive)", corev1.ServicePort{Name: "GRPC-WEB", Port: 8080}, true),
		Entry("port name grpc (bare)", corev1.ServicePort{Name: "grpc", Port: 9090}, true),
		Entry("port name http2 (bare)", corev1.ServicePort{Name: "http2", Port: 8080}, true),
		Entry("port name https-main", corev1.ServicePort{Name: "https-main", Port: 443}, false),
		Entry("port name tcp-metrics", corev1.ServicePort{Name: "tcp-metrics", Port: 9090}, false),
		Entry("port name http-server", corev1.ServicePort{Name: "http-server", Port: 80}, false),
		Entry("empty port name", corev1.ServicePort{Name: "", Port: 80}, false),
		Entry("appProtocol kubernetes.io/h2c", corev1.ServicePort{Name: "https-main", Port: 443, AppProtocol: new("kubernetes.io/h2c")}, true),
		Entry("appProtocol grpc", corev1.ServicePort{Name: "tcp-main", Port: 443, AppProtocol: new("grpc")}, true),
		Entry("appProtocol http2", corev1.ServicePort{Name: "tcp-main", Port: 443, AppProtocol: new("http2")}, true),
		Entry("appProtocol grpc-web", corev1.ServicePort{Name: "tcp-main", Port: 443, AppProtocol: new("grpc-web")}, true),
		Entry("appProtocol http overrides grpc port name", corev1.ServicePort{Name: "grpc-main", Port: 443, AppProtocol: new("http")}, false),
		Entry("appProtocol tcp overrides grpc port name", corev1.ServicePort{Name: "grpc-main", Port: 443, AppProtocol: new("tcp")}, false),
	)
})

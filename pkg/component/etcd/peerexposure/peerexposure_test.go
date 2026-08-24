// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package peerexposure_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	istioapinetworkingv1beta1 "istio.io/api/networking/v1beta1"
	istionetworkingv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/gardener/gardener/pkg/component"
	. "github.com/gardener/gardener/pkg/component/etcd/peerexposure"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("PeerExposure", func() {
	var (
		ctx = context.Background()

		c          client.Client
		component_ component.DeployWaiter
		values     Values

		namespace         = "shoot--foo--bar"
		istioNS           = "istio-ingress"
		clientHost        = "src-etcd-main.ingress.seed.example.com"
		clientServiceFQDN = "etcd-main-client." + "shoot--foo--bar" + ".svc.cluster.local"
		istioLabels       = map[string]string{"istio": "ingressgateway"}
		members           = []PeerMember{
			{
				SNIHost:      "src-etcd-main-0.ingress.seed.example.com",
				PodFQDN:      "etcd-main-0.etcd-main-peer." + "shoot--foo--bar" + ".svc.cluster.local",
				ExternalPort: 12380,
			},
		}
		multiMembers = []PeerMember{
			{
				SNIHost:      "src-etcd-main-0.ingress.seed.example.com",
				PodFQDN:      "etcd-main-0.etcd-main-peer." + "shoot--foo--bar" + ".svc.cluster.local",
				ExternalPort: 12380,
			},
			{
				SNIHost:      "src-etcd-main-1.ingress.seed.example.com",
				PodFQDN:      "etcd-main-1.etcd-main-peer." + "shoot--foo--bar" + ".svc.cluster.local",
				ExternalPort: 12381,
			},
			{
				SNIHost:      "src-etcd-main-2.ingress.seed.example.com",
				PodFQDN:      "etcd-main-2.etcd-main-peer." + "shoot--foo--bar" + ".svc.cluster.local",
				ExternalPort: 12382,
			},
		}

		gateway        *istionetworkingv1beta1.Gateway
		virtualService *istionetworkingv1beta1.VirtualService
	)

	BeforeEach(func() {
		s := runtime.NewScheme()
		Expect(istionetworkingv1beta1.AddToScheme(s)).To(Succeed())
		Expect(networkingv1.AddToScheme(s)).To(Succeed())
		c = fakeclient.NewClientBuilder().WithScheme(s).Build()

		values = Values{
			Role:                         "main",
			Members:                      members,
			IstioIngressGatewayNamespace: istioNS,
			IstioIngressGatewayLabels:    istioLabels,
		}
		component_ = New(c, namespace, values)

		gateway = &istionetworkingv1beta1.Gateway{ObjectMeta: metav1.ObjectMeta{Name: "etcd-main-peer", Namespace: namespace}}
		virtualService = &istionetworkingv1beta1.VirtualService{ObjectMeta: metav1.ObjectMeta{Name: "etcd-main-peer", Namespace: namespace}}
	})

	Describe("#Deploy", func() {
		It("should create the gateway with TLS passthrough on the etcd peer port", func() {
			Expect(component_.Deploy(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(gateway), gateway)).To(Succeed())
			Expect(gateway.Labels).To(HaveKeyWithValue("app", "etcd-peer-exposure"))
			Expect(gateway.Spec.Selector).To(Equal(istioLabels))
			Expect(gateway.Spec.Servers).To(HaveLen(1))
			server := gateway.Spec.Servers[0]
			Expect(server.Hosts).To(Equal([]string{members[0].SNIHost}))
			Expect(server.Port.Number).To(Equal(uint32(12380)))
			Expect(server.Port.Name).To(Equal("tls-etcd-peer-0"))
			Expect(server.Port.Protocol).To(Equal("TLS"))
			Expect(server.Tls.Mode).To(Equal(istioapinetworkingv1beta1.ServerTLSSettings_PASSTHROUGH))
		})

		It("should create the virtual service with one per-member route so each SNI host reaches its own pod", func() {
			Expect(component_.Deploy(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(virtualService), virtualService)).To(Succeed())
			Expect(virtualService.Labels).To(HaveKeyWithValue("app", "etcd-peer-exposure"))
			Expect(virtualService.Spec.ExportTo).To(Equal([]string{istioNS}))
			Expect(virtualService.Spec.Hosts).To(Equal([]string{members[0].SNIHost}))
			Expect(virtualService.Spec.Gateways).To(Equal([]string{"etcd-main-peer"}))
			Expect(virtualService.Spec.Tls).To(HaveLen(1))
			route := virtualService.Spec.Tls[0]
			Expect(route.Match[0].Port).To(Equal(uint32(12380)))
			Expect(route.Match[0].SniHosts).To(Equal([]string{members[0].SNIHost}))
			Expect(route.Route[0].Destination.Host).To(Equal(members[0].PodFQDN))
			Expect(route.Route[0].Destination.Port.Number).To(Equal(uint32(2380)))
		})

		It("should create a ServiceEntry per member exporting the pod subdomain to the ingress namespace", func() {
			Expect(component_.Deploy(ctx)).To(Succeed())

			se := &istionetworkingv1beta1.ServiceEntry{ObjectMeta: metav1.ObjectMeta{Name: "etcd-main-peer-0", Namespace: namespace}}
			Expect(c.Get(ctx, client.ObjectKeyFromObject(se), se)).To(Succeed())
			Expect(se.Labels).To(HaveKeyWithValue("app", "etcd-peer-exposure"))
			Expect(se.Spec.Hosts).To(Equal([]string{members[0].PodFQDN}))
			Expect(se.Spec.ExportTo).To(Equal([]string{istioNS}))
			Expect(se.Spec.Resolution).To(Equal(istioapinetworkingv1beta1.ServiceEntry_DNS))
			Expect(se.Spec.Ports).To(HaveLen(1))
			Expect(se.Spec.Ports[0].Number).To(Equal(uint32(2380)))
			Expect(se.Spec.Ports[0].Name).To(Equal("tls-etcd-peer"))
			Expect(se.Spec.Ports[0].Protocol).To(Equal("TLS"))
		})

		It("should create a network policy allowing the istio ingress gateway to reach the etcd peer port", func() {
			Expect(component_.Deploy(ctx)).To(Succeed())

			networkPolicy := &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-istio-ingress-to-etcd-main-peer", Namespace: namespace}}
			Expect(c.Get(ctx, client.ObjectKeyFromObject(networkPolicy), networkPolicy)).To(Succeed())
			Expect(networkPolicy.Labels).To(HaveKeyWithValue("app", "etcd-peer-exposure"))
			Expect(networkPolicy.Spec.PodSelector.MatchLabels).To(HaveKeyWithValue("app", "etcd-statefulset"))
			Expect(networkPolicy.Spec.PodSelector.MatchLabels).To(HaveKeyWithValue("role", "main"))
			Expect(networkPolicy.Spec.PolicyTypes).To(ConsistOf(networkingv1.PolicyTypeIngress))
			Expect(networkPolicy.Spec.Ingress).To(HaveLen(1))
			Expect(networkPolicy.Spec.Ingress[0].From[0].NamespaceSelector.MatchLabels).To(HaveKeyWithValue("kubernetes.io/metadata.name", istioNS))
			Expect(networkPolicy.Spec.Ingress[0].From[0].PodSelector.MatchLabels).To(Equal(istioLabels))
			Expect(networkPolicy.Spec.Ingress[0].Ports[0].Port.IntValue()).To(Equal(2380))
		})

		It("should create an egress network policy in the istio-ingress namespace allowing the gateway to reach etcd pods on the peer port", func() {
			Expect(component_.Deploy(ctx)).To(Succeed())

			egressNP := &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-istio-ingress-to-etcd-main-peer-" + namespace, Namespace: istioNS}}
			Expect(c.Get(ctx, client.ObjectKeyFromObject(egressNP), egressNP)).To(Succeed())
			Expect(egressNP.Labels).To(HaveKeyWithValue("app", "etcd-peer-exposure"))
			Expect(egressNP.Spec.PodSelector.MatchLabels).To(Equal(istioLabels))
			Expect(egressNP.Spec.PolicyTypes).To(ConsistOf(networkingv1.PolicyTypeEgress))
			Expect(egressNP.Spec.Egress).To(HaveLen(1))
			Expect(egressNP.Spec.Egress[0].To[0].NamespaceSelector.MatchLabels).To(HaveKeyWithValue("kubernetes.io/metadata.name", namespace))
			Expect(egressNP.Spec.Egress[0].To[0].PodSelector.MatchLabels).To(HaveKeyWithValue("app", "etcd-statefulset"))
			Expect(egressNP.Spec.Egress[0].Ports[0].Port.IntValue()).To(Equal(2380))
		})

		Context("with multiple members", func() {
			BeforeEach(func() {
				values.Members = multiMembers
				component_ = New(c, namespace, values)
			})

			It("should create one gateway server per member with distinct ports and SNI hosts", func() {
				Expect(component_.Deploy(ctx)).To(Succeed())

				Expect(c.Get(ctx, client.ObjectKeyFromObject(gateway), gateway)).To(Succeed())
				Expect(gateway.Spec.Servers).To(HaveLen(3))
				for i, m := range multiMembers {
					srv := gateway.Spec.Servers[i]
					Expect(srv.Hosts).To(Equal([]string{m.SNIHost}))
					Expect(srv.Port.Number).To(Equal(m.ExternalPort))
					Expect(srv.Port.Name).To(Equal(fmt.Sprintf("tls-etcd-peer-%d", i)))
					Expect(srv.Tls.Mode).To(Equal(istioapinetworkingv1beta1.ServerTLSSettings_PASSTHROUGH))
				}
			})

			It("should create one virtual service TLS route per member routing to the correct pod", func() {
				Expect(component_.Deploy(ctx)).To(Succeed())

				Expect(c.Get(ctx, client.ObjectKeyFromObject(virtualService), virtualService)).To(Succeed())
				Expect(virtualService.Spec.Hosts).To(ConsistOf(
					multiMembers[0].SNIHost, multiMembers[1].SNIHost, multiMembers[2].SNIHost,
				))
				Expect(virtualService.Spec.Tls).To(HaveLen(3))
				for i, m := range multiMembers {
					route := virtualService.Spec.Tls[i]
					Expect(route.Match[0].Port).To(Equal(m.ExternalPort))
					Expect(route.Match[0].SniHosts).To(Equal([]string{m.SNIHost}))
					Expect(route.Route[0].Destination.Host).To(Equal(m.PodFQDN))
					Expect(route.Route[0].Destination.Port.Number).To(Equal(uint32(2380)))
				}
			})

			It("should create one indexed ServiceEntry per member", func() {
				Expect(component_.Deploy(ctx)).To(Succeed())

				for i, m := range multiMembers {
					se := &istionetworkingv1beta1.ServiceEntry{ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("etcd-main-peer-%d", i), Namespace: namespace}}
					Expect(c.Get(ctx, client.ObjectKeyFromObject(se), se)).To(Succeed())
					Expect(se.Spec.Hosts).To(Equal([]string{m.PodFQDN}))
				}
			})
		})

		Context("when ClientHost is set", func() {
			BeforeEach(func() {
				values.ClientHost = clientHost
				component_ = New(c, namespace, values)
			})

			It("should create a client gateway with TLS passthrough on the etcd client port", func() {
				Expect(component_.Deploy(ctx)).To(Succeed())

				clientGateway := &istionetworkingv1beta1.Gateway{ObjectMeta: metav1.ObjectMeta{Name: "etcd-main-client", Namespace: namespace}}
				Expect(c.Get(ctx, client.ObjectKeyFromObject(clientGateway), clientGateway)).To(Succeed())
				Expect(clientGateway.Spec.Selector).To(Equal(istioLabels))
				Expect(clientGateway.Spec.Servers).To(HaveLen(1))
				srv := clientGateway.Spec.Servers[0]
				Expect(srv.Hosts).To(Equal([]string{clientHost}))
				Expect(srv.Port.Number).To(Equal(uint32(12379)))
				Expect(srv.Port.Name).To(Equal("tls-etcd-client"))
				Expect(srv.Port.Protocol).To(Equal("TLS"))
				Expect(srv.Tls.Mode).To(Equal(istioapinetworkingv1beta1.ServerTLSSettings_PASSTHROUGH))
			})

			It("should create a client virtual service routing the SNI host to the etcd client service", func() {
				Expect(component_.Deploy(ctx)).To(Succeed())

				clientVS := &istionetworkingv1beta1.VirtualService{ObjectMeta: metav1.ObjectMeta{Name: "etcd-main-client", Namespace: namespace}}
				Expect(c.Get(ctx, client.ObjectKeyFromObject(clientVS), clientVS)).To(Succeed())
				Expect(clientVS.Spec.ExportTo).To(Equal([]string{istioNS}))
				Expect(clientVS.Spec.Hosts).To(Equal([]string{clientHost}))
				Expect(clientVS.Spec.Gateways).To(Equal([]string{"etcd-main-client"}))
				Expect(clientVS.Spec.Tls).To(HaveLen(1))
				route := clientVS.Spec.Tls[0]
				Expect(route.Match[0].Port).To(Equal(uint32(12379)))
				Expect(route.Match[0].SniHosts).To(Equal([]string{clientHost}))
				Expect(route.Route[0].Destination.Host).To(Equal(clientServiceFQDN))
				Expect(route.Route[0].Destination.Port.Number).To(Equal(uint32(2379)))
			})

			It("should create a client ServiceEntry exporting the etcd client service to the ingress namespace", func() {
				Expect(component_.Deploy(ctx)).To(Succeed())

				clientSE := &istionetworkingv1beta1.ServiceEntry{ObjectMeta: metav1.ObjectMeta{Name: "etcd-main-client", Namespace: namespace}}
				Expect(c.Get(ctx, client.ObjectKeyFromObject(clientSE), clientSE)).To(Succeed())
				Expect(clientSE.Spec.Hosts).To(Equal([]string{clientServiceFQDN}))
				Expect(clientSE.Spec.ExportTo).To(Equal([]string{istioNS}))
				Expect(clientSE.Spec.Resolution).To(Equal(istioapinetworkingv1beta1.ServiceEntry_DNS))
				Expect(clientSE.Spec.Ports).To(HaveLen(1))
				Expect(clientSE.Spec.Ports[0].Number).To(Equal(uint32(2379)))
				Expect(clientSE.Spec.Ports[0].Name).To(Equal("tls-etcd-client"))
				Expect(clientSE.Spec.Ports[0].Protocol).To(Equal("TLS"))
			})

			It("should create a network policy allowing the istio ingress gateway to reach the etcd client port", func() {
				Expect(component_.Deploy(ctx)).To(Succeed())

				clientNP := &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-istio-ingress-to-etcd-main-client", Namespace: namespace}}
				Expect(c.Get(ctx, client.ObjectKeyFromObject(clientNP), clientNP)).To(Succeed())
				Expect(clientNP.Spec.PodSelector.MatchLabels).To(HaveKeyWithValue("app", "etcd-statefulset"))
				Expect(clientNP.Spec.PodSelector.MatchLabels).To(HaveKeyWithValue("role", "main"))
				Expect(clientNP.Spec.PolicyTypes).To(ConsistOf(networkingv1.PolicyTypeIngress))
				Expect(clientNP.Spec.Ingress).To(HaveLen(1))
				Expect(clientNP.Spec.Ingress[0].From[0].NamespaceSelector.MatchLabels).To(HaveKeyWithValue("kubernetes.io/metadata.name", istioNS))
				Expect(clientNP.Spec.Ingress[0].From[0].PodSelector.MatchLabels).To(Equal(istioLabels))
				Expect(clientNP.Spec.Ingress[0].Ports[0].Port.IntValue()).To(Equal(2379))
			})

			It("should create an egress network policy in the istio namespace allowing the gateway to reach etcd pods on the client port", func() {
				Expect(component_.Deploy(ctx)).To(Succeed())

				clientEgressNP := &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-istio-ingress-to-etcd-main-client-" + namespace, Namespace: istioNS}}
				Expect(c.Get(ctx, client.ObjectKeyFromObject(clientEgressNP), clientEgressNP)).To(Succeed())
				Expect(clientEgressNP.Spec.PodSelector.MatchLabels).To(Equal(istioLabels))
				Expect(clientEgressNP.Spec.PolicyTypes).To(ConsistOf(networkingv1.PolicyTypeEgress))
				Expect(clientEgressNP.Spec.Egress).To(HaveLen(1))
				Expect(clientEgressNP.Spec.Egress[0].To[0].NamespaceSelector.MatchLabels).To(HaveKeyWithValue("kubernetes.io/metadata.name", namespace))
				Expect(clientEgressNP.Spec.Egress[0].To[0].PodSelector.MatchLabels).To(HaveKeyWithValue("app", "etcd-statefulset"))
				Expect(clientEgressNP.Spec.Egress[0].Ports[0].Port.IntValue()).To(Equal(2379))
			})
		})

		Context("when ClientHost is not set", func() {
			It("should not create any client resources", func() {
				Expect(component_.Deploy(ctx)).To(Succeed())

				clientGateway := &istionetworkingv1beta1.Gateway{ObjectMeta: metav1.ObjectMeta{Name: "etcd-main-client", Namespace: namespace}}
				Expect(c.Get(ctx, client.ObjectKeyFromObject(clientGateway), clientGateway)).To(BeNotFoundError())
			})
		})
	})

	Describe("#Destroy", func() {
		It("should delete the gateway, virtual service, service entries and network policies", func() {
			Expect(component_.Deploy(ctx)).To(Succeed())
			Expect(component_.Destroy(ctx)).To(Succeed())

			networkPolicy := &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-istio-ingress-to-etcd-main-peer", Namespace: namespace}}
			egressNP := &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-istio-ingress-to-etcd-main-peer-" + namespace, Namespace: istioNS}}
			se := &istionetworkingv1beta1.ServiceEntry{ObjectMeta: metav1.ObjectMeta{Name: "etcd-main-peer-0", Namespace: namespace}}
			Expect(c.Get(ctx, client.ObjectKeyFromObject(gateway), gateway)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(virtualService), virtualService)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(networkPolicy), networkPolicy)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(egressNP), egressNP)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(se), se)).To(BeNotFoundError())
		})

		Context("when ClientHost is set", func() {
			BeforeEach(func() {
				values.ClientHost = clientHost
				component_ = New(c, namespace, values)
			})

			It("should also delete all client resources", func() {
				Expect(component_.Deploy(ctx)).To(Succeed())
				Expect(component_.Destroy(ctx)).To(Succeed())

				clientGateway := &istionetworkingv1beta1.Gateway{ObjectMeta: metav1.ObjectMeta{Name: "etcd-main-client", Namespace: namespace}}
				clientVS := &istionetworkingv1beta1.VirtualService{ObjectMeta: metav1.ObjectMeta{Name: "etcd-main-client", Namespace: namespace}}
				clientSE := &istionetworkingv1beta1.ServiceEntry{ObjectMeta: metav1.ObjectMeta{Name: "etcd-main-client", Namespace: namespace}}
				clientNP := &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-istio-ingress-to-etcd-main-client", Namespace: namespace}}
				clientEgressNP := &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-istio-ingress-to-etcd-main-client-" + namespace, Namespace: istioNS}}
				Expect(c.Get(ctx, client.ObjectKeyFromObject(clientGateway), clientGateway)).To(BeNotFoundError())
				Expect(c.Get(ctx, client.ObjectKeyFromObject(clientVS), clientVS)).To(BeNotFoundError())
				Expect(c.Get(ctx, client.ObjectKeyFromObject(clientSE), clientSE)).To(BeNotFoundError())
				Expect(c.Get(ctx, client.ObjectKeyFromObject(clientNP), clientNP)).To(BeNotFoundError())
				Expect(c.Get(ctx, client.ObjectKeyFromObject(clientEgressNP), clientEgressNP)).To(BeNotFoundError())
			})
		})
	})
})

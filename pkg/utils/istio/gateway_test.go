// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package istio_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	istioapinetworkingv1beta1 "istio.io/api/networking/v1beta1"
	istionetworkingv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"

	. "github.com/gardener/gardener/pkg/utils/istio"
)

var _ = Describe("Gateway", func() {
	DescribeTable("#GatewayWithTLSPassthrough", func(labels map[string]string, istioLabels map[string]string, hosts []string) {
		gateway := &istionetworkingv1beta1.Gateway{}

		function := GatewayWithTLSPassthrough(gateway, labels, istioLabels, hosts)

		Expect(function).NotTo(BeNil())

		err := function()

		Expect(err).ShouldNot(HaveOccurred())
		Expect(gateway.Labels).To(Equal(labels))
		Expect(gateway.Spec.Selector).To(Equal(istioLabels))
		Expect(gateway.Spec.Servers).To(HaveLen(1))
		Expect(gateway.Spec.Servers[0].Hosts).To(Equal(hosts))
		Expect(gateway.Spec.Servers[0].Port.Number).To(Equal(uint32(443)))
	},

		Entry("Nil values", nil, nil, nil),
		Entry("Some values", map[string]string{"foo": "bar", "key": "value"}, map[string]string{"app": "istio", "istio": "gateway"}, []string{"host-1", "host-2"}),
	)

	DescribeTable("#GatewayWithTLSTermination", func(labels map[string]string, istioLabels map[string]string, hosts []string, tlsSecret string) {
		gateway := &istionetworkingv1beta1.Gateway{}

		function := GatewayWithTLSTermination(gateway, labels, istioLabels, hosts, tlsSecret)

		Expect(function).NotTo(BeNil())

		err := function()

		Expect(err).ShouldNot(HaveOccurred())
		Expect(gateway.Labels).To(Equal(labels))
		Expect(gateway.Spec.Selector).To(Equal(istioLabels))
		Expect(gateway.Spec.Servers).To(HaveLen(1))
		Expect(gateway.Spec.Servers[0].Hosts).To(Equal(hosts))
		Expect(gateway.Spec.Servers[0].Port.Number).To(Equal(uint32(443)))
		Expect(gateway.Spec.Servers[0].Tls.CredentialName).To(Equal(tlsSecret))
	},

		Entry("Nil values", nil, nil, nil, ""),
		Entry("Some values", map[string]string{"foo": "bar", "key": "value"}, map[string]string{"app": "istio", "istio": "gateway"}, []string{"host-1", "host-2"}, "my-secret"),
	)

	DescribeTable("#GatewayWithMutualTLS", func(labels map[string]string, istioLabels map[string]string, serverConfigs []ServerConfig) {
		gateway := &istionetworkingv1beta1.Gateway{}

		function := GatewayWithMutualTLS(gateway, labels, istioLabels, serverConfigs)

		Expect(function).NotTo(BeNil())

		err := function()

		Expect(err).ShouldNot(HaveOccurred())
		Expect(gateway.Labels).To(Equal(labels))
		Expect(gateway.Spec.Selector).To(Equal(istioLabels))
		Expect(gateway.Spec.Servers).To(HaveLen(len(serverConfigs)))
		for i, serverConfig := range serverConfigs {
			Expect(gateway.Spec.Servers[i].Hosts).To(Equal(serverConfig.Hosts))
			Expect(gateway.Spec.Servers[i].Port.Number).To(Equal(uint32(443)))
			Expect(gateway.Spec.Servers[i].Port.Name).To(Equal(serverConfig.PortName))
			Expect(gateway.Spec.Servers[i].Port.Protocol).To(Equal("HTTPS"))
			Expect(gateway.Spec.Servers[i].Tls.CredentialName).To(Equal(serverConfig.TLSSecret))
			Expect(gateway.Spec.Servers[i].Tls.Mode).To(Equal(istioapinetworkingv1beta1.ServerTLSSettings_OPTIONAL_MUTUAL))
			Expect(gateway.Spec.Servers[i].Tls.MinProtocolVersion).To(Equal(serverConfig.MinProtocolVersion))
		}
	},

		Entry("Nil values", nil, nil, []ServerConfig{{Hosts: nil, PortName: "", TLSSecret: "", MinProtocolVersion: istioapinetworkingv1beta1.ServerTLSSettings_TLS_AUTO}}),
		Entry("Some values", map[string]string{"foo": "bar", "key": "value"}, map[string]string{"app": "istio", "istio": "gateway"}, []ServerConfig{{Hosts: []string{"host-1", "host-2"}, PortName: "foo", TLSSecret: "my-secret", MinProtocolVersion: istioapinetworkingv1beta1.ServerTLSSettings_TLS_AUTO}}),
		Entry("Multiple servers", map[string]string{"foo": "bar", "key": "value"}, map[string]string{"app": "istio", "istio": "gateway"}, []ServerConfig{{Hosts: []string{"host-1", "host-2"}, PortName: "foo", TLSSecret: "my-secret", MinProtocolVersion: istioapinetworkingv1beta1.ServerTLSSettings_TLS_AUTO}, {Hosts: []string{"host-3", "host-4"}, PortName: "bar", TLSSecret: "my-other-secret", MinProtocolVersion: istioapinetworkingv1beta1.ServerTLSSettings_TLS_AUTO}}),
		Entry("TLSV1_2", map[string]string{"foo": "bar"}, map[string]string{"app": "istio"}, []ServerConfig{{Hosts: []string{"host-1"}, PortName: "tls", TLSSecret: "secret", MinProtocolVersion: istioapinetworkingv1beta1.ServerTLSSettings_TLSV1_2}}),
		Entry("TLSV1_3", map[string]string{"foo": "bar"}, map[string]string{"app": "istio"}, []ServerConfig{{Hosts: []string{"host-1"}, PortName: "tls", TLSSecret: "secret", MinProtocolVersion: istioapinetworkingv1beta1.ServerTLSSettings_TLSV1_3}}),
	)
})

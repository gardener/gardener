// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
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
	DescribeTable("#GatewayWithTLSPassthrough", func(labels map[string]string, istioLabels map[string]string, hosts []string, port uint32) {
		gateway := &istionetworkingv1beta1.Gateway{}

		function := GatewayWithTLSPassthrough(gateway, labels, istioLabels, hosts, port)

		Expect(function).NotTo(BeNil())

		err := function()

		Expect(err).ShouldNot(HaveOccurred())
		Expect(gateway.Labels).To(Equal(labels))
		Expect(gateway.Spec.Selector).To(Equal(istioLabels))
		Expect(gateway.Spec.Servers).To(HaveLen(1))
		Expect(gateway.Spec.Servers[0].Hosts).To(Equal(hosts))
		Expect(gateway.Spec.Servers[0].Port.Number).To(Equal(port))
	},

		Entry("Nil values", nil, nil, nil, uint32(0)),
		Entry("Some values", map[string]string{"foo": "bar", "key": "value"}, map[string]string{"app": "istio", "istio": "gateway"}, []string{"host-1", "host-2"}, uint32(123456)),
	)

	DescribeTable("#GatewayWithTLSTermination", func(labels map[string]string, istioLabels map[string]string, hosts []string, port uint32, tlsSecret string) {
		gateway := &istionetworkingv1beta1.Gateway{}

		function := GatewayWithTLSTermination(gateway, labels, istioLabels, hosts, port, tlsSecret)

		Expect(function).NotTo(BeNil())

		err := function()

		Expect(err).ShouldNot(HaveOccurred())
		Expect(gateway.Labels).To(Equal(labels))
		Expect(gateway.Spec.Selector).To(Equal(istioLabels))
		Expect(gateway.Spec.Servers).To(HaveLen(1))
		Expect(gateway.Spec.Servers[0].Hosts).To(Equal(hosts))
		Expect(gateway.Spec.Servers[0].Port.Number).To(Equal(port))
		Expect(gateway.Spec.Servers[0].Tls.CredentialName).To(Equal(tlsSecret))
	},

		Entry("Nil values", nil, nil, nil, uint32(0), ""),
		Entry("Some values", map[string]string{"foo": "bar", "key": "value"}, map[string]string{"app": "istio", "istio": "gateway"}, []string{"host-1", "host-2"}, uint32(123456), "my-secret"),
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
			Expect(gateway.Spec.Servers[i].Port.Number).To(Equal(serverConfig.Port))
			Expect(gateway.Spec.Servers[i].Port.Name).To(Equal(serverConfig.PortName))
			Expect(gateway.Spec.Servers[i].Port.Protocol).To(Equal("HTTPS"))
			Expect(gateway.Spec.Servers[i].Tls.CredentialName).To(Equal(serverConfig.TLSSecret))
			Expect(gateway.Spec.Servers[i].Tls.Mode).To(Equal(istioapinetworkingv1beta1.ServerTLSSettings_OPTIONAL_MUTUAL))
		}
	},

		Entry("Nil values", nil, nil, []ServerConfig{{Hosts: nil, Port: uint32(0), PortName: "", TLSSecret: ""}}),
		Entry("Some values", map[string]string{"foo": "bar", "key": "value"}, map[string]string{"app": "istio", "istio": "gateway"}, []ServerConfig{{Hosts: []string{"host-1", "host-2"}, Port: uint32(12345), PortName: "foo", TLSSecret: "my-secret"}}),
		Entry("Multiple servers", map[string]string{"foo": "bar", "key": "value"}, map[string]string{"app": "istio", "istio": "gateway"}, []ServerConfig{{Hosts: []string{"host-1", "host-2"}, Port: uint32(12345), PortName: "foo", TLSSecret: "my-secret"}, {Hosts: []string{"host-3", "host-4"}, Port: uint32(123456), PortName: "bar", TLSSecret: "my-other-secret"}}),
	)
})

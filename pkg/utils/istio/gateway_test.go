// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package istio_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	istionetworkingv1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"

	. "github.com/gardener/gardener/pkg/utils/istio"
)

var _ = Describe("Gateway", func() {
	DescribeTable("#GatewayWithTLSPassthrough", func(labels map[string]string, istioLabels map[string]string, hosts []string, port uint32) {
		gateway := &istionetworkingv1beta1.Gateway{}

		function := GatewayWithTLSPassthrough(gateway, labels, istioLabels, hosts, port)

		// ginkgo-linter:ignore-err-assert-warning
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

		// ginkgo-linter:ignore-err-assert-warning
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
})

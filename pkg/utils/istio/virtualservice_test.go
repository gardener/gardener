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

var _ = Describe("VirtualService", func() {
	DescribeTable("#VirtualServiceWithSNIMatch", func(labels map[string]string, hosts []string, gatewayName string, port uint32, destinationHost string) {
		virtualService := &istionetworkingv1beta1.VirtualService{}

		function := VirtualServiceWithSNIMatch(virtualService, labels, hosts, gatewayName, port, destinationHost)

		Expect(function).NotTo(BeNil())

		err := function()

		Expect(err).ShouldNot(HaveOccurred())
		Expect(virtualService.Labels).To(Equal(labels))
		Expect(virtualService.Spec.Hosts).To(Equal(hosts))
		Expect(virtualService.Spec.Gateways).To(HaveLen(1))
		Expect(virtualService.Spec.Gateways[0]).To(Equal(gatewayName))
		Expect(virtualService.Spec.Tls).To(HaveLen(1))
		Expect(virtualService.Spec.Tls[0].Match).To(HaveLen(1))
		Expect(virtualService.Spec.Tls[0].Match[0].Port).To(Equal(port))
		Expect(virtualService.Spec.Tls[0].Match[0].SniHosts).To(Equal(hosts))
		Expect(virtualService.Spec.Tls[0].Route).To(HaveLen(1))
		Expect(virtualService.Spec.Tls[0].Route[0].Destination.Host).To(Equal(destinationHost))
		Expect(virtualService.Spec.Tls[0].Route[0].Destination.Port.Number).To(Equal(port))
	},

		Entry("Nil values", nil, nil, "", uint32(0), ""),
		Entry("Some values", map[string]string{"foo": "bar", "key": "value"}, []string{"host-1", "host-2"}, "my-gateway", uint32(123456), "destination.namespace.svc.cluster.local"),
	)

	DescribeTable("#VirtualServiceForTLSTermination", func(labels map[string]string, hosts []string, gatewayName string, port uint32, destinationHost string) {
		virtualService := &istionetworkingv1beta1.VirtualService{}

		function := VirtualServiceForTLSTermination(virtualService, labels, hosts, gatewayName, port, destinationHost)

		Expect(function).NotTo(BeNil())

		err := function()

		Expect(err).ShouldNot(HaveOccurred())
		Expect(virtualService.Labels).To(Equal(labels))
		Expect(virtualService.Spec.Hosts).To(Equal(hosts))
		Expect(virtualService.Spec.Gateways).To(HaveLen(1))
		Expect(virtualService.Spec.Gateways[0]).To(Equal(gatewayName))
		Expect(virtualService.Spec.Http).To(HaveLen(1))
		Expect(virtualService.Spec.Http[0].Match).To(BeEmpty())
		Expect(virtualService.Spec.Http[0].Route).To(HaveLen(1))
		Expect(virtualService.Spec.Http[0].Route[0].Destination.Host).To(Equal(destinationHost))
		Expect(virtualService.Spec.Http[0].Route[0].Destination.Port.Number).To(Equal(port))
	},

		Entry("Nil values", nil, nil, "", uint32(0), ""),
		Entry("Some values", map[string]string{"foo": "bar", "key": "value"}, []string{"host-1", "host-2"}, "my-gateway", uint32(123456), "destination.namespace.svc.cluster.local"),
	)
})

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

var _ = Describe("DestinationRule", func() {
	DescribeTable("#DestinationRuleWithLocalityPreference", func(labels map[string]string, destinationHost string) {
		destinationRule := &istionetworkingv1beta1.DestinationRule{}

		function := DestinationRuleWithLocalityPreference(destinationRule, labels, destinationHost)

		Expect(function).NotTo(BeNil())

		err := function()

		Expect(err).ShouldNot(HaveOccurred())
		Expect(destinationRule.Labels).To(Equal(labels))
		Expect(destinationRule.Spec.Host).To(Equal(destinationHost))
	},

		Entry("Nil values", nil, ""),
		Entry("Some values", map[string]string{"foo": "bar", "key": "value"}, "destination.namespace.svc.cluster.local"),
	)

	DescribeTable("#DestinationRuleWithLocalityPreferenceAndTLS", func(labels map[string]string, destinationHost string, tlsMode istioapinetworkingv1beta1.ClientTLSSettings_TLSmode) {
		destinationRule := &istionetworkingv1beta1.DestinationRule{}
		tlsSettings := &istioapinetworkingv1beta1.ClientTLSSettings{Mode: tlsMode}
		function := DestinationRuleWithLocalityPreferenceAndTLS(destinationRule, labels, destinationHost, tlsSettings)

		Expect(function).NotTo(BeNil())

		err := function()

		Expect(err).ShouldNot(HaveOccurred())
		Expect(destinationRule.Labels).To(Equal(labels))
		Expect(destinationRule.Spec.Host).To(Equal(destinationHost))
		Expect(destinationRule.Spec.TrafficPolicy.Tls).To(Equal(tlsSettings))
	},

		Entry("Nil values", nil, "", istioapinetworkingv1beta1.ClientTLSSettings_DISABLE),
		Entry("Some values", map[string]string{"foo": "bar", "key": "value"}, "destination.namespace.svc.cluster.local", istioapinetworkingv1beta1.ClientTLSSettings_SIMPLE),
	)
})

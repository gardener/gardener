// Copyright 2024 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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

		function := DestinationRuleWithLocalityPreferenceAndTLS(destinationRule, labels, destinationHost, tlsMode)

		Expect(function).NotTo(BeNil())

		err := function()

		Expect(err).ShouldNot(HaveOccurred())
		Expect(destinationRule.Labels).To(Equal(labels))
		Expect(destinationRule.Spec.Host).To(Equal(destinationHost))
		Expect(destinationRule.Spec.TrafficPolicy.Tls.Mode).To(Equal(tlsMode))
	},

		Entry("Nil values", nil, "", istioapinetworkingv1beta1.ClientTLSSettings_DISABLE),
		Entry("Some values", map[string]string{"foo": "bar", "key": "value"}, "destination.namespace.svc.cluster.local", istioapinetworkingv1beta1.ClientTLSSettings_SIMPLE),
	)
})

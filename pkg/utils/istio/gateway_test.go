// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
		Expect(len(gateway.Spec.Servers)).To(Equal(1))
		Expect(gateway.Spec.Servers[0].Hosts).To(Equal(hosts))
		Expect(gateway.Spec.Servers[0].Port.Number).To(Equal(port))
	},

		Entry("Nil values", nil, nil, nil, uint32(0)),
		Entry("Some values", map[string]string{"foo": "bar", "key": "value"}, map[string]string{"app": "istio", "istio": "gateway"}, []string{"host-1", "host-2"}, uint32(123456)),
	)
})

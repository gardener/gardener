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

var _ = Describe("VirtualService", func() {
	DescribeTable("#VirtualServiceWithSNIMatch", func(labels map[string]string, hosts []string, gatewayName string, externalPort uint32, destinationHost string, destinationPort uint32) {
		virtualService := &istionetworkingv1beta1.VirtualService{}

		function := VirtualServiceWithSNIMatch(virtualService, labels, hosts, gatewayName, externalPort, destinationHost, destinationPort)

		Expect(function).NotTo(BeNil())

		err := function()

		Expect(err).ShouldNot(HaveOccurred())
		Expect(virtualService.Labels).To(Equal(labels))
		Expect(virtualService.Spec.Hosts).To(Equal(hosts))
		Expect(len(virtualService.Spec.Gateways)).To(Equal(1))
		Expect(virtualService.Spec.Gateways[0]).To(Equal(gatewayName))
		Expect(len(virtualService.Spec.Tls)).To(Equal(1))
		Expect(len(virtualService.Spec.Tls[0].Match)).To(Equal(1))
		Expect(virtualService.Spec.Tls[0].Match[0].Port).To(Equal(externalPort))
		Expect(virtualService.Spec.Tls[0].Match[0].SniHosts).To(Equal(hosts))
		Expect(len(virtualService.Spec.Tls[0].Route)).To(Equal(1))
		Expect(virtualService.Spec.Tls[0].Route[0].Destination.Host).To(Equal(destinationHost))
		Expect(virtualService.Spec.Tls[0].Route[0].Destination.Port.Number).To(Equal(destinationPort))
	},

		Entry("Nil values", nil, nil, "", uint32(0), "", uint32(0)),
		Entry("Some values", map[string]string{"foo": "bar", "key": "value"}, []string{"host-1", "host-2"}, "my-gateway", uint32(123456), "destination.namespace.svc.cluster.local", uint32(654321)),
	)
})

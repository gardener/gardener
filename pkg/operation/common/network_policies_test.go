// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package common_test

import (
	"net"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	. "github.com/gardener/gardener/pkg/operation/common"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("networkpolicies", func() {

	Describe("#AllPrivateNetworkBlocks", func() {

		It("should contain correct CIDRs", func() {
			result := AllPrivateNetworkBlocks()
			_, block8, _ := net.ParseCIDR("10.0.0.0/8")
			_, block12, _ := net.ParseCIDR("172.16.0.0/12")
			_, block16, _ := net.ParseCIDR("192.168.0.0/16")
			_, carrierGradeBlock, _ := net.ParseCIDR("100.64.0.0/10")
			Expect(result).To(ConsistOf(*block8, *block12, *block16, *carrierGradeBlock))
		})

	})

	Describe("#ToExceptNetworks", func() {

		It("should return correct result", func() {

			result, err := ToExceptNetworks(AllPrivateNetworkBlocks(), "10.10.0.0/24", "172.16.1.0/24", "192.168.1.0/24", "100.64.1.0/24")
			expectedResult := []interface{}{
				map[string]interface{}{
					"network": "10.0.0.0/8",
					"except":  []gardencorev1alpha1.CIDR{"10.10.0.0/24"},
				},
				map[string]interface{}{
					"network": "172.16.0.0/12",
					"except":  []gardencorev1alpha1.CIDR{"172.16.1.0/24"},
				},
				map[string]interface{}{
					"network": "192.168.0.0/16",
					"except":  []gardencorev1alpha1.CIDR{"192.168.1.0/24"},
				},
				map[string]interface{}{
					"network": "100.64.0.0/10",
					"except":  []gardencorev1alpha1.CIDR{"100.64.1.0/24"},
				},
			}

			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(ConsistOf(expectedResult))

		})

	})

})

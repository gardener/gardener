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
	. "github.com/gardener/gardener/pkg/operation/common"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
)

var _ = Describe("common", func() {
	Describe("utils", func() {
		Describe("#IdentifyAddressType", func() {
			It("should return a tuple with first value equals hostname", func() {
				address := "example.com"

				addrType, addr := IdentifyAddressType(address)

				Expect(addrType).To(Equal("hostname"))
				Expect(addr).To(BeNil())
			})

			It("should return a tuple with first value equals ip", func() {
				address := "127.0.0.1"

				addrType, addr := IdentifyAddressType(address)

				Expect(addrType).To(Equal("ip"))
				Expect(addr).NotTo(BeNil())
			})
		})

		Describe("#ComputeClusterIP", func() {
			It("should return a cluster IP as string", func() {
				var (
					ip   = "100.64.0.0"
					cidr = gardenv1beta1.CIDR(ip + "/13")
				)

				result := ComputeClusterIP(cidr, 10)

				Expect(result).To(Equal("100.64.0.10"))
			})
		})

		Describe("#DiskSize", func() {
			It("should return a string", func() {
				var (
					size    = "10"
					sizeInt = 10
				)

				result := DiskSize(size + "Gi")

				Expect(result).To(Equal(sizeInt))
			})
		})

		Describe("#GenerateAddonConfig", func() {
			Context("values=nil and enabled=false", func() {
				It("should return a map with key enabled=false", func() {
					var (
						values  map[string]interface{}
						enabled = false
					)

					result := GenerateAddonConfig(values, enabled)

					Expect(result).To(SatisfyAll(
						HaveKeyWithValue("enabled", enabled),
						HaveLen(1),
					))
				})
			})

			Context("values=nil and enabled=true", func() {
				It("should return a map with key enabled=true", func() {
					var (
						values  map[string]interface{}
						enabled = true
					)

					result := GenerateAddonConfig(values, enabled)

					Expect(result).To(SatisfyAll(
						HaveKeyWithValue("enabled", enabled),
						HaveLen(1),
					))
				})
			})

			Context("values=<empty map> and enabled=true", func() {
				It("should return a map with key enabled=true", func() {
					var (
						values  = map[string]interface{}{}
						enabled = true
					)

					result := GenerateAddonConfig(values, enabled)

					Expect(result).To(SatisfyAll(
						HaveKeyWithValue("enabled", enabled),
						HaveLen(1),
					))
				})
			})

			Context("values=<non-empty map> and enabled=true", func() {
				It("should return a map with the values and key enabled=true", func() {
					var (
						values = map[string]interface{}{
							"foo": "bar",
						}
						enabled = true
					)

					result := GenerateAddonConfig(values, enabled)

					for key := range values {
						_, ok := result[key]
						Expect(ok).To(BeTrue())
					}
					Expect(result).To(SatisfyAll(
						HaveKeyWithValue("enabled", enabled),
						HaveLen(1+len(values)),
					))
				})
			})

			Context("values=<non-empty map> and enabled=false", func() {
				It("should return a map with key enabled=false", func() {
					var (
						values = map[string]interface{}{
							"foo": "bar",
						}
						enabled = false
					)

					result := GenerateAddonConfig(values, enabled)

					Expect(result).To(SatisfyAll(
						HaveKeyWithValue("enabled", enabled),
						HaveLen(1),
					))
				})
			})
		})
	})
})

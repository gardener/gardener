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

package utils_test

import (
	gardencore "github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apis/garden"
	. "github.com/gardener/gardener/plugin/pkg/utils"

	"k8s.io/apimachinery/pkg/util/validation/field"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
)

var _ = Describe("utils", func() {
	Describe("#ValidateNetworkDisjointedness", func() {
		var (
			seedPodsCIDR     = gardencore.CIDR("10.241.128.0/17")
			seedServicesCIDR = gardencore.CIDR("10.241.0.0/17")
			seedNodesCIDR    = gardencore.CIDR("10.240.0.0/16")

			seedNetworks = garden.SeedNetworks{
				Pods:     seedPodsCIDR,
				Services: seedServicesCIDR,
				Nodes:    seedNodesCIDR,
			}
		)

		It("should pass the validation", func() {
			var (
				podsCIDR     = gardencore.CIDR("10.242.128.0/17")
				servicesCIDR = gardencore.CIDR("10.242.0.0/17")
				nodesCIDR    = gardencore.CIDR("10.241.0.0/16")

				validK8sNetworks = gardencore.K8SNetworks{
					Pods:     &podsCIDR,
					Services: &servicesCIDR,
					Nodes:    &nodesCIDR,
				}
			)

			errorList := ValidateNetworkDisjointedness(seedNetworks, validK8sNetworks, field.NewPath(""))

			Expect(errorList).To(BeEmpty())
		})

		It("should fail due to disjointedness", func() {
			var (
				podsCIDR     = seedPodsCIDR
				servicesCIDR = seedServicesCIDR
				nodesCIDR    = seedNodesCIDR

				validK8sNetworks = gardencore.K8SNetworks{
					Pods:     &podsCIDR,
					Services: &servicesCIDR,
					Nodes:    &nodesCIDR,
				}
			)

			errorList := ValidateNetworkDisjointedness(seedNetworks, validK8sNetworks, field.NewPath(""))

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("[].nodes"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("[].services"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("[].pods"),
			}))))
		})

		It("should fail due to missing fields", func() {
			var (
				validK8sNetworks = gardencore.K8SNetworks{
					Pods:     nil,
					Services: nil,
					Nodes:    nil,
				}
			)

			errorList := ValidateNetworkDisjointedness(seedNetworks, validK8sNetworks, field.NewPath(""))

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("[].nodes"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("[].services"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("[].pods"),
			}))))
		})
	})
})

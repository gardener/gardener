// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package core_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/gardener/gardener/pkg/apis/core"
	utilcore "github.com/gardener/gardener/pkg/utils/validation/gardener/core"
)

var _ = Describe("Capabilities utility tests", func() {
	var (
		capabilities = core.Capabilities{
			"architecture":   "amd64,arm64",
			"hypervisorType": "gen1,gen2,gen3",
		}
	)

	Describe("#ParseCapabilities", func() {
		It("should retain the order of the CapabilityValues after parsing", func() {
			parsedCapabilities := utilcore.ParseCapabilities(capabilities)
			architectureValues := parsedCapabilities["architecture"]
			Expect(architectureValues).To(Equal(utilcore.CapabilityValues{"amd64", "arm64"}))
			Expect(architectureValues).NotTo(Equal(utilcore.CapabilityValues{"arm64", "amd64"}))

			hypervisorValues := parsedCapabilities["hypervisorType"]
			Expect(hypervisorValues).To(Equal(utilcore.CapabilityValues{"gen1", "gen2", "gen3"}))
		})

		It("should sanitize capability values on parsing", func() {
			var capabilities2 = core.Capabilities{
				"architecture":   "  amd64 ,arm64 ,amd32,  asap       , I look weird  ",
				"hypervisorType": "gen1, gen4",
			}
			parsedCapabilities := utilcore.ParseCapabilities(capabilities2)

			architectureValues := parsedCapabilities["architecture"]
			Expect(architectureValues).To(ConsistOf("amd64", "arm64", "amd32", "asap", "I look weird"))
			Expect(architectureValues.Contains("amd64", "I look weird")).To(BeTrue())
			hypervisorValues := parsedCapabilities["hypervisorType"]
			Expect(hypervisorValues).To(ConsistOf("gen1", "gen4"))
			Expect(hypervisorValues.Contains("gen1", "gen4")).To(BeTrue())
		})

	})
})

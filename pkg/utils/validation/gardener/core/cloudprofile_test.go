// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package core_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/gardener/gardener/pkg/apis/core"
	. "github.com/gardener/gardener/pkg/utils/validation/gardener/core"
)

var _ = Describe("Capabilities utility tests", func() {

	Describe("#ValidateCapabilitiesAgainstDefinition", func() {
		var dummyPath = field.NewPath("dummy")
		capabilitiesDefinition := core.Capabilities{
			"architecture": {
				Values: []string{"amd64", "arm64"},
			},
			"hypervisorType": {
				Values: []string{"gen1", "gen2", "gen3"},
			},
		}

		It("should pass validation with correct capabilities", func() {
			capabilities := core.Capabilities{
				"architecture": {
					Values: []string{"amd64"},
				},
				"hypervisorType": {
					Values: []string{"gen2"},
				},
			}

			errList := ValidateCapabilitiesAgainstDefinition(capabilities, capabilitiesDefinition, dummyPath)
			Expect(errList).To(BeEmpty())
		})

		It("should fail validation with empty capability values", func() {
			capabilities := core.Capabilities{
				"architecture": {
					Values: []string{},
				},
			}

			errList := ValidateCapabilitiesAgainstDefinition(capabilities, capabilitiesDefinition, dummyPath)
			Expect(errList).To(HaveLen(1))
			Expect(errList[0].Type).To(Equal(field.ErrorTypeInvalid))
			Expect(errList[0].Field).To(Equal("dummy.architecture"))
			Expect(errList[0].Detail).To(Equal("must not be empty"))
		})

		It("should fail validation with non-subset capability values", func() {
			capabilities := core.Capabilities{
				"architecture": {
					Values: []string{"x86"},
				},
			}

			errList := ValidateCapabilitiesAgainstDefinition(capabilities, capabilitiesDefinition, dummyPath)
			Expect(errList).To(HaveLen(1))
			Expect(errList[0].Type).To(Equal(field.ErrorTypeInvalid))
			Expect(errList[0].Field).To(Equal("dummy.architecture"))
			Expect(errList[0].Detail).To(Equal("must be a subset of spec.capabilitiesDefinition of the provider's cloudProfile"))
		})
	})
})

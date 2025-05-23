// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/gardener/gardener/plugin/pkg/shoot/dnsrewriting/apis/shootdnsrewriting"
	. "github.com/gardener/gardener/plugin/pkg/shoot/dnsrewriting/apis/shootdnsrewriting/validation"
)

var _ = Describe("Validation", func() {
	Describe("#ValidateConfiguration", func() {
		var config *shootdnsrewriting.Configuration

		BeforeEach(func() {
			config = &shootdnsrewriting.Configuration{}
		})

		It("should allow empty configuration", func() {
			errorList := ValidateConfiguration(config)

			Expect(errorList).To(BeEmpty())
		})

		It("should allow valid suffixes", func() {
			config.CommonSuffixes = []string{"gardener.cloud", ".github.com"}

			errorList := ValidateConfiguration(config)

			Expect(errorList).To(BeEmpty())
		})

		It("should forbid invalid suffixes", func() {
			config.CommonSuffixes = []string{"foo", ".foo", ".foo.bar", "foo.bar"}

			errorList := ValidateConfiguration(config)

			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":     Equal(field.ErrorTypeInvalid),
					"Field":    Equal("commonSuffixes[0]"),
					"Detail":   Equal("must contain at least one non-leading dot ('.')"),
					"BadValue": Equal("foo"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":     Equal(field.ErrorTypeInvalid),
					"Field":    Equal("commonSuffixes[1]"),
					"Detail":   Equal("must contain at least one non-leading dot ('.')"),
					"BadValue": Equal(".foo"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":     Equal(field.ErrorTypeDuplicate),
					"Field":    Equal("commonSuffixes[1]"),
					"BadValue": Equal("foo"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":     Equal(field.ErrorTypeDuplicate),
					"Field":    Equal("commonSuffixes[3]"),
					"BadValue": Equal("foo.bar"),
				})),
			))
		})
	})
})

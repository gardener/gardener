// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/gardener/gardener/pkg/apis/core"
	. "github.com/gardener/gardener/pkg/apis/core/validation"
)

var _ = Describe("Utils tests", func() {
	Describe("#ValidateFailureToleranceTypeValue", func() {
		var fldPath *field.Path

		BeforeEach(func() {
			fldPath = field.NewPath("spec", "highAvailability", "failureTolerance", "type")
		})

		It("highAvailability is set to failureTolerance of node", func() {
			errorList := ValidateFailureToleranceTypeValue(core.FailureToleranceTypeNode, fldPath)
			Expect(errorList).To(BeEmpty())
		})

		It("highAvailability is set to failureTolerance of zone", func() {
			errorList := ValidateFailureToleranceTypeValue(core.FailureToleranceTypeZone, fldPath)
			Expect(errorList).To(BeEmpty())
		})

		It("highAvailability is set to an unsupported value", func() {
			errorList := ValidateFailureToleranceTypeValue("region", fldPath)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeNotSupported),
					"Field": Equal(fldPath.String()),
				}))))
		})
	})

	Describe("#ValidateIPFamilies", func() {
		var fldPath *field.Path

		BeforeEach(func() {
			fldPath = field.NewPath("ipFamilies")
		})

		It("should deny unsupported IP families", func() {
			errorList := ValidateIPFamilies([]core.IPFamily{"foo", "bar"}, fldPath)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":     Equal(field.ErrorTypeNotSupported),
					"Field":    Equal(fldPath.Index(0).String()),
					"BadValue": BeEquivalentTo("foo"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":     Equal(field.ErrorTypeNotSupported),
					"Field":    Equal(fldPath.Index(1).String()),
					"BadValue": BeEquivalentTo("bar"),
				})),
			))
		})

		It("should deny duplicate IP families", func() {
			errorList := ValidateIPFamilies([]core.IPFamily{core.IPFamilyIPv4, core.IPFamilyIPv6, core.IPFamilyIPv4, core.IPFamilyIPv6}, fldPath)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":     Equal(field.ErrorTypeDuplicate),
					"Field":    Equal(fldPath.Index(2).String()),
					"BadValue": Equal(core.IPFamilyIPv4),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":     Equal(field.ErrorTypeDuplicate),
					"Field":    Equal(fldPath.Index(3).String()),
					"BadValue": Equal(core.IPFamilyIPv6),
				})),
			))
		})

		It("should allow IPv4 single-stack", func() {
			errorList := ValidateIPFamilies([]core.IPFamily{core.IPFamilyIPv4}, fldPath)
			Expect(errorList).To(BeEmpty())
		})

		It("should allow IPv6 single-stack", func() {
			errorList := ValidateIPFamilies([]core.IPFamily{core.IPFamilyIPv6}, fldPath)
			Expect(errorList).To(BeEmpty())
		})
	})
})

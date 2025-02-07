// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation_test

import (
	"github.com/bsm/gomega/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"

	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
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

	Describe("#ValidateCapabilities", func() {
		var (
			dummyPath    *field.Path       = field.NewPath("dummy")
			capabilities core.Capabilities = map[string]string{
				"architecture":   "amd64,arm64",
				"hypervisorType": "gen1,gen2,gen3",
			}
		)
		It("should accept a capabilitiesDefinition with architecture as capability", func() {
			errorList := ValidateCapabilitiesDefinition(capabilities, dummyPath)
			Expect(errorList).To(BeEmpty())

		})

		DescribeTable("should reject invalid capabilitiesDefinition",
			func(capabilities core.Capabilities, expectedError []types.GomegaMatcher) {
				errorList := ValidateCapabilitiesDefinition(capabilities, dummyPath)
				Expect(errorList).To(ConsistOf(expectedError))
			},
			Entry("empty capability values", core.Capabilities{"architecture": "amd64", "hypervisorType": ""}, []types.GomegaMatcher{
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal(dummyPath.Child("hypervisorType").String()),
				})),
			}),
			Entry("missing architecture capability", core.Capabilities{"hypervisorType": "gen1,gen2,gen3"}, []types.GomegaMatcher{
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal(dummyPath.Child("architecture").String()),
				})),
			}),
		)

		It("should unmarshal correct CapabilitiesSet", func() {
			var CapabilitiesSet = []v1.JSON{
				{Raw: []byte(`{"architecture":"amd64","hypervisorType":"gen2"}`)},
				{Raw: []byte(`{"architecture":"amd64","hypervisorType":"gen2"}`)},
				{Raw: []byte(`{"architecture":"arm64","hypervisorType":"gen1"}`)},
				{Raw: []byte(`{"architecture":"arm64","hypervisorType":"gen2,gen3"}`)},
			}
			capabilitiesSet, err := UnmarshalCapabilitiesSet(CapabilitiesSet, dummyPath)

			Expect(err).To(BeNil())
			Expect(capabilitiesSet).To(HaveLen(4))
			Expect(capabilitiesSet[0]).To(Equal(core.Capabilities{"architecture": "amd64", "hypervisorType": "gen2"}))
		})

		It("should error on unparsable CapabilitiesSet", func() {
			var CapabilitiesSet = []v1.JSON{
				{Raw: []byte(`{"architecture":"amd64","hypervisorType":"gen2"}`)},
				{Raw: []byte(`{"architecture":"amd64","hype....🆘`)},
			}

			capabilitiesSet, err := UnmarshalCapabilitiesSet(CapabilitiesSet, dummyPath)

			Expect(err).To(ConsistOf([]types.GomegaMatcher{
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("dummy[1]"),
				})),
			}))
			Expect(capabilitiesSet[1]).To(HaveLen(0))
		})

		It("should sanitize capability values on parsing", func() {
			var capabilities2 core.Capabilities = map[string]string{
				"architecture":   "  amd64 ,arm64 ,amd32, , 'asdas   '    , I look wierd  ",
				"hypervisorType": `"gen1", "gen4"`,
			}
			parsedCapabilities := ParseCapabilityValues(capabilities2)
			architectureSet := parsedCapabilities["architecture"]
			hypervisorTypeSet := parsedCapabilities["hypervisorType"]

			Expect(architectureSet.Contains("amd64", "arm64", "amd32", "I look wierd", "asdas")).To(BeTrue())
			Expect(architectureSet).To(HaveLen(5))
			Expect(hypervisorTypeSet.Contains("gen1", "gen4")).To(BeTrue())
			Expect(hypervisorTypeSet).To(HaveLen(2))
		})

		It("should create intersection of capabilities", func() {
			var capabilities2 core.Capabilities = map[string]string{
				"architecture":   "amd64,arm64,amd32",
				"hypervisorType": "gen1,gen4",
				"notIntersect":   "value",
			}
			intersection := GetCapabilitiesIntersection(ParseCapabilityValues(capabilities), ParseCapabilityValues(capabilities2))
			isArchitectureIntersection := intersection["architecture"].Contains("amd64", "arm64")
			isHypervisorTypeIntersection := intersection["hypervisorType"].Contains("gen1")
			Expect(isArchitectureIntersection).To(BeTrue())
			Expect(isHypervisorTypeIntersection).To(BeTrue())
			Expect(len(intersection)).To(Equal(2))
		})
	})

})

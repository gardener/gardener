// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/gardener/gardener/pkg/apis/core"
	. "github.com/gardener/gardener/pkg/apis/core/validation"
)

var _ = Describe("Capabilities utility tests", func() {
	var (
		dummyPath                      = field.NewPath("dummy")
		capabilities core.Capabilities = map[string]string{
			"architecture":   "amd64,arm64",
			"hypervisorType": "gen1,gen2,gen3",
		}
	)

	Describe("#ValidateCapabilities", func() {
		It("should accept a capabilitiesDefinition with architecture as capability", func() {
			errorList := ValidateCapabilitiesDefinition(capabilities, dummyPath)
			Expect(errorList).To(BeEmpty())
		})

		DescribeTable("should reject invalid capabilitiesDefinition",
			func(capabilities core.Capabilities, expectedError []gomegatypes.GomegaMatcher) {
				errorList := ValidateCapabilitiesDefinition(capabilities, dummyPath)
				Expect(errorList).To(ConsistOf(expectedError))
			},
			Entry("reject empty capability name", core.Capabilities{"architecture": "amd64", "": "gen1,gen2,gen3"}, []gomegatypes.GomegaMatcher{
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(dummyPath.String()),
				})),
			}),
			Entry("empty capability values", core.Capabilities{"architecture": "amd64", "hypervisorType": ""}, []gomegatypes.GomegaMatcher{
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(dummyPath.Child("hypervisorType").String()),
				})),
			}),
			Entry("missing architecture capability", core.Capabilities{"hypervisorType": "gen1,gen2,gen3"}, []gomegatypes.GomegaMatcher{
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal(dummyPath.Child("architecture").String()),
				})),
			}),
		)

	})

	Describe("#UnmarshalCapabilitiesSet", func() {
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
			var rawCapabilitiesSet = []v1.JSON{
				{Raw: []byte(`{"architecture":"amd64","hypervisorType":"gen2"}`)},
				// invalid JSON
				{Raw: []byte(`{"architecture":"amd64","hype....🆘`)},
				// number cannot be unmarshalled as string
				{Raw: []byte(`{"invalid":12}`)},
				// array cannot be unmarshalled as string
				{Raw: []byte(`{"invalid":["a","b"]}`)},
				// object cannot be unmarshalled as string
				{Raw: []byte(`{"invalid":{"a":"b"}}`)},
				// boolean cannot be unmarshalled as string
				{Raw: []byte(`{"invalid":true}`)},
				// empty object cannot be unmarshalled as string
				{Raw: []byte(`{"invalid":{}}`)},
				// empty array cannot be unmarshalled as string
				{Raw: []byte(`{"invalid":[]}`)},
			}

			_, err := UnmarshalCapabilitiesSet(rawCapabilitiesSet, dummyPath)

			Expect(err).To(ConsistOf([]gomegatypes.GomegaMatcher{
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("dummy[1]"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("dummy[2]"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("dummy[3]"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("dummy[4]"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("dummy[5]"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("dummy[6]"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("dummy[7]"),
				})),
			}))
		})

		It("should reject when the RAW CapabilitiesSet is not Unmarshalled completely", func() {
			var rawCapabilitiesSet = []v1.JSON{
				{Raw: []byte(`{"architecture":"amd64","hypervisorType":"gen2"}`)},
				{Raw: []byte(`{"architecture": "amd64",  "hypervisorType": "gen2",  "extraField": {"a": "b"}`)},
				{Raw: []byte(`{"architecture":"arm64","hypervisorType":"gen1"}`)},
			}
			capabilitiesSet, err := UnmarshalCapabilitiesSet(rawCapabilitiesSet, dummyPath)
			Expect(err).To(ConsistOf([]gomegatypes.GomegaMatcher{
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("dummy[1]"),
				})),
			}))
			Expect(capabilitiesSet).To(HaveLen(3))
		})
	})

	Describe("#ParseCapabilities", func() {
		It("should retain the order of the CapabilityValues after parsing", func() {
			parsedCapabilities := ParseCapabilities(capabilities)
			architectureValues := parsedCapabilities["architecture"]
			Expect(architectureValues).To(Equal(CapabilityValues{"amd64", "arm64"}))
			Expect(architectureValues).NotTo(Equal(CapabilityValues{"arm64", "amd64"}))

			hypervisorValues := parsedCapabilities["hypervisorType"]
			Expect(hypervisorValues).To(Equal(CapabilityValues{"gen1", "gen2", "gen3"}))
		})

		It("should sanitize capability values on parsing", func() {
			var capabilities2 = core.Capabilities{
				"architecture":   "  amd64 ,arm64 ,amd32,  asap       , I look weird  ",
				"hypervisorType": "gen1, gen4",
			}
			parsedCapabilities := ParseCapabilities(capabilities2)

			architectureValues := parsedCapabilities["architecture"]
			Expect(architectureValues).To(ConsistOf("amd64", "arm64", "amd32", "asap", "I look weird"))
			Expect(architectureValues.Contains("amd64", "I look weird")).To(BeTrue())
			hypervisorValues := parsedCapabilities["hypervisorType"]
			Expect(hypervisorValues).To(ConsistOf("gen1", "gen4"))
			Expect(hypervisorValues.Contains("gen1", "gen4")).To(BeTrue())
		})

		It("should create intersection of capabilities", func() {
			var capabilities2 core.Capabilities = map[string]string{
				"architecture":   "amd64,arm64,amd32",
				"hypervisorType": "gen1,gen4",
				"notIntersect":   "value",
			}
			intersection := GetCapabilitiesIntersection(ParseCapabilities(capabilities), ParseCapabilities(capabilities2))
			architectureValues := intersection["architecture"]
			Expect(architectureValues.Contains("amd64", "arm64")).To(BeTrue())
			hypervisorValues := intersection["hypervisorType"]
			Expect(hypervisorValues.Contains("gen1")).To(BeTrue())
			noIntersectValues := intersection["notIntersect"]
			Expect(noIntersectValues.Contains("value")).To(BeFalse())
			Expect(intersection).To(HaveLen(2))
		})
	})
})

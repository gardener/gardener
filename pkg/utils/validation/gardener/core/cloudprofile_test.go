// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package core_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
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

	Describe("#UnmarshalCapabilitiesSet", func() {
		var dummyPath = field.NewPath("dummy")
		It("should unmarshal correct CapabilitiesSet", func() {

			var rawCapabilitiesSet = core.CapabilitiesSet{
				{Raw: []byte(`{"architecture":"amd64","hypervisorType":"gen2"}`)},
				{Raw: []byte(`{"architecture":"amd64","hypervisorType":"gen2"}`)},
				{Raw: []byte(`{"architecture":"arm64","hypervisorType":"gen1"}`)},
				{Raw: []byte(`{"architecture":"arm64","hypervisorType":"gen2,gen3"}`)},
			}
			capabilitiesSlice, err := UnmarshalCapabilitiesSet(rawCapabilitiesSet, dummyPath)
			expectedCapabilities := core.Capabilities{
				"architecture": {
					Values: []string{"amd64"},
				},
				"hypervisorType": {
					Values: []string{"gen2"},
				},
			}

			Expect(err).To(BeNil())
			Expect(capabilitiesSlice).To(HaveLen(4))
			Expect(capabilitiesSlice[0]).To(Equal(expectedCapabilities))
		})

		It("should error on unparsable CapabilitiesSet", func() {
			var rawCapabilitiesSet = core.CapabilitiesSet{
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
			var rawCapabilitiesSet = core.CapabilitiesSet{
				{Raw: []byte(`{"architecture":"amd64","hypervisorType":"gen2"}`)},
				{Raw: []byte(`{"architecture": "amd64",  "hypervisorType": "gen2",  "extraField": {"a": "b"}`)},
				{Raw: []byte(`{"architecture":"arm64","hypervisorType":"gen1"}`)},
			}

			capabilitiesSlice, err := UnmarshalCapabilitiesSet(rawCapabilitiesSet, dummyPath)
			Expect(err).To(ConsistOf([]gomegatypes.GomegaMatcher{
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("dummy[1]"),
				})),
			}))
			Expect(capabilitiesSlice).To(HaveLen(3))
		})
	})
})

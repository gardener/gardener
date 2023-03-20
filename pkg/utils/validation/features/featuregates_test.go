// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package features_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	"k8s.io/apimachinery/pkg/util/validation/field"

	. "github.com/gardener/gardener/pkg/utils/validation/features"
)

var _ = Describe("featuregates", func() {
	DescribeTable("#IsFeatureGateSupported",
		func(featureGate, version string, supported, success bool) {
			result, err := IsFeatureGateSupported(featureGate, version)
			if success {
				Expect(err).To(Not(HaveOccurred()))
				Expect(result).To(Equal(supported))
			} else {
				Expect(err).To(HaveOccurred())
			}
		},

		Entry("AnyVolumeDataSource is supported in 1.18.14", "AnyVolumeDataSource", "1.18.14", true, true),                // AddedInVersion: 1.18
		Entry("CSIStorageCapacity is not supported in 1.18.14", "CSIStorageCapacity", "1.18.14", false, true),             // AddedInVersion: 1.19
		Entry("CustomResourceValidation is not supported in 1.18.14", "CustomResourceValidation", "1.18.14", false, true), // RemovedInVersion: 1.18
		Entry("DefaultIngressClass is supported in 1.18.14", "DefaultIngressClass", "1.18.14", true, true),                // AddedInVersion: 1.18, RemovedInVersion: 1.20
		Entry("DynamicAuditing is supported in 1.18.14", "DynamicAuditing", "1.18.14", true, true),                        // RemovedInVersion: 1.19
		Entry("Foo is unknown in 1.18.14", "Foo", "1.18.14", false, false),                                                // Unknown

		Entry("AnyVolumeDataSource is supported in 1.19.10", "AnyVolumeDataSource", "1.19.10", true, true),                // AddedInVersion: 1.18
		Entry("CSIStorageCapacity is supported in 1.19.10", "CSIStorageCapacity", "1.19.10", true, true),                  // AddedInVersion: 1.19
		Entry("CustomResourceValidation is not supported in 1.19.10", "CustomResourceValidation", "1.19.10", false, true), // RemovedInVersion: 1.18
		Entry("DefaultIngressClass is supported in 1.19.10", "DefaultIngressClass", "1.19.10", true, true),                // AddedInVersion: 1.18, RemovedInVersion: 1.20
		Entry("DynamicAuditing is not supported in 1.19.10", "DynamicAuditing", "1.19.10", false, true),                   // RemovedInVersion: 1.19
		Entry("Foo is unknown in 1.19.10", "Foo", "1.19.10", false, false),                                                // Unknown

		Entry("AllAlpha is supported in 1.17.0", "AllAlpha", "1.17.0", true, true),        // AddedInVersion: 1.17
		Entry("AllAlpha is not supported in 1.16.15", "AllAlpha", "1.16.15", false, true), // AddedInVersion: 1.17
	)

	Describe("FeatureGateVersionRange", func() {
		DescribeTable("#Contains",
			func(vr *FeatureGateVersionRange, version string, contains, success bool) {
				result, err := vr.Contains(version)
				if success {
					Expect(err).To(Not(HaveOccurred()))
					Expect(result).To(Equal(contains))
				} else {
					Expect(err).To(HaveOccurred())
				}
			},

			Entry("[,) contains 1.2.3", &FeatureGateVersionRange{}, "1.2.3", true, true),
			Entry("[,) contains 0.1.2", &FeatureGateVersionRange{}, "0.1.2", true, true),
			Entry("[,) contains 1.3.5", &FeatureGateVersionRange{}, "1.3.5", true, true),
			Entry("[,) fails with foo", &FeatureGateVersionRange{}, "foo", false, false),

			Entry("[, 1.3) contains 1.2.3", &FeatureGateVersionRange{RemovedInVersion: "1.3"}, "1.2.3", true, true),
			Entry("[, 1.3) contains 0.1.2", &FeatureGateVersionRange{RemovedInVersion: "1.3"}, "0.1.2", true, true),
			Entry("[, 1.3) doesn't contain 1.3.5", &FeatureGateVersionRange{RemovedInVersion: "1.3"}, "1.3.5", false, true),
			Entry("[, 1.3) fails with foo", &FeatureGateVersionRange{RemovedInVersion: "1.3"}, "foo", false, false),

			Entry("[1.0, ) contains 1.2.3", &FeatureGateVersionRange{AddedInVersion: "1.0"}, "1.2.3", true, true),
			Entry("[1.0, ) doesn't contain 0.1.2", &FeatureGateVersionRange{AddedInVersion: "1.0"}, "0.1.2", false, true),
			Entry("[1.0, ) contains 1.3.5", &FeatureGateVersionRange{AddedInVersion: "1.0"}, "1.3.5", true, true),
			Entry("[1.0, ) fails with foo", &FeatureGateVersionRange{AddedInVersion: "1.0"}, "foo", false, false),

			Entry("[1.0, 1.3) contains 1.2.3", &FeatureGateVersionRange{AddedInVersion: "1.0", RemovedInVersion: "1.3"}, "1.2.3", true, true),
			Entry("[1.0, 1.3) doesn't contain 0.1.2", &FeatureGateVersionRange{AddedInVersion: "1.0", RemovedInVersion: "1.3"}, "0.1.2", false, true),
			Entry("[1.0, 1.3) doesn't contain 1.3.5", &FeatureGateVersionRange{AddedInVersion: "1.0", RemovedInVersion: "1.3"}, "1.3.5", false, true),
			Entry("[1.0, 1.3) fails with foo", &FeatureGateVersionRange{AddedInVersion: "1.0", RemovedInVersion: "1.3"}, "foo", false, false),
		)
	})

	Describe("#ValidateFeatureGates", func() {
		DescribeTable("validate feature gates",
			func(featureGates map[string]bool, version string, matcher gomegatypes.GomegaMatcher) {
				errList := ValidateFeatureGates(featureGates, version, nil)
				Expect(errList).To(matcher)
			},

			Entry("empty list", nil, "1.18.14", BeEmpty()),
			Entry("supported feature gate", map[string]bool{"AnyVolumeDataSource": true}, "1.18.14", BeEmpty()),
			Entry("unsupported feature gate", map[string]bool{"CustomResourceValidation": true}, "1.18.14", ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeForbidden),
				"Field":  Equal(field.NewPath("CustomResourceValidation").String()),
				"Detail": Equal("not supported in Kubernetes version 1.18.14"),
			})))),
			Entry("unknown feature gate", map[string]bool{"Foo": true}, "1.18.14", ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":     Equal(field.ErrorTypeInvalid),
				"Field":    Equal(field.NewPath("Foo").String()),
				"BadValue": Equal("Foo"),
				"Detail":   Equal("unknown feature gate Foo"),
			})))),
			Entry("setting non-default value for locked feature gate", map[string]bool{"EndpointSlice": false}, "1.21.5", ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeForbidden),
				"Field":  Equal(field.NewPath("EndpointSlice").String()),
				"Detail": Equal("cannot set feature gate to false, feature is locked to true"),
			})))),
		)
	})
})

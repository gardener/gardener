// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package features_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	"k8s.io/apimachinery/pkg/util/validation/field"

	. "github.com/gardener/gardener/pkg/utils/validation/features"
)

var _ = Describe("vpaFeatureGates", func() {
	Describe("#ValidateVpaFeatureGates", func() {
		DescribeTable("validate vpa feature gates",
			func(featureGates map[string]bool, version string, matcher gomegatypes.GomegaMatcher) {
				errList := ValidateVpaFeatureGates(featureGates, version, nil)
				Expect(errList).To(matcher)
			},
			Entry("empty list", nil, "1", BeEmpty()),
			Entry("supported InPlaceOrRecreate", map[string]bool{"InPlaceOrRecreate": true}, "1.33", BeEmpty()),
			Entry("unsupported InPlaceOrRecreate", map[string]bool{"InPlaceOrRecreate": true}, "1.32", ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeForbidden),
				"Field":  Equal(field.NewPath("InPlaceOrRecreate").String()),
				"Detail": Equal("not supported in Kubernetes version 1.32"),
			})))),
			Entry("unknown feature gate", map[string]bool{"SomeFeatureGate": true}, "1.33", ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeInvalid),
				"Field":  Equal(field.NewPath("SomeFeatureGate").String()),
				"Detail": Equal("unknown feature gate"),
			})))),
		)
	})
})

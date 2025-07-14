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
			func(featureGates map[string]bool, matcher gomegatypes.GomegaMatcher) {
				errList := ValidateVpaFeatureGates(featureGates, nil)
				Expect(errList).To(matcher)
			},
			Entry("empty list", nil, BeEmpty()),
			Entry("supported feature gate", map[string]bool{"InPlaceOrRecreate": true}, BeEmpty()),
			Entry("unsupported feature gate", map[string]bool{"Foo": true}, ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeInvalid),
				"Field":  Equal(field.NewPath("Foo").String()),
				"Detail": Equal("unknown feature gate"),
			})))),
		)
	})
})

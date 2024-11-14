// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package features_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/component-base/featuregate"

	. "github.com/gardener/gardener/pkg/features"
)

var _ = Describe("Features", func() {
	Describe("#GetFeatures", func() {
		It("should return the spec for the given feature gate", func() {
			Expect(GetFeatures("DefaultSeccompProfile", "ShootForceDeletion", "Foo")).To(Equal(map[featuregate.Feature]featuregate.FeatureSpec{
				DefaultSeccompProfile: {Default: false, PreRelease: featuregate.Alpha},
				ShootForceDeletion:    {Default: true, PreRelease: featuregate.Beta},
			}))
		})
	})
})

// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Numbers", func() {
	Describe("#MinGreaterThanZero", func() {
		It("should return 0 if no value is greater than 0", func() {
			Expect(MinGreaterThanZero(-1, -1)).To(BeEquivalentTo(0))
			Expect(MinGreaterThanZero(-1, 0)).To(BeEquivalentTo(0))
			Expect(MinGreaterThanZero(0, -1)).To(BeEquivalentTo(0))
			Expect(MinGreaterThanZero(0, 0)).To(BeEquivalentTo(0))
		})

		It("should return the larger value if the other value is not greater than 0", func() {
			Expect(MinGreaterThanZero(0, 1)).To(BeEquivalentTo(1))
			Expect(MinGreaterThanZero(-1, 1)).To(BeEquivalentTo(1))
			Expect(MinGreaterThanZero(1, 0)).To(BeEquivalentTo(1))
			Expect(MinGreaterThanZero(1, -1)).To(BeEquivalentTo(1))
		})

		It("should return the smaller value if both values are greater than 0", func() {
			Expect(MinGreaterThanZero(1, 2)).To(BeEquivalentTo(1))
			Expect(MinGreaterThanZero(2, 1)).To(BeEquivalentTo(1))
		})

		It("should return the value if both are equal", func() {
			Expect(MinGreaterThanZero(0, 0)).To(BeEquivalentTo(0))
			Expect(MinGreaterThanZero(1, 1)).To(BeEquivalentTo(1))
		})
	})
})

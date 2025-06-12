// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package matchers

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("NotContainAny", func() {
	DescribeTable("MatchTable",
		func(actual []string, forbidden []string, shouldMatch bool) {
			match, err := NotContainAny(forbidden...).Match(actual)
			Expect(err).NotTo(HaveOccurred())
			Expect(match).To(Equal(shouldMatch), "Expected NotContainAny to match: %v, got: %v", shouldMatch, match)
		},
		Entry("should match when actual does not contain any forbidden elements", []string{"apple", "banana", "cherry"}, []string{"orange", "grape"}, true),
		Entry("should match when there are no forbidden elements", []string{"apple", "banana", "cherry"}, []string{}, true),
		Entry("should match when there are no actual elements", []string{}, []string{"orange", "grape"}, true),
		Entry("should match when actual and forbidden are empty", []string{}, []string{}, true),

		Entry("should not match when actual is a forbidden element", []string{"apple"}, []string{"apple"}, false),
		Entry("should not match when actual does contain a forbidden elements", []string{"apple", "banana", "cherry"}, []string{"apple"}, false),
		Entry("should not match when actual does contain any of forbidden elements", []string{"apple", "banana", "cherry"}, []string{"apple", "grape"}, false),
		Entry("should not match when actual does contain all of forbidden elements", []string{"apple", "banana", "cherry"}, []string{"apple", "banana"}, false),
	)

	Describe("Match", func() {
		It("should return an error if the actual value is not a slice of strings", func() {
			_, err := NotContainAny("apple").Match(42)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("NotContainAny expects a []string"))
		})
	})
})

// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package matchers

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = DescribeTable("#ContainAnyOf",
	func(actual []string, wanted []string, shouldMatch bool) {
		match, err := ContainAnyOf(wanted...).Match(actual)
		Expect(err).NotTo(HaveOccurred())
		Expect(match).To(Equal(shouldMatch), "Expected ContainAnyOf to match: %v, got: %v", shouldMatch, match)
	},
	Entry("should not match when actual does not contain any wanted elements", []string{"apple", "banana", "cherry"}, []string{"orange", "grape"}, false),
	Entry("should not match when there are no wanted elements", []string{"apple", "banana", "cherry"}, []string{}, false),
	Entry("should not match when there are no actual elements", []string{}, []string{"orange", "grape"}, false),
	Entry("should not match when actual and forbidden are empty", []string{}, []string{}, false),

	Entry("should match when actual is a wanted element", []string{"apple"}, []string{"apple"}, true),
	Entry("should match when actual does contain the wanted element", []string{"apple", "banana", "cherry"}, []string{"apple"}, true),
	Entry("should match when actual does contain any of wanted elements", []string{"apple", "banana", "cherry"}, []string{"apple", "grape"}, true),
	Entry("should match when actual does contain all of wanted elements", []string{"apple", "banana", "cherry"}, []string{"apple", "banana"}, true),
)

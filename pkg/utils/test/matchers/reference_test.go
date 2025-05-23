// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package matchers_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"

	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Reference Matcher", func() {
	test := func(actual, expected any) {
		It("should be true if objects share the same reference", func() {
			sameRef := actual

			Expect(actual).To(ShareSameReferenceAs(sameRef))
		})

		It("should be false if objects don't share the same reference", func() {
			Expect(actual).NotTo(ShareSameReferenceAs(expected))
		})
	}

	Context("when values are maps", func() {
		test(map[string]string{"foo": "bar"}, map[string]string{"foo": "bar"})
	})

	Context("when values are slices", func() {
		test([]string{"foo", "bar"}, []string{"foo", "bar"})
	})

	Context("when values are pointers", func() {
		test(ptr.To("foo"), ptr.To("foo"))
	})
})

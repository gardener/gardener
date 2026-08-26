// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package validation_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	"k8s.io/apimachinery/pkg/util/validation/field"

	. "github.com/gardener/gardener/pkg/utils/validation"
)

var _ = Describe("Validation", func() {
	DescribeTable("#ValidateFreeFormText",
		func(text string, matcher gomegatypes.GomegaMatcher) {
			Expect(ValidateFreeFormText(text, field.NewPath(""))).To(matcher)
		},
		Entry("should allow letters", "abcXYZ", BeEmpty()),
		Entry("should allow digits", "1234567890", BeEmpty()),
		Entry("should allow spaces", " \t\n", BeEmpty()),
		Entry("should allow slashes", "foo/bar\\baz", BeEmpty()),
		Entry("should allow punctuation", ".,:-_", BeEmpty()),
		Entry("should allow unicode letters", "こんにちはПриветمرحبا", BeEmpty()),
		Entry("should allow unicode digits", "٠١٢٣٤٥٦٧٨٩۰۱۲۳۴۵۶۷۸۹", BeEmpty()),
		Entry("should reject special characters", "{}<>*;", Not(BeEmpty())),
		Entry("should reject control characters", "Hello\x00World", Not(BeEmpty())),
		Entry("should reject emojis", "Hello😊World", Not(BeEmpty())),
		Entry("should reject symbols", "Hello©®™✓World", Not(BeEmpty())),
	)
})

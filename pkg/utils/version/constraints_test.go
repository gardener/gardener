// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package version_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"

	. "github.com/gardener/gardener/pkg/utils/version"
)

var _ = Describe("Constraints", func() {
	Describe("#CheckVersion", func() {
		DescribeTable("should check version strings correctly",
			func(constraintStr, version string, matcher gomegatypes.GomegaMatcher) {
				constraint := NewConstraint(constraintStr)
				Expect(constraint.CheckVersion(version)).To(matcher)
			},

			Entry("match: >= 1.0.0 with 1.0.0", ">= 1.0.0", "1.0.0", BeTrue()),
			Entry("match: >= 1.0.0 with 1.2.3", ">= 1.0.0", "1.2.3", BeTrue()),
			Entry("match: < 2.0.0 with 1.9.9", "< 2.0.0", "1.9.9", BeTrue()),
			Entry("match: = 1.2.3 with 1.2.3", "= 1.2.3", "1.2.3", BeTrue()),
			Entry("invalid: empty string", ">= 1.0.0", "", BeFalse()),
			Entry("invalid: not a version", ">= 1.0.0", "not-a-version", BeFalse()),
			Entry("invalid: text prefix", ">= 1.0.0", "version-1.2.3", BeFalse()),
			Entry("invalid: malformed", ">= 1.0.0", "1..2.3", BeFalse()),
		)
	})
})

// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubernetesversion_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"

	. "github.com/gardener/gardener/pkg/utils/validation/kubernetesversion"
)

var _ = Describe("Version", func() {
	DescribeTable("#CheckIfSupported",
		func(gitVersion string, matcher gomegatypes.GomegaMatcher) {
			Expect(CheckIfSupported(gitVersion)).To(matcher)
		},

		Entry("1.26", "1.26", MatchError(ContainSubstring("unsupported kubernetes version"))),
		Entry("1.27", "1.27", Succeed()),
		Entry("1.28", "1.28", Succeed()),
		Entry("1.29", "1.29", Succeed()),
		Entry("1.30", "1.30", Succeed()),
		Entry("1.31", "1.31", Succeed()),
		Entry("1.32", "1.32", Succeed()),
		Entry("1.33", "1.33", MatchError(ContainSubstring("unsupported kubernetes version"))),
	)
})

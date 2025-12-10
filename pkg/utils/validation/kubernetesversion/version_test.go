// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubernetesversion_test

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"

	. "github.com/gardener/gardener/pkg/utils/validation/kubernetesversion"
)

var _ = Describe("Version", func() {
	DescribeTable("#CheckIfSupported",
		func(gitVersion string, disableVersionCheck bool, matcher gomegatypes.GomegaMatcher) {
			env := "false"
			if disableVersionCheck {
				env = "true"
			}
			_ = os.Setenv("EXPERIMENTAL_DISABLE_KUBERNETES_VERSION_CHECK", env)
			Expect(CheckIfSupported(gitVersion)).To(matcher)
		},

		Entry("1.29", "1.29", false, MatchError(ContainSubstring("unsupported kubernetes version"))),
		Entry("1.30", "1.30", false, Succeed()),
		Entry("1.31", "1.31", false, Succeed()),
		Entry("1.32", "1.32", false, Succeed()),
		Entry("1.33", "1.33", false, Succeed()),
		Entry("1.34", "1.34", false, Succeed()),
		Entry("1.35", "1.35", false, MatchError(ContainSubstring("unsupported kubernetes version"))),

		// Disabling the version check by setting the env var EXPERIMENTAL_DISABLE_KUBERNETES_VERSION_CHECK to true
		Entry("1.23", "1.23", true, Succeed()), // too low
		Entry("2.34", "2.34", true, Succeed()), // too high
	)
})

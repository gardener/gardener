// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package version_test

import (
	"github.com/Masterminds/semver/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"

	. "github.com/gardener/gardener/pkg/utils/version"
)

var _ = Describe("Version", func() {
	DescribeTable("Constraints",
		func(constraint *semver.Constraints, version *semver.Version, matcher gomegatypes.GomegaMatcher) {
			Expect(constraint.Check(version)).To(matcher)
		},

		Entry("ConstraintK8sGreaterEqual128, success", ConstraintK8sGreaterEqual128, semver.MustParse("1.28.0"), BeTrue()),
		Entry("ConstraintK8sGreaterEqual128, failure", ConstraintK8sGreaterEqual128, semver.MustParse("1.27.0"), BeFalse()),
		Entry("ConstraintK8sGreaterEqual128, success w/ suffix", ConstraintK8sGreaterEqual128, semver.MustParse("v1.28.0-foo.12"), BeTrue()),
		Entry("ConstraintK8sGreaterEqual128, failure w/ suffix", ConstraintK8sGreaterEqual128, semver.MustParse("v1.27.0-foo.12"), BeFalse()),

		Entry("ConstraintK8sGreaterEqual129, success", ConstraintK8sGreaterEqual129, semver.MustParse("1.29.0"), BeTrue()),
		Entry("ConstraintK8sGreaterEqual129, failure", ConstraintK8sGreaterEqual129, semver.MustParse("1.28.0"), BeFalse()),
		Entry("ConstraintK8sGreaterEqual129, success w/ suffix", ConstraintK8sGreaterEqual129, semver.MustParse("v1.29.0-foo.12"), BeTrue()),
		Entry("ConstraintK8sGreaterEqual129, failure w/ suffix", ConstraintK8sGreaterEqual129, semver.MustParse("v1.28.0-foo.12"), BeFalse()),

		Entry("ConstraintK8sLess130, success", ConstraintK8sLess130, semver.MustParse("1.29.1"), BeTrue()),
		Entry("ConstraintK8sLess130, failure", ConstraintK8sLess130, semver.MustParse("1.30.0"), BeFalse()),
		Entry("ConstraintK8sLess130, success w/ suffix", ConstraintK8sLess130, semver.MustParse("v1.29.1-foo.12"), BeTrue()),
		Entry("ConstraintK8sLess130, failure w/ suffix", ConstraintK8sLess130, semver.MustParse("v1.30.0-foo.12"), BeFalse()),

		Entry("ConstraintK8sGreaterEqual130, success", ConstraintK8sGreaterEqual130, semver.MustParse("1.30.0"), BeTrue()),
		Entry("ConstraintK8sGreaterEqual130, failure", ConstraintK8sGreaterEqual130, semver.MustParse("1.29.0"), BeFalse()),
		Entry("ConstraintK8sGreaterEqual130, success w/ suffix", ConstraintK8sGreaterEqual130, semver.MustParse("v1.30.0-foo.12"), BeTrue()),
		Entry("ConstraintK8sGreaterEqual130, failure w/ suffix", ConstraintK8sGreaterEqual130, semver.MustParse("v1.29.0-foo.12"), BeFalse()),

		Entry("ConstraintK8sLess131, success", ConstraintK8sLess131, semver.MustParse("1.30.1"), BeTrue()),
		Entry("ConstraintK8sLess131, failure", ConstraintK8sLess131, semver.MustParse("1.31.0"), BeFalse()),
		Entry("ConstraintK8sLess131, success w/ suffix", ConstraintK8sLess131, semver.MustParse("v1.30.1-foo.12"), BeTrue()),
		Entry("ConstraintK8sLess131, failure w/ suffix", ConstraintK8sLess131, semver.MustParse("v1.31.0-foo.12"), BeFalse()),

		Entry("ConstraintK8sEqual131, success", ConstraintK8sEqual131, semver.MustParse("1.31.1"), BeTrue()),
		Entry("ConstraintK8sEqual131, failure", ConstraintK8sEqual131, semver.MustParse("1.30.0"), BeFalse()),
		Entry("ConstraintK8sEqual131, success w/ suffix", ConstraintK8sEqual131, semver.MustParse("v1.31.1-foo.12"), BeTrue()),
		Entry("ConstraintK8sEqual131, failure w/ suffix", ConstraintK8sEqual131, semver.MustParse("v1.30.0-foo.12"), BeFalse()),

		Entry("ConstraintK8sGreaterEqual131, success", ConstraintK8sGreaterEqual131, semver.MustParse("1.31.0"), BeTrue()),
		Entry("ConstraintK8sGreaterEqual131, failure", ConstraintK8sGreaterEqual131, semver.MustParse("1.30.0"), BeFalse()),
		Entry("ConstraintK8sGreaterEqual131, success w/ suffix", ConstraintK8sGreaterEqual131, semver.MustParse("v1.31.0-foo.12"), BeTrue()),
		Entry("ConstraintK8sGreaterEqual131, failure w/ suffix", ConstraintK8sGreaterEqual131, semver.MustParse("v1.30.0-foo.12"), BeFalse()),

		Entry("ConstraintK8sLess132, success", ConstraintK8sLess132, semver.MustParse("1.31.1"), BeTrue()),
		Entry("ConstraintK8sLess132, failure", ConstraintK8sLess132, semver.MustParse("1.32.0"), BeFalse()),
		Entry("ConstraintK8sLess132, success w/ suffix", ConstraintK8sLess132, semver.MustParse("v1.31.1-foo.12"), BeTrue()),
		Entry("ConstraintK8sLess132, failure w/ suffix", ConstraintK8sLess132, semver.MustParse("v1.32.0-foo.12"), BeFalse()),

		Entry("ConstraintK8sGreaterEqual132, success", ConstraintK8sGreaterEqual132, semver.MustParse("1.32.0"), BeTrue()),
		Entry("ConstraintK8sGreaterEqual132, failure", ConstraintK8sGreaterEqual132, semver.MustParse("1.31.0"), BeFalse()),
		Entry("ConstraintK8sGreaterEqual132, success w/ suffix", ConstraintK8sGreaterEqual132, semver.MustParse("v1.32.0-foo.12"), BeTrue()),
		Entry("ConstraintK8sGreaterEqual132, failure w/ suffix", ConstraintK8sGreaterEqual132, semver.MustParse("v1.31.0-foo.12"), BeFalse()),

		Entry("ConstraintK8sGreaterEqual133, success", ConstraintK8sGreaterEqual133, semver.MustParse("1.33.0"), BeTrue()),
		Entry("ConstraintK8sGreaterEqual133, failure", ConstraintK8sGreaterEqual133, semver.MustParse("1.32.0"), BeFalse()),
		Entry("ConstraintK8sGreaterEqual133, success w/ suffix", ConstraintK8sGreaterEqual133, semver.MustParse("v1.33.0-foo.12"), BeTrue()),
		Entry("ConstraintK8sGreaterEqual133, failure w/ suffix", ConstraintK8sGreaterEqual133, semver.MustParse("v1.32.0-foo.12"), BeFalse()),
	)

	DescribeTable("#CompareVersions",
		func(version1, operator, version2 string, expected gomegatypes.GomegaMatcher) {
			result, err := CompareVersions(version1, operator, version2)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(expected)
		},

		Entry("match", "1.2.3", ">", "1.2.2", BeTrue()),
		Entry("no match", "1.2.3", ">", "1.2.4", BeFalse()),
		Entry("match w/ suffix", "1.2.3-foo.12", ">", "v1.2.2-foo.23", BeTrue()),
		Entry("no match w/ suffix", "1.2.3-foo.12", ">", "v1.2.4-foo.34", BeFalse()),
	)

	DescribeTable("#CheckVersionMeetsConstraint",
		func(version, constraint string, expected gomegatypes.GomegaMatcher) {
			result, err := CheckVersionMeetsConstraint(version, constraint)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(expected)
		},

		Entry("match", "1.2.3", "> 1.2.2", BeTrue()),
		Entry("no match", "1.2.3", "> 1.2.4", BeFalse()),
		Entry("match w/ suffix", "1.2.3-foo.12", "> v1.2.2-foo.23", BeTrue()),
		Entry("no match w/ suffix", "1.2.3-foo.12", "> v1.2.4-foo.34", BeFalse()),
	)

	Describe("VersionRange", func() {
		DescribeTable("#Contains",
			func(vr VersionRange, version string, contains, success bool) {
				result, err := vr.Contains(version)
				if success {
					Expect(err).To(Not(HaveOccurred()))
					Expect(result).To(Equal(contains))
				} else {
					Expect(err).To(HaveOccurred())
				}
			},

			Entry("[,) contains 1.2.3", VersionRange{}, "1.2.3", true, true),
			Entry("[,) contains 0.1.2", VersionRange{}, "0.1.2", true, true),
			Entry("[,) contains 1.3.5", VersionRange{}, "1.3.5", true, true),
			Entry("[,) fails with foo", VersionRange{}, "foo", false, false),

			Entry("[, 1.3) contains 1.2.3", VersionRange{RemovedInVersion: "1.3"}, "1.2.3", true, true),
			Entry("[, 1.3) contains 0.1.2", VersionRange{RemovedInVersion: "1.3"}, "0.1.2", true, true),
			Entry("[, 1.3) doesn't contain 1.3.5", VersionRange{RemovedInVersion: "1.3"}, "1.3.5", false, true),
			Entry("[, 1.3) fails with foo", VersionRange{RemovedInVersion: "1.3"}, "foo", false, false),

			Entry("[1.0, ) contains 1.2.3", VersionRange{AddedInVersion: "1.0"}, "1.2.3", true, true),
			Entry("[1.0, ) doesn't contain 0.1.2", VersionRange{AddedInVersion: "1.0"}, "0.1.2", false, true),
			Entry("[1.0, ) contains 1.3.5", VersionRange{AddedInVersion: "1.0"}, "1.3.5", true, true),
			Entry("[1.0, ) fails with foo", VersionRange{AddedInVersion: "1.0"}, "foo", false, false),

			Entry("[1.0, 1.3) contains 1.2.3", VersionRange{AddedInVersion: "1.0", RemovedInVersion: "1.3"}, "1.2.3", true, true),
			Entry("[1.0, 1.3) doesn't contain 0.1.2", VersionRange{AddedInVersion: "1.0", RemovedInVersion: "1.3"}, "0.1.2", false, true),
			Entry("[1.0, 1.3) doesn't contain 1.3.5", VersionRange{AddedInVersion: "1.0", RemovedInVersion: "1.3"}, "1.3.5", false, true),
			Entry("[1.0, 1.3) fails with foo", VersionRange{AddedInVersion: "1.0", RemovedInVersion: "1.3"}, "foo", false, false),
		)

		DescribeTable("#SupportedVersions",
			func(vr VersionRange, expected string) {
				result := vr.SupportedVersionRange()
				Expect(result).To(Equal(expected))
			},

			Entry("No AddedInVersion", VersionRange{RemovedInVersion: "1.1.0"}, "versions < 1.1.0"),
			Entry("No RemovedInVersion", VersionRange{AddedInVersion: "1.1.0"}, "versions >= 1.1.0"),
			Entry("No AddedInVersion amnd RemovedInVersion", VersionRange{}, "all kubernetes versions"),
			Entry("AddedInVersion amnd RemovedInVersion", VersionRange{AddedInVersion: "1.1.0", RemovedInVersion: "1.2.0"}, "versions >= 1.1.0, < 1.2.0"),
		)
	})
})

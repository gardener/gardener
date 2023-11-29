// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
		Entry("ConstraintK8sGreaterEqual125, success", ConstraintK8sGreaterEqual125, semver.MustParse("1.25.0"), BeTrue()),
		Entry("ConstraintK8sGreaterEqual125, failure", ConstraintK8sGreaterEqual125, semver.MustParse("1.24.0"), BeFalse()),
		Entry("ConstraintK8sGreaterEqual125, success w/ suffix", ConstraintK8sGreaterEqual125, semver.MustParse("v1.25.0-foo.12"), BeTrue()),
		Entry("ConstraintK8sGreaterEqual125, failure w/ suffix", ConstraintK8sGreaterEqual125, semver.MustParse("v1.24.0-foo.12"), BeFalse()),

		Entry("ConstraintK8sLess125, success", ConstraintK8sLess125, semver.MustParse("1.24.1"), BeTrue()),
		Entry("ConstraintK8sLess125, failure", ConstraintK8sLess125, semver.MustParse("1.25.0"), BeFalse()),
		Entry("ConstraintK8sLess125, success w/ suffix", ConstraintK8sLess125, semver.MustParse("v1.24.1-foo.12"), BeTrue()),
		Entry("ConstraintK8sLess125, failure w/ suffix", ConstraintK8sLess125, semver.MustParse("v1.25.0-foo.12"), BeFalse()),

		Entry("ConstraintK8sGreaterEqual126, success", ConstraintK8sGreaterEqual126, semver.MustParse("1.26.0"), BeTrue()),
		Entry("ConstraintK8sGreaterEqual126, failure", ConstraintK8sGreaterEqual126, semver.MustParse("1.25.0"), BeFalse()),
		Entry("ConstraintK8sGreaterEqual126, success w/ suffix", ConstraintK8sGreaterEqual126, semver.MustParse("v1.26.0-foo.12"), BeTrue()),
		Entry("ConstraintK8sGreaterEqual126, failure w/ suffix", ConstraintK8sGreaterEqual126, semver.MustParse("v1.25.0-foo.12"), BeFalse()),

		Entry("ConstraintK8sLess126, success", ConstraintK8sLess126, semver.MustParse("1.25.1"), BeTrue()),
		Entry("ConstraintK8sLess126, failure", ConstraintK8sLess126, semver.MustParse("1.26.0"), BeFalse()),
		Entry("ConstraintK8sLess126, success w/ suffix", ConstraintK8sLess126, semver.MustParse("v1.25.1-foo.12"), BeTrue()),
		Entry("ConstraintK8sLess126, failure w/ suffix", ConstraintK8sLess126, semver.MustParse("v1.26.0-foo.12"), BeFalse()),

		Entry("ConstraintK8sGreaterEqual127, success", ConstraintK8sGreaterEqual127, semver.MustParse("1.27.0"), BeTrue()),
		Entry("ConstraintK8sGreaterEqual127, failure", ConstraintK8sGreaterEqual127, semver.MustParse("1.26.0"), BeFalse()),
		Entry("ConstraintK8sGreaterEqual127, success w/ suffix", ConstraintK8sGreaterEqual127, semver.MustParse("v1.27.0-foo.12"), BeTrue()),
		Entry("ConstraintK8sGreaterEqual127, failure w/ suffix", ConstraintK8sGreaterEqual127, semver.MustParse("v1.26.0-foo.12"), BeFalse()),

		Entry("ConstraintK8sLess127, success", ConstraintK8sLess127, semver.MustParse("1.26.1"), BeTrue()),
		Entry("ConstraintK8sLess127, failure", ConstraintK8sLess127, semver.MustParse("1.27.0"), BeFalse()),
		Entry("ConstraintK8sLess127, success w/ suffix", ConstraintK8sLess127, semver.MustParse("v1.26.1-foo.12"), BeTrue()),
		Entry("ConstraintK8sLess127, failure w/ suffix", ConstraintK8sLess127, semver.MustParse("v1.27.0-foo.12"), BeFalse()),

		Entry("ConstraintK8sGreaterEqual128, success", ConstraintK8sGreaterEqual128, semver.MustParse("1.28.0"), BeTrue()),
		Entry("ConstraintK8sGreaterEqual128, failure", ConstraintK8sGreaterEqual128, semver.MustParse("1.27.0"), BeFalse()),
		Entry("ConstraintK8sGreaterEqual128, success w/ suffix", ConstraintK8sGreaterEqual128, semver.MustParse("v1.28.0-foo.12"), BeTrue()),
		Entry("ConstraintK8sGreaterEqual128, failure w/ suffix", ConstraintK8sGreaterEqual128, semver.MustParse("v1.27.0-foo.12"), BeFalse()),
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

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
	"github.com/Masterminds/semver"
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

		Entry("ConstraintK8sEqual120, success", ConstraintK8sEqual120, semver.MustParse("1.20.1"), BeTrue()),
		Entry("ConstraintK8sEqual120, failure", ConstraintK8sEqual120, semver.MustParse("1.19.0"), BeFalse()),
		Entry("ConstraintK8sEqual120, success w/ suffix", ConstraintK8sEqual120, semver.MustParse("v1.20.1-foo.12"), BeTrue()),
		Entry("ConstraintK8sEqual120, failure w/ suffix", ConstraintK8sEqual120, semver.MustParse("v1.19.0-foo.12"), BeFalse()),

		Entry("ConstraintK8sGreaterEqual120, success", ConstraintK8sGreaterEqual120, semver.MustParse("1.20.0"), BeTrue()),
		Entry("ConstraintK8sGreaterEqual120, failure", ConstraintK8sGreaterEqual120, semver.MustParse("1.19.0"), BeFalse()),
		Entry("ConstraintK8sGreaterEqual120, success w/ suffix", ConstraintK8sGreaterEqual120, semver.MustParse("v1.20.0-foo.12"), BeTrue()),
		Entry("ConstraintK8sGreaterEqual120, failure w/ suffix", ConstraintK8sGreaterEqual120, semver.MustParse("v1.19.0-foo.12"), BeFalse()),

		Entry("ConstraintK8sLessEqual121, success", ConstraintK8sLessEqual121, semver.MustParse("1.20.0"), BeTrue()),
		Entry("ConstraintK8sLessEqual121, failure", ConstraintK8sLessEqual121, semver.MustParse("1.22.0"), BeFalse()),
		Entry("ConstraintK8sLessEqual121, success w/ suffix", ConstraintK8sLessEqual121, semver.MustParse("v1.20.0-foo.12"), BeTrue()),
		Entry("ConstraintK8sLessEqual121, failure w/ suffix", ConstraintK8sLessEqual121, semver.MustParse("v1.22.0-foo.12"), BeFalse()),

		Entry("ConstraintK8sEqual121, success", ConstraintK8sEqual121, semver.MustParse("1.21.1"), BeTrue()),
		Entry("ConstraintK8sEqual121, failure", ConstraintK8sEqual121, semver.MustParse("1.20.0"), BeFalse()),
		Entry("ConstraintK8sEqual121, success w/ suffix", ConstraintK8sEqual121, semver.MustParse("v1.21.1-foo.12"), BeTrue()),
		Entry("ConstraintK8sEqual121, failure w/ suffix", ConstraintK8sEqual121, semver.MustParse("v1.20.0-foo.12"), BeFalse()),

		Entry("ConstraintK8sGreaterEqual121, success", ConstraintK8sGreaterEqual121, semver.MustParse("1.21.0"), BeTrue()),
		Entry("ConstraintK8sGreaterEqual121, failure", ConstraintK8sGreaterEqual121, semver.MustParse("1.20.0"), BeFalse()),
		Entry("ConstraintK8sGreaterEqual121, success w/ suffix", ConstraintK8sGreaterEqual121, semver.MustParse("v1.21.0-foo.12"), BeTrue()),
		Entry("ConstraintK8sGreaterEqual121, failure w/ suffix", ConstraintK8sGreaterEqual121, semver.MustParse("1.20.0-foo.12"), BeFalse()),

		Entry("ConstraintK8sLessEqual122, success", ConstraintK8sLessEqual122, semver.MustParse("1.21.0"), BeTrue()),
		Entry("ConstraintK8sLessEqual122, failure", ConstraintK8sLessEqual122, semver.MustParse("1.23.0"), BeFalse()),
		Entry("ConstraintK8sLessEqual122, success w/ suffix", ConstraintK8sLessEqual122, semver.MustParse("v1.21.0-foo.12"), BeTrue()),
		Entry("ConstraintK8sLessEqual122, failure w/ suffix", ConstraintK8sLessEqual122, semver.MustParse("v1.23.0-foo.12"), BeFalse()),

		Entry("ConstraintK8sEqual122, success", ConstraintK8sEqual122, semver.MustParse("1.22.1"), BeTrue()),
		Entry("ConstraintK8sEqual122, failure", ConstraintK8sEqual122, semver.MustParse("1.21.0"), BeFalse()),
		Entry("ConstraintK8sEqual122, success w/ suffix", ConstraintK8sEqual122, semver.MustParse("v1.22.1-foo.12"), BeTrue()),
		Entry("ConstraintK8sEqual122, failure w/ suffix", ConstraintK8sEqual122, semver.MustParse("v1.21.0-foo.12"), BeFalse()),

		Entry("ConstraintK8sGreaterEqual122, success", ConstraintK8sGreaterEqual122, semver.MustParse("1.22.0"), BeTrue()),
		Entry("ConstraintK8sGreaterEqual122, failure", ConstraintK8sGreaterEqual122, semver.MustParse("1.21.0"), BeFalse()),
		Entry("ConstraintK8sGreaterEqual122, success w/ suffix", ConstraintK8sGreaterEqual122, semver.MustParse("v1.22.0-foo.12"), BeTrue()),
		Entry("ConstraintK8sGreaterEqual122, failure w/ suffix", ConstraintK8sGreaterEqual122, semver.MustParse("v1.21.0-foo.12"), BeFalse()),

		Entry("ConstraintK8sEqual123, success", ConstraintK8sEqual123, semver.MustParse("1.23.1"), BeTrue()),
		Entry("ConstraintK8sEqual123, failure", ConstraintK8sEqual123, semver.MustParse("1.22.0"), BeFalse()),
		Entry("ConstraintK8sEqual123, success w/ suffix", ConstraintK8sEqual123, semver.MustParse("1.23.1-foo.12"), BeTrue()),
		Entry("ConstraintK8sEqual123, failure w/ suffix", ConstraintK8sEqual123, semver.MustParse("1.22.0-foo.12"), BeFalse()),

		Entry("ConstraintK8sGreaterEqual123, success", ConstraintK8sGreaterEqual123, semver.MustParse("1.23.0"), BeTrue()),
		Entry("ConstraintK8sGreaterEqual123, failure", ConstraintK8sGreaterEqual123, semver.MustParse("1.22.0"), BeFalse()),
		Entry("ConstraintK8sGreaterEqual123, success w/ suffix", ConstraintK8sGreaterEqual123, semver.MustParse("v1.23.0-foo.12"), BeTrue()),
		Entry("ConstraintK8sGreaterEqual123, failure w/ suffix", ConstraintK8sGreaterEqual123, semver.MustParse("v1.22.0-foo.12"), BeFalse()),

		Entry("ConstraintK8sEqual124, success", ConstraintK8sEqual124, semver.MustParse("1.24.1"), BeTrue()),
		Entry("ConstraintK8sEqual124, failure", ConstraintK8sEqual124, semver.MustParse("1.23.0"), BeFalse()),
		Entry("ConstraintK8sEqual124, success w/ suffix", ConstraintK8sEqual124, semver.MustParse("v1.24.1-foo.12"), BeTrue()),
		Entry("ConstraintK8sEqual124, failure w/ suffix", ConstraintK8sEqual124, semver.MustParse("v1.23.0-foo.12"), BeFalse()),

		Entry("ConstraintK8sLess124, success", ConstraintK8sLess124, semver.MustParse("1.23.1"), BeTrue()),
		Entry("ConstraintK8sLess124, failure", ConstraintK8sLess124, semver.MustParse("1.24.0"), BeFalse()),
		Entry("ConstraintK8sLess124, success w/ suffix", ConstraintK8sLess124, semver.MustParse("v1.23.1-foo.12"), BeTrue()),
		Entry("ConstraintK8sLess124, failure w/ suffix", ConstraintK8sLess124, semver.MustParse("v1.24.0-foo.12"), BeFalse()),

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
})

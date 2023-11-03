// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kubernetes_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/gardener/gardener/pkg/utils/kubernetes"
)

var _ = Describe("controllers", func() {
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
	})
})

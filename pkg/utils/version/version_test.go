// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/gardener/gardener/pkg/utils/version"
)

var _ = Describe("Version", func() {
	DescribeTable("#Normalize",
		func(input, output string) {
			Expect(Normalize(input)).To(Equal(output))
		},

		Entry("version w/ 'v'-prefix w/o suffixes", "v1.2.3", "1.2.3"),
		Entry("version w/ 'v'-prefix w suffixes starting with '-'", "v1.2.3-foo.bar", "1.2.3"),
		Entry("version w/ 'v'-prefix w suffixes starting with '+'", "v1.2.3+foo.bar", "1.2.3"),
		Entry("version w/o 'v'-prefix w/o suffixes", "1.2.3", "1.2.3"),
		Entry("version w/o 'v'-prefix w suffixes starting with '-'", "1.2.3-foo.bar", "1.2.3"),
		Entry("version w/o 'v'-prefix w suffixes starting with '+'", "1.2.3+foo.bar", "1.2.3"),
	)
})

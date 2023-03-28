// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

		Entry("1.19", "1.19", MatchError(ContainSubstring("unsupported kubernetes version"))),
		Entry("1.20", "1.20", Succeed()),
		Entry("1.21", "1.21", Succeed()),
		Entry("1.22", "1.22", Succeed()),
		Entry("1.23", "1.23", Succeed()),
		Entry("1.24", "1.24", Succeed()),
		Entry("1.25", "1.25", Succeed()),
		Entry("1.26", "1.26", Succeed()),
		Entry("1.27", "1.27", MatchError(ContainSubstring("unsupported kubernetes version"))),
	)
})

// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package utils_test

import (
	. "github.com/gardener/gardener/pkg/operation/seed/utils"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("utils", func() {

	DescribeTable("#ComputeGardenNamespace",
		func(name, expected string) {
			actual := ComputeGardenNamespace(name)
			Expect(actual).To(Equal(expected))
		},
		Entry("empty name", "", "seed-"),
		Entry("garden", "garden", "seed-garden"),
		Entry("dash", "-", "seed--"),
		Entry("garden prefixed with dash", "-garden", "seed--garden"),
	)

	DescribeTable("#ComputeSeedName",
		func(name, expected string, expectErr bool) {
			actual, err := ComputeSeedName(name)
			if expectErr {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(actual).To(Equal(expected))
			}
		},
		Entry("expect error with empty name", "", "", true),
		Entry("expect error with garden name", "garden", "", true),
		Entry("expect error with dash", "-", "", true),
		Entry("expect success with empty name", "seed-", "", false),
		Entry("expect success with dash name", "seed--", "-", false),
		Entry("expect success with duplicated prefix", "seed-seed-", "seed-", false),
		Entry("expect success with duplicated prefix", "seed-seed-a", "seed-a", false),
		Entry("expect success with garden name", "seed-garden", "garden", false),
	)
})

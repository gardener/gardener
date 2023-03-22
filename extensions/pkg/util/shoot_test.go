// Copyright 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package util_test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/version"

	"github.com/gardener/gardener/extensions/pkg/util"
)

var _ = Describe("Shoot", func() {
	Describe("#VersionMajorMinor", func() {
		It("should return an error due to an invalid version format", func() {
			v, err := util.VersionMajorMinor("invalid-semver")

			Expect(v).To(BeEmpty())
			Expect(err).To(HaveOccurred())
		})

		It("should return the major/minor part of the given version", func() {
			var (
				major = 14
				minor = 123

				expectedVersion = fmt.Sprintf("%d.%d", major, minor)
			)

			v, err := util.VersionMajorMinor(fmt.Sprintf("%s.88", expectedVersion))

			Expect(v).To(Equal(expectedVersion))
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("#VersionInfo", func() {
		It("should return an error due to an invalid version format", func() {
			v, err := util.VersionInfo("invalid-semver")

			Expect(v).To(BeNil())
			Expect(err).To(HaveOccurred())
		})

		It("should convert the given version to a correct version.Info", func() {
			var (
				expectedVersionInfo = &version.Info{
					Major:      "14",
					Minor:      "123",
					GitVersion: "v14.123.42",
				}
			)

			v, err := util.VersionInfo("14.123.42")

			Expect(v).To(Equal(expectedVersionInfo))
			Expect(err).NotTo(HaveOccurred())
		})
	})
})

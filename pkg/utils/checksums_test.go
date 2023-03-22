// Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/gardener/gardener/pkg/utils"
)

var _ = Describe("Checksums", func() {
	Describe("#ComputeSecretChecksum", func() {
		It("should compute the correct checksum", func() {
			checksum := ComputeSecretChecksum(map[string][]byte{"foo": []byte("bar")})
			Expect(checksum).To(Equal("bd142ccf5968384068077c58de4d3ad833204a151d3e9f1182703f07b69125b8"))
			Expect(checksum).To(HaveLen(64))
		})
	})

	Describe("#ComputeConfigMapChecksum", func() {
		It("should compute the correct checksum", func() {
			checksum := ComputeConfigMapChecksum(map[string]string{"foo": "bar"})
			Expect(checksum).To(Equal("bd142ccf5968384068077c58de4d3ad833204a151d3e9f1182703f07b69125b8"))
			Expect(checksum).To(HaveLen(64))
		})
	})

	Describe("#ComputeChecksum", func() {
		It("should compute the correct checksum", func() {
			checksum := ComputeChecksum("foo")
			Expect(checksum).To(Equal("b2213295d564916f89a6a42455567c87c3f480fcd7a1c15e220f17d7169a790b"))
			Expect(checksum).To(HaveLen(64))
		})
	})
})

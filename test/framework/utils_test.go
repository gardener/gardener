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

package framework_test

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/test/framework"
)

var _ = Describe("Utils tests", func() {

	It("should not fail if a path exists", func() {
		tmpdir, err := os.MkdirTemp("", "e2e-")
		Expect(err).ToNot(HaveOccurred())
		defer os.RemoveAll(tmpdir)
		framework.FileExists(tmpdir)
	})

	Context("string set", func() {
		It("should succeed if a string is set", func() {
			Expect(framework.StringSet("test")).To(BeTrue())
		})
		It("should fail if a string is empty", func() {
			Expect(framework.StringSet("")).To(BeFalse())
		})
	})

	It("should parse shoot from file", func() {
		shoot := &gardencorev1beta1.Shoot{}
		err := framework.ReadObject("./testdata/test-shoot.yaml", shoot)
		Expect(err).ToNot(HaveOccurred())

		Expect(shoot.Name).To(Equal("test"))
		Expect(shoot.Namespace).To(Equal("ns"))
	})

})

// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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

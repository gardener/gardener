// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package gardener_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/gardener/gardener/pkg/utils/gardener"
)

var _ = Describe("ServiceAccount", func() {
	Describe("#ExtensionShootServiceAccountName", func() {
		It("should return the correct name", func() {
			Expect(ExtensionShootServiceAccountName("myshoot", "my-ci")).To(Equal("extension-shoot--myshoot--my-ci"))
		})
	})

	Describe("#ParseExtensionShootServiceAccountName", func() {
		It("should parse a valid name", func() {
			shootName, ok := ParseExtensionShootServiceAccountName("extension-shoot--myshoot--my-ci")
			Expect(ok).To(BeTrue())
			Expect(shootName).To(Equal("myshoot"))
		})

		It("should return false for a name without the prefix", func() {
			_, ok := ParseExtensionShootServiceAccountName("some-other-sa")
			Expect(ok).To(BeFalse())
		})

		It("should return false for a name with the prefix but no second --", func() {
			_, ok := ParseExtensionShootServiceAccountName("extension-shoot--malformed")
			Expect(ok).To(BeFalse())
		})

		It("should return false for a name with empty shoot name", func() {
			_, ok := ParseExtensionShootServiceAccountName("extension-shoot----my-ci")
			Expect(ok).To(BeFalse())
		})
	})
})

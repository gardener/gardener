// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/gardener/gardener/pkg/apis/operator/v1alpha1/helper"
)

var _ = Describe("Extension", func() {
	Describe("#ExtensionRuntimeManagedResourceName", func() {
		It("should return the expected managed resource name", func() {
			Expect(ExtensionRuntimeManagedResourceName("provider-test")).To(Equal("extension-provider-test-garden"))
		})
	})

	Describe("#ExtensionRuntimeNamespaceName", func() {
		It("should return the expected namespace name", func() {
			Expect(ExtensionRuntimeNamespaceName("provider-test")).To(Equal("runtime-extension-provider-test"))
		})
	})
})

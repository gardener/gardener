// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package core_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
)

var _ = Describe("SecretBinding", func() {
	DescribeTable("#GetProviderType",
		func(secretBinding *gardencore.SecretBinding, expected string) {
			actual := secretBinding.GetProviderType()
			Expect(actual).To(Equal(expected))
		},
		Entry("when provider is nil", &gardencore.SecretBinding{Provider: nil}, ""),
		Entry("when provider is set", &gardencore.SecretBinding{Provider: &gardencore.SecretBindingProvider{Type: "foo"}}, "foo"),
	)
})

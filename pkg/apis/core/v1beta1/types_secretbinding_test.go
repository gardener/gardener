// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package v1beta1_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

var _ = Describe("SecretBinding", func() {
	DescribeTable("#GetProviderType",
		func(secretBinding *gardencorev1beta1.SecretBinding, expected string) {
			actual := secretBinding.GetProviderType()
			Expect(actual).To(Equal(expected))
		},
		Entry("when provider is nil", &gardencorev1beta1.SecretBinding{Provider: nil}, ""),
		Entry("when provider is set", &gardencorev1beta1.SecretBinding{Provider: &gardencorev1beta1.SecretBindingProvider{Type: "foo"}}, "foo"),
	)
})

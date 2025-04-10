// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
)

var _ = Describe("Helper", func() {
	DescribeTable("#SecretBindingHasType",
		func(secretBinding *gardencorev1beta1.SecretBinding, toFind string, expected bool) {
			actual := SecretBindingHasType(secretBinding, toFind)
			Expect(actual).To(Equal(expected))
		},

		Entry("with empty provider field", &gardencorev1beta1.SecretBinding{}, "foo", false),
		Entry("when single-value provider type equals to the given type", &gardencorev1beta1.SecretBinding{Provider: &gardencorev1beta1.SecretBindingProvider{Type: "foo"}}, "foo", true),
		Entry("when single-value provider type does not match the given type", &gardencorev1beta1.SecretBinding{Provider: &gardencorev1beta1.SecretBindingProvider{Type: "foo"}}, "bar", false),
		Entry("when multi-value provider type contains the given type", &gardencorev1beta1.SecretBinding{Provider: &gardencorev1beta1.SecretBindingProvider{Type: "foo,bar"}}, "bar", true),
		Entry("when multi-value provider type does not contain the given type", &gardencorev1beta1.SecretBinding{Provider: &gardencorev1beta1.SecretBindingProvider{Type: "foo,bar"}}, "baz", false),
	)

	DescribeTable("#AddTypeToSecretBinding",
		func(secretBinding *gardencorev1beta1.SecretBinding, toAdd, expected string) {
			AddTypeToSecretBinding(secretBinding, toAdd)
			Expect(secretBinding.Provider.Type).To(Equal(expected))
		},

		Entry("with empty provider field", &gardencorev1beta1.SecretBinding{}, "foo", "foo"),
		Entry("when single-value provider type already exists", &gardencorev1beta1.SecretBinding{Provider: &gardencorev1beta1.SecretBindingProvider{Type: "foo"}}, "foo", "foo"),
		Entry("when single-value provider type does not exist", &gardencorev1beta1.SecretBinding{Provider: &gardencorev1beta1.SecretBindingProvider{Type: "foo"}}, "bar", "foo,bar"),
		Entry("when multi-value provider type already exists", &gardencorev1beta1.SecretBinding{Provider: &gardencorev1beta1.SecretBindingProvider{Type: "foo,bar"}}, "foo", "foo,bar"),
		Entry("when multi-value provider type does not exist", &gardencorev1beta1.SecretBinding{Provider: &gardencorev1beta1.SecretBindingProvider{Type: "foo,bar"}}, "baz", "foo,bar,baz"),
	)

	DescribeTable("#GetSecretBindingTypes",
		func(secretBinding *gardencorev1beta1.SecretBinding, expected []string) {
			actual := GetSecretBindingTypes(secretBinding)
			Expect(actual).To(Equal(expected))
		},

		Entry("with nil provider type", &gardencorev1beta1.SecretBinding{Provider: nil}, []string{}),
		Entry("with single-value provider type", &gardencorev1beta1.SecretBinding{Provider: &gardencorev1beta1.SecretBindingProvider{Type: "foo"}}, []string{"foo"}),
		Entry("with multi-value provider type", &gardencorev1beta1.SecretBinding{Provider: &gardencorev1beta1.SecretBindingProvider{Type: "foo,bar,baz"}}, []string{"foo", "bar", "baz"}),
	)
})

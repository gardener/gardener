// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package matchers_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Deep", func() {
	var actual, expected *corev1.Pod

	BeforeEach(func() {
		actual = &corev1.Pod{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Pod",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "foo",
			},
		}
		expected = &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: "foo",
			},
		}
	})

	Describe("#DeepDerivativeEqual", func() {
		It("should be true when expected has less info", func() {
			Expect(actual).To(DeepDerivativeEqual(expected))
		})

		It("should be false when objects differ", func() {
			expected.Name = "baz"
			Expect(actual).ToNot(DeepDerivativeEqual(expected))
		})

		It("should throw error when both are nil", func() {
			success, err := DeepDerivativeEqual(nil).Match(nil)

			Expect(success).Should(BeFalse())
			Expect(err).Should(HaveOccurred())
		})
	})

	Describe("#DeepEqual", func() {
		It("should be true when expected is equal", func() {
			expected.TypeMeta = actual.TypeMeta
			Expect(actual).To(DeepEqual(expected))
		})

		It("should be false when expected has less info", func() {
			Expect(actual).NotTo(DeepEqual(expected))
		})

		It("should be false when objects differ", func() {
			expected.Name = "baz"
			Expect(actual).ToNot(DeepEqual(expected))
		})

		It("should throw error when both are nil", func() {
			success, err := DeepEqual(nil).Match(nil)

			Expect(success).Should(BeFalse())
			Expect(err).Should(HaveOccurred())
		})
	})
})

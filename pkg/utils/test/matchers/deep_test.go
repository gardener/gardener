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

// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package test_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/utils/test"
)

var _ = Describe("gomock Matchers", func() {
	Describe("#HasObjectKeyOf", func() {
		var (
			matcher  gomock.Matcher
			expected client.Object
		)

		BeforeEach(func() {
			expected = &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "bar",
				},
			}
			matcher = test.HasObjectKeyOf(expected)
		})

		Describe("#Matches", func() {
			It("return true for same key", func() {
				Expect(matcher.Matches(expected.DeepCopyObject())).To(BeTrue())
			})
			It("return false for different key", func() {
				otherObject := expected.DeepCopyObject().(client.Object)
				otherObject.SetName("other")
				Expect(matcher.Matches(otherObject)).To(BeFalse())
			})
			It("return false for non-objects", func() {
				notAnObject := corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "foo",
						Namespace: "bar",
					},
				}
				Expect(matcher.Matches(notAnObject)).To(BeFalse())
			})
		})

		Describe("#String", func() {
			It("should return key in description", func() {
				Expect(matcher.String()).To(ContainSubstring(client.ObjectKeyFromObject(expected).String()))
			})
		})
	})
})

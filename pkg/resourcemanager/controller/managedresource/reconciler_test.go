// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package managedresource

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var _ = Describe("Controller", func() {
	Describe("#injectLabels", func() {
		var (
			obj, expected *unstructured.Unstructured
			labels        map[string]string
		)

		BeforeEach(func() {
			obj = &unstructured.Unstructured{Object: map[string]any{}}
			expected = obj.DeepCopy()
		})

		It("do nothing as labels is nil", func() {
			labels = nil
			Expect(injectLabels(obj, labels)).To(Succeed())
			Expect(obj).To(Equal(expected))
		})

		It("do nothing as labels is empty", func() {
			labels = map[string]string{}
			Expect(injectLabels(obj, labels)).To(Succeed())
			Expect(obj).To(Equal(expected))
		})

		It("should correctly inject labels into the object's metadata", func() {
			labels = map[string]string{
				"inject": "me",
			}
			expected.SetLabels(labels)

			Expect(injectLabels(obj, labels)).To(Succeed())
			Expect(obj).To(Equal(expected))
		})

		It("should correctly inject labels into the object's pod template's metadata", func() {
			labels = map[string]string{
				"inject": "me",
			}

			// add .spec.template to object
			Expect(unstructured.SetNestedMap(obj.Object, map[string]any{
				"template": map[string]any{},
			}, "spec")).To(Succeed())

			expected = obj.DeepCopy()
			Expect(unstructured.SetNestedMap(expected.Object, map[string]any{
				"inject": "me",
			}, "spec", "template", "metadata", "labels")).To(Succeed())
			expected.SetLabels(labels)

			Expect(injectLabels(obj, labels)).To(Succeed())
			Expect(obj).To(Equal(expected))
		})

		It("should correctly inject labels into the object's volumeClaimTemplates' metadata", func() {
			labels = map[string]string{
				"inject": "me",
			}

			// add .spec.volumeClaimTemplates to object
			Expect(unstructured.SetNestedMap(obj.Object, map[string]any{
				"volumeClaimTemplates": []any{
					map[string]any{
						"metadata": map[string]any{
							"name": "volume-claim-name",
						},
					},
				},
			}, "spec")).To(Succeed())

			expected = obj.DeepCopy()
			Expect(unstructured.SetNestedMap(expected.Object, map[string]any{
				"volumeClaimTemplates": []any{
					map[string]any{
						"metadata": map[string]any{
							"name": "volume-claim-name",
							"labels": map[string]any{
								"inject": "me",
							},
						},
					},
				},
			}, "spec")).To(Succeed())
			expected.SetLabels(labels)

			Expect(injectLabels(obj, labels)).To(Succeed())
			Expect(obj).To(Equal(expected))
		})
	})
})

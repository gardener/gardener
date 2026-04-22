// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package managedresource

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
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

	Describe("#deletionPropagation", func() {
		var obj *unstructured.Unstructured

		BeforeEach(func() {
			obj = &unstructured.Unstructured{Object: map[string]any{}}
		})

		It("should return an empty string when the deletion propagation is not set", func() {
			Expect(deletionPropagation(obj)).To(BeEmpty())
		})

		It("should return an error when the deletion propagation is invalid", func() {
			Expect(unstructured.SetNestedField(obj.Object, "invalid", "metadata", "annotations", resourcesv1alpha1.DeletionPropagationOnInvalidUpdate)).To(Succeed())

			Expect(deletionPropagation(obj)).Error().To(HaveOccurred())
		})

		It("should return the correct deletion propagation value when it is set", func() {
			Expect(unstructured.SetNestedField(obj.Object, string(metav1.DeletePropagationOrphan), "metadata", "annotations", resourcesv1alpha1.DeletionPropagationOnInvalidUpdate)).To(Succeed())

			Expect(deletionPropagation(obj)).To(Equal(metav1.DeletePropagationOrphan))
		})
	})
})

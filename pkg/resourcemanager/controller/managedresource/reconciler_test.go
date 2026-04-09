// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package managedresource

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"

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

	Describe("#deleteOnInvalidUpdate", func() {
		var obj *unstructured.Unstructured

		BeforeEach(func() {
			obj = &unstructured.Unstructured{Object: map[string]any{}}
		})

		It("should return false for a generic error with no immutable cause", func() {
			err := apierrors.NewBadRequest("something went wrong")
			Expect(deleteOnInvalidUpdate(obj, err)).To(BeFalse())
		})

		It("should return true for an Invalid cause with 'field is immutable'", func() {
			err := apierrors.NewInvalid(
				schema.GroupKind{Group: "apps", Kind: "Deployment"},
				"metrics-server",
				field.ErrorList{field.Invalid(field.NewPath("spec", "selector"), nil, "field is immutable")},
			)
			Expect(deleteOnInvalidUpdate(obj, err)).To(BeTrue())
		})

		It("should return true for a Forbidden cause with 'field is immutable'", func() {
			err := apierrors.NewInvalid(
				schema.GroupKind{Group: "", Kind: "Secret"},
				"test-secret",
				field.ErrorList{field.Forbidden(field.NewPath("data"), "field is immutable when `immutable` is set")},
			)
			Expect(deleteOnInvalidUpdate(obj, err)).To(BeTrue())
		})

		It("should return false for an Invalid cause with an unrelated message", func() {
			err := apierrors.NewInvalid(
				schema.GroupKind{Group: "apps", Kind: "Deployment"},
				"test-deploy",
				field.ErrorList{field.Invalid(field.NewPath("spec", "replicas"), nil, "must be greater than zero")},
			)
			Expect(deleteOnInvalidUpdate(obj, err)).To(BeFalse())
		})

		It("should return true when the delete-on-invalid-update annotation is set, regardless of error", func() {
			obj.SetAnnotations(map[string]string{resourcesv1alpha1.DeleteOnInvalidUpdate: "true"})
			err := apierrors.NewBadRequest("something went wrong")
			Expect(deleteOnInvalidUpdate(obj, err)).To(BeTrue())
		})
	})
})

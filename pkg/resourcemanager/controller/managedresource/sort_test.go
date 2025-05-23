// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package managedresource

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
)

var _ = Describe("Sorter", func() {
	Describe("Reference sorter", func() {
		var refs, refsBase []resourcesv1alpha1.ObjectReference

		BeforeEach(func() {
			refsBase = []resourcesv1alpha1.ObjectReference{
				{
					ObjectReference: corev1.ObjectReference{
						APIVersion: "v1",
						Kind:       "ConfigMap",
						Name:       "foo",
						Namespace:  "bar",
					},
				},
				{
					ObjectReference: corev1.ObjectReference{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
						Name:       "nginx",
						Namespace:  "web",
					},
				},
			}

			// copy refs for assertions, as referenceSorter is sorting in-place
			refs = append(refsBase[:0:0], refsBase...)
		})

		Describe("#sortObjectReferences", func() {
			It("should correctly sort refs", func() {
				sortObjectReferences(refs)
				Expect(refs).To(Equal(refsBase))
			})
			It("should correctly sort refs (inverted order)", func() {
				refs[0], refs[1] = refs[1], refs[0]
				sortObjectReferences(refs)
				Expect(refs).To(Equal(refsBase))
			})
		})

		Describe("#newReferenceSorter", func() {
			var sorter referenceSorter

			BeforeEach(func() {
				sorter = newReferenceSorter(refs).(referenceSorter)
			})

			It("should return the correct length", func() {
				Expect(sorter.Len()).To(BeEquivalentTo(len(refsBase)))
			})

			It("should return the correct length (nil slice)", func() {
				sorter = newReferenceSorter(nil).(referenceSorter)
				Expect(sorter.Len()).To(BeEquivalentTo(0))
			})

			It("should calculate the correct keys for refs", func() {
				Expect(refs).To(Equal([]resourcesv1alpha1.ObjectReference{
					refsBase[0],
					refsBase[1],
				}))
				Expect(sorter.keys).To(Equal([]string{
					"/ConfigMap/bar/foo",
					"apps/Deployment/web/nginx",
				}))
			})

			It("should correctly compare refs", func() {
				Expect(sorter.Less(0, 1)).To(BeTrue())
			})

			It("should correctly swap refs and keys", func() {
				sorter.Swap(0, 1)
				Expect(refs).To(Equal([]resourcesv1alpha1.ObjectReference{
					refsBase[1],
					refsBase[0],
				}))
				Expect(sorter.keys).To(Equal([]string{
					"apps/Deployment/web/nginx",
					"/ConfigMap/bar/foo",
				}))
			})
		})
	})

	Describe("Kind sorter", func() {
		var obj, objBase []object

		Context("object's kind is different, sort base on kind", func() {
			BeforeEach(func() {
				objBase = []object{
					{
						obj: &unstructured.Unstructured{
							Object: map[string]any{
								"apiVersion": "v1",
								"kind":       "ConfigMap",
								"metadata": map[string]any{
									"name":      "foo",
									"namespace": "bar",
								},
							},
						},
					},
					{
						obj: &unstructured.Unstructured{
							Object: map[string]any{
								"apiVersion": "apps/v1",
								"kind":       "Deployment",
								"metadata": map[string]any{
									"name":      "nginx",
									"namespace": "web",
								},
							},
						},
					},
				}

				// copy refs for assertions, as kindSorter is sorting in-place
				obj = append(obj[:0:0], objBase...)
			})

			Describe("#sortObjectReferences", func() {
				It("should correctly sort refs", func() {
					sortByKind(obj)
					Expect(obj).To(Equal(objBase))
				})
				It("should correctly sort refs (inverted order)", func() {
					obj[0], obj[1] = obj[1], obj[0]
					sortByKind(obj)
					Expect(obj).To(Equal(objBase))
				})
			})
		})

		Context("object's kind is same, sort base on name", func() {
			BeforeEach(func() {
				objBase = []object{
					{
						obj: &unstructured.Unstructured{
							Object: map[string]any{
								"apiVersion": "v1",
								"kind":       "ConfigMap",
								"metadata": map[string]any{
									"name":      "foo",
									"namespace": "bar",
								},
							},
						},
					},
					{
						obj: &unstructured.Unstructured{
							Object: map[string]any{
								"apiVersion": "v1",
								"kind":       "ConfigMap",
								"metadata": map[string]any{
									"name":      "foo1",
									"namespace": "bar",
								},
							},
						},
					},
				}

				// copy refs for assertions, as kindSorter is sorting in-place
				obj = append(obj[:0:0], objBase...)
			})

			Describe("#sortObjectReferences", func() {
				It("should correctly sort refs", func() {
					sortByKind(obj)
					Expect(obj).To(Equal(objBase))
				})
				It("should correctly sort refs (inverted order)", func() {
					obj[0], obj[1] = obj[1], obj[0]
					sortByKind(obj)
					Expect(obj).To(Equal(objBase))
				})
			})
		})
	})
})

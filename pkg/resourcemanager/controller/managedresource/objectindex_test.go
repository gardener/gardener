// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package managedresource

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
)

var _ = Describe("objectIndex", func() {
	Describe("#NewObjectIndex, #Lookup", func() {
		It("without equivalences", func() {
			oldRef := v1alpha1.ObjectReference{
				ObjectReference: corev1.ObjectReference{Name: "name", Namespace: "ns", Kind: "kindA", APIVersion: "groupA/v2"},
			}

			unusedRef := v1alpha1.ObjectReference{
				ObjectReference: corev1.ObjectReference{Name: "foo", Namespace: "bar", Kind: "kind", APIVersion: "group/v1"},
			}

			existingRefs := []v1alpha1.ObjectReference{
				oldRef,
				unusedRef,
			}

			index := NewObjectIndex(existingRefs, nil)

			newRef := v1alpha1.ObjectReference{
				ObjectReference: corev1.ObjectReference{Name: "name", Namespace: "ns", Kind: "kindB", APIVersion: "groupB/v1"},
			}

			_, found := index.Lookup(newRef)
			Expect(found).To(BeFalse())

			foundRef, found := index.Lookup(oldRef)
			Expect(found).To(BeTrue())
			Expect(foundRef).To(Equal(oldRef))

			Expect(index.Found(oldRef)).To(BeTrue())
			Expect(index.Found(unusedRef)).To(BeFalse())
		})

		It("with default equivalences", func() {
			oldRef := v1alpha1.ObjectReference{
				ObjectReference: corev1.ObjectReference{Name: "name", Namespace: "ns", Kind: "Deployment", APIVersion: "extensions/v1beta1"},
			}

			unusedRef := v1alpha1.ObjectReference{
				ObjectReference: corev1.ObjectReference{Name: "foo", Namespace: "bar", Kind: "kind", APIVersion: "group/v1"},
			}

			existingRefs := []v1alpha1.ObjectReference{
				oldRef,
				unusedRef,
			}

			index := NewObjectIndex(existingRefs, NewEquivalences())

			newRef := v1alpha1.ObjectReference{
				ObjectReference: corev1.ObjectReference{Name: "name", Namespace: "ns", Kind: "Deployment", APIVersion: "apps/v1"},
			}

			foundRef, found := index.Lookup(newRef)
			Expect(found).To(BeTrue())
			Expect(foundRef).To(Equal(oldRef))

			Expect(index.Found(oldRef)).To(BeTrue())
			Expect(index.Found(unusedRef)).To(BeFalse())
		})

		It("with equivalences", func() {
			equis := [][]metav1.GroupKind{
				{
					{Group: "groupA", Kind: "kindA"},
					{Group: "groupB", Kind: "kindB"},
				},
			}

			oldRef := v1alpha1.ObjectReference{
				ObjectReference: corev1.ObjectReference{Name: "name", Namespace: "ns", Kind: "kindA", APIVersion: "groupA/v2"},
			}

			unusedRef := v1alpha1.ObjectReference{
				ObjectReference: corev1.ObjectReference{Name: "foo", Namespace: "bar", Kind: "kind", APIVersion: "group/v1"},
			}
			existingRefs := []v1alpha1.ObjectReference{
				oldRef,
				unusedRef,
			}

			index := NewObjectIndex(existingRefs, NewEquivalences(equis...))

			newRef := v1alpha1.ObjectReference{
				ObjectReference: corev1.ObjectReference{Name: "name", Namespace: "ns", Kind: "kindB", APIVersion: "groupB/v1"},
			}

			foundRef, found := index.Lookup(newRef)
			Expect(found).To(BeTrue())
			Expect(foundRef).To(Equal(oldRef))

			Expect(index.Found(oldRef)).To(BeTrue())
			Expect(index.Found(unusedRef)).To(BeFalse())
		})
	})
})

// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package extensions_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/gardener/gardener/pkg/extensions"
)

var _ = Describe("Add", func() {
	var infrastructure *extensionsv1alpha1.Infrastructure

	BeforeEach(func() {
		infrastructure = &extensionsv1alpha1.Infrastructure{
			Spec: extensionsv1alpha1.InfrastructureSpec{
				DefaultSpec: extensionsv1alpha1.DefaultSpec{
					Type: "type",
				},
			},
		}
	})

	Describe("#ObjectPredicate", func() {
		var p predicate.Predicate

		BeforeEach(func() {
			p = ObjectPredicate()
		})

		Describe("#Create", func() {
			It("should return true", func() {
				Expect(p.Create(event.CreateEvent{})).To(BeTrue())
			})
		})

		Describe("#Update", func() {
			It("should return true for periodic cache resyncs", func() {
				Expect(p.Update(event.UpdateEvent{ObjectNew: infrastructure, ObjectOld: infrastructure.DeepCopy()})).To(BeTrue())
			})

			It("should return false because object is no extensions object", func() {
				Expect(p.Update(event.UpdateEvent{ObjectOld: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{ResourceVersion: "123"}}, ObjectNew: infrastructure})).To(BeFalse())
			})

			It("should return false because old object is no extensions object", func() {
				Expect(p.Update(event.UpdateEvent{ObjectOld: infrastructure, ObjectNew: &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{ResourceVersion: "123"}}})).To(BeFalse())
			})

			It("should return false because extensions type did not change", func() {
				oldInfrastructure := infrastructure.DeepCopy()
				infrastructure.ResourceVersion = "1"
				Expect(p.Update(event.UpdateEvent{ObjectNew: infrastructure, ObjectOld: oldInfrastructure})).To(BeFalse())
			})

			It("should return true because extension type was changed", func() {
				oldInfrastructure := infrastructure.DeepCopy()
				infrastructure.Spec.Type = "foo"
				Expect(p.Update(event.UpdateEvent{ObjectNew: infrastructure, ObjectOld: oldInfrastructure})).To(BeTrue())
			})
		})

		Describe("#Delete", func() {
			It("should return true", func() {
				Expect(p.Delete(event.DeleteEvent{})).To(BeTrue())
			})
		})

		Describe("#Generic", func() {
			It("should return false", func() {
				Expect(p.Generic(event.GenericEvent{})).To(BeFalse())
			})
		})
	})
})

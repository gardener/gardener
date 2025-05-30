//  SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
//  SPDX-License-Identifier: Apache-2.0

package predicate_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	. "github.com/gardener/gardener/pkg/operator/predicate"
)

var _ = Describe("Extension", func() {
	var p predicate.Predicate

	Describe("#ExtensionRequirementsChanged", func() {
		BeforeEach(func() {
			p = ExtensionRequirementsChanged()
		})

		Describe("#Create", func() {
			It("should return true", func() {
				Expect(p.Create(event.CreateEvent{})).To(BeTrue())
			})
		})

		Describe("#Update", func() {
			var (
				extensionOld, extensionNew *operatorv1alpha1.Extension
				requiredCondition          gardencorev1beta1.Condition
			)

			BeforeEach(func() {
				extensionOld = &operatorv1alpha1.Extension{}
				extensionNew = &operatorv1alpha1.Extension{}

				requiredCondition = gardencorev1beta1.Condition{
					Type: "RequiredRuntime",
				}
			})

			It("should false if condition status is unchanged", func() {
				extensionOld.Status.Conditions = []gardencorev1beta1.Condition{requiredCondition}
				extensionNew.Status.Conditions = []gardencorev1beta1.Condition{requiredCondition}

				Expect(p.Update(event.UpdateEvent{
					ObjectOld: extensionOld,
					ObjectNew: extensionNew,
				})).To(BeFalse())
			})

			It("should false if condition is not available", func() {
				fooCondition := gardencorev1beta1.Condition{Type: "foo"}

				extensionOld.Status.Conditions = []gardencorev1beta1.Condition{}
				extensionNew.Status.Conditions = []gardencorev1beta1.Condition{fooCondition}

				Expect(p.Update(event.UpdateEvent{
					ObjectOld: extensionOld,
					ObjectNew: extensionNew,
				})).To(BeFalse())
			})

			It("should true if condition status is changed", func() {
				requiredConditionTrue := *(requiredCondition.DeepCopy())
				requiredConditionTrue.Status = "True"

				extensionOld.Status.Conditions = []gardencorev1beta1.Condition{requiredCondition}
				extensionNew.Status.Conditions = []gardencorev1beta1.Condition{requiredConditionTrue}

				Expect(p.Update(event.UpdateEvent{
					ObjectOld: extensionOld,
					ObjectNew: extensionNew,
				})).To(BeTrue())
			})

			It("should true if condition was added", func() {
				extensionOld.Status.Conditions = []gardencorev1beta1.Condition{}
				extensionNew.Status.Conditions = []gardencorev1beta1.Condition{requiredCondition}

				Expect(p.Update(event.UpdateEvent{
					ObjectOld: extensionOld,
					ObjectNew: extensionNew,
				})).To(BeTrue())
			})

			It("should true if condition was removed", func() {
				extensionOld.Status.Conditions = []gardencorev1beta1.Condition{requiredCondition}
				extensionNew.Status.Conditions = []gardencorev1beta1.Condition{}

				Expect(p.Update(event.UpdateEvent{
					ObjectOld: extensionOld,
					ObjectNew: extensionNew,
				})).To(BeTrue())
			})
		})

		Describe("#Delete", func() {
			It("should return false", func() {
				Expect(p.Delete(event.DeleteEvent{})).To(BeFalse())
			})
		})

		Describe("#Generic", func() {
			It("should return false", func() {
				Expect(p.Generic(event.GenericEvent{})).To(BeFalse())
			})
		})
	})
})

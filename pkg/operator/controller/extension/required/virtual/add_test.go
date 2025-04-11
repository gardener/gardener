// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package virtual_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	. "github.com/gardener/gardener/pkg/operator/controller/extension/required/virtual"
)

var _ = Describe("Add", func() {
	Describe("#RequiredConditionChangedPredicate", func() {
		var (
			predicate              predicate.Predicate
			controllerInstallation *gardencorev1beta1.ControllerInstallation

			test = func(objectOld, objectNew client.Object, result bool) {
				ExpectWithOffset(1, predicate.Create(event.CreateEvent{Object: objectNew})).To(BeTrue())
				ExpectWithOffset(1, predicate.Update(event.UpdateEvent{ObjectOld: objectOld, ObjectNew: objectNew})).To(Equal(result))
				ExpectWithOffset(1, predicate.Delete(event.DeleteEvent{Object: objectNew})).To(BeTrue())
				ExpectWithOffset(1, predicate.Generic(event.GenericEvent{Object: objectNew})).To(BeTrue())
			}
		)

		BeforeEach(func() {
			predicate = (&Reconciler{}).RequiredConditionChangedPredicate()
			controllerInstallation = &gardencorev1beta1.ControllerInstallation{
				Status: gardencorev1beta1.ControllerInstallationStatus{
					Conditions: []gardencorev1beta1.Condition{
						{Type: "Test", Status: gardencorev1beta1.ConditionFalse},
						{Type: "Required", Status: gardencorev1beta1.ConditionFalse},
					},
				},
			}
		})

		It("should return true if required condition changed", func() {
			controllerInstallationOld := controllerInstallation.DeepCopy()
			controllerInstallationOld.Status.Conditions = v1beta1helper.MergeConditions(controllerInstallationOld.Status.Conditions, gardencorev1beta1.Condition{Type: "Required", Status: gardencorev1beta1.ConditionTrue})

			test(controllerInstallationOld, controllerInstallation, true)
		})

		It("should return true if condition was added with status 'True'", func() {
			controllerInstallationOld := controllerInstallation.DeepCopy()
			controllerInstallationOld.Status.Conditions = v1beta1helper.RemoveConditions(controllerInstallationOld.Status.Conditions, "Required")
			controllerInstallation.Status.Conditions = v1beta1helper.MergeConditions(controllerInstallationOld.Status.Conditions, gardencorev1beta1.Condition{Type: "Required", Status: gardencorev1beta1.ConditionTrue})

			test(controllerInstallationOld, controllerInstallation, true)
		})

		It("should return false if required condition is not available", func() {
			controllerInstallation.Status.Conditions = v1beta1helper.RemoveConditions(controllerInstallation.Status.Conditions, "Required")

			test(controllerInstallation.DeepCopy(), controllerInstallation, false)
		})

		It("should return false if required condition is unchanged", func() {
			test(controllerInstallation, controllerInstallation, false)
		})

		It("should return false if condition was added with status 'False'", func() {
			controllerInstallationOld := controllerInstallation.DeepCopy()
			controllerInstallationOld.Status.Conditions = v1beta1helper.RemoveConditions(controllerInstallationOld.Status.Conditions, "Required")
			controllerInstallation.Status.Conditions = v1beta1helper.MergeConditions(controllerInstallationOld.Status.Conditions, gardencorev1beta1.Condition{Type: "Required", Status: gardencorev1beta1.ConditionFalse})

			test(controllerInstallationOld, controllerInstallation, false)
		})
	})
})

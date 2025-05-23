// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package predicate_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"

	. "github.com/gardener/gardener/pkg/resourcemanager/predicate"
)

var _ = Describe("Predicate", func() {
	Describe("#HasOperationAnnotation", func() {
		var (
			createEvent  event.CreateEvent
			updateEvent  event.UpdateEvent
			deleteEvent  event.DeleteEvent
			genericEvent event.GenericEvent
		)

		BeforeEach(func() {
			configMap := &corev1.ConfigMap{}

			createEvent = event.CreateEvent{
				Object: configMap,
			}
			updateEvent = event.UpdateEvent{
				ObjectOld: configMap,
				ObjectNew: configMap,
			}
			deleteEvent = event.DeleteEvent{
				Object: configMap,
			}
			genericEvent = event.GenericEvent{
				Object: configMap,
			}
		})

		DescribeTable("it should match",
			func(value string) {
				annotations := map[string]string{"gardener.cloud/operation": value}
				createEvent.Object.SetAnnotations(annotations)
				updateEvent.ObjectOld.SetAnnotations(annotations)
				updateEvent.ObjectNew.SetAnnotations(annotations)
				deleteEvent.Object.SetAnnotations(annotations)
				genericEvent.Object.SetAnnotations(annotations)

				predicate := HasOperationAnnotation()

				Expect(predicate.Create(createEvent)).To(BeTrue())
				Expect(predicate.Update(updateEvent)).To(BeTrue())
				Expect(predicate.Delete(deleteEvent)).To(BeTrue())
				Expect(predicate.Generic(genericEvent)).To(BeTrue())
			},

			Entry("reconcile", "reconcile"),
			Entry("migrate", "migrate"),
			Entry("restore", "restore"),
		)

		It("should not match", func() {
			predicate := HasOperationAnnotation()

			Expect(predicate.Create(createEvent)).To(BeFalse())
			Expect(predicate.Update(updateEvent)).To(BeFalse())
			Expect(predicate.Delete(deleteEvent)).To(BeTrue())
			Expect(predicate.Generic(genericEvent)).To(BeFalse())
		})
	})
})

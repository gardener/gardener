// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package csimigration_test

import (
	. "github.com/gardener/gardener/extensions/pkg/controller/csimigration"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

var _ = Describe("predicate", func() {
	Describe("#ClusterCSIMigrationControllerNotFinished", func() {
		It("should return true because controller not finished", func() {
			var (
				predicate                                           = ClusterCSIMigrationControllerNotFinished()
				createEvent, updateEvent, deleteEvent, genericEvent = computeEvents(false)
			)

			Expect(predicate.Create(createEvent)).To(BeTrue())
			Expect(predicate.Update(updateEvent)).To(BeTrue())
			Expect(predicate.Delete(deleteEvent)).To(BeTrue())
			Expect(predicate.Generic(genericEvent)).To(BeTrue())
		})

		It("should return false because controller is finished", func() {
			var (
				predicate                                           = ClusterCSIMigrationControllerNotFinished()
				createEvent, updateEvent, deleteEvent, genericEvent = computeEvents(true)
			)

			Expect(predicate.Create(createEvent)).To(BeFalse())
			Expect(predicate.Update(updateEvent)).To(BeFalse())
			Expect(predicate.Delete(deleteEvent)).To(BeFalse())
			Expect(predicate.Generic(genericEvent)).To(BeFalse())
		})
	})
})

func computeEvents(controllerFinished bool) (event.CreateEvent, event.UpdateEvent, event.DeleteEvent, event.GenericEvent) {
	cluster := &extensionsv1alpha1.Cluster{}

	if controllerFinished {
		cluster.ObjectMeta.Annotations = map[string]string{AnnotationKeyControllerFinished: "true"}
	}

	return event.CreateEvent{Object: cluster},
		event.UpdateEvent{ObjectOld: cluster, ObjectNew: cluster},
		event.DeleteEvent{Object: cluster},
		event.GenericEvent{Object: cluster}
}

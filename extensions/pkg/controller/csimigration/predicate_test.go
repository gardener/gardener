// Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package csimigration_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/event"

	. "github.com/gardener/gardener/extensions/pkg/controller/csimigration"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
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

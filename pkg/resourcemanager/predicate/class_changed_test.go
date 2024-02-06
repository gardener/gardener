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

package predicate_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/event"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	resourcemanagerpredicate "github.com/gardener/gardener/pkg/resourcemanager/predicate"
)

var _ = Describe("Predicate", func() {
	var (
		managedResource *resourcesv1alpha1.ManagedResource
		createEvent     event.CreateEvent
		updateEvent     event.UpdateEvent
		deleteEvent     event.DeleteEvent
		genericEvent    event.GenericEvent
	)

	BeforeEach(func() {
		managedResource = &resourcesv1alpha1.ManagedResource{
			Spec: resourcesv1alpha1.ManagedResourceSpec{
				Class: ptr.To("shoot"),
			},
		}
		createEvent = event.CreateEvent{
			Object: managedResource,
		}
		updateEvent = event.UpdateEvent{
			ObjectOld: managedResource,
			ObjectNew: managedResource,
		}
		deleteEvent = event.DeleteEvent{
			Object: managedResource,
		}
		genericEvent = event.GenericEvent{
			Object: managedResource,
		}
	})

	Describe("#ClassChangedPredicate", func() {
		It("should not match on update (no change)", func() {
			predicate := resourcemanagerpredicate.ClassChangedPredicate()

			Expect(predicate.Create(createEvent)).To(BeTrue())
			Expect(predicate.Update(updateEvent)).To(BeFalse())
			Expect(predicate.Delete(deleteEvent)).To(BeTrue())
			Expect(predicate.Generic(genericEvent)).To(BeTrue())
		})

		It("should not match on update (old not set)", func() {
			updateEvent.ObjectOld = nil

			predicate := resourcemanagerpredicate.ClassChangedPredicate()

			Expect(predicate.Create(createEvent)).To(BeTrue())
			Expect(predicate.Update(updateEvent)).To(BeFalse())
			Expect(predicate.Delete(deleteEvent)).To(BeTrue())
			Expect(predicate.Generic(genericEvent)).To(BeTrue())
		})

		It("should not match on update (old is not a ManagedResource)", func() {
			updateEvent.ObjectOld = &corev1.Pod{}

			predicate := resourcemanagerpredicate.ClassChangedPredicate()

			Expect(predicate.Create(createEvent)).To(BeTrue())
			Expect(predicate.Update(updateEvent)).To(BeFalse())
			Expect(predicate.Delete(deleteEvent)).To(BeTrue())
			Expect(predicate.Generic(genericEvent)).To(BeTrue())
		})

		It("should not match on update (new not set)", func() {
			updateEvent.ObjectNew = nil

			predicate := resourcemanagerpredicate.ClassChangedPredicate()

			Expect(predicate.Create(createEvent)).To(BeTrue())
			Expect(predicate.Update(updateEvent)).To(BeFalse())
			Expect(predicate.Delete(deleteEvent)).To(BeTrue())
			Expect(predicate.Generic(genericEvent)).To(BeTrue())
		})

		It("should not match on update (new is not a ManagedResource)", func() {
			updateEvent.ObjectNew = &corev1.Pod{}

			predicate := resourcemanagerpredicate.ClassChangedPredicate()

			Expect(predicate.Create(createEvent)).To(BeTrue())
			Expect(predicate.Update(updateEvent)).To(BeFalse())
			Expect(predicate.Delete(deleteEvent)).To(BeTrue())
			Expect(predicate.Generic(genericEvent)).To(BeTrue())
		})

		It("should match on update (class changed)", func() {
			managedResourceNew := managedResource.DeepCopy()
			managedResourceNew.Spec.Class = ptr.To("other")
			updateEvent.ObjectNew = managedResourceNew

			predicate := resourcemanagerpredicate.ClassChangedPredicate()

			Expect(predicate.Create(createEvent)).To(BeTrue())
			Expect(predicate.Update(updateEvent)).To(BeTrue())
			Expect(predicate.Delete(deleteEvent)).To(BeTrue())
			Expect(predicate.Generic(genericEvent)).To(BeTrue())
		})
	})
})

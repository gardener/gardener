// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	. "github.com/gardener/gardener/pkg/resourcemanager/predicate"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

var _ = Describe("#ConditionStatusChanged", func() {
	var (
		managedResource *resourcesv1alpha1.ManagedResource
		createEvent     event.CreateEvent
		updateEvent     event.UpdateEvent
		deleteEvent     event.DeleteEvent
		genericEvent    event.GenericEvent
		conditionType   resourcesv1alpha1.ConditionType
	)

	BeforeEach(func() {
		managedResource = &resourcesv1alpha1.ManagedResource{
			Spec: resourcesv1alpha1.ManagedResourceSpec{
				Class: pointer.StringPtr("shoot"),
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

		conditionType = resourcesv1alpha1.ResourcesApplied
	})

	It("should not match on update (no change)", func() {
		predicate := ConditionStatusChanged(conditionType, DefaultConditionChange)

		Expect(predicate.Create(createEvent)).To(BeTrue())
		Expect(predicate.Update(updateEvent)).To(BeFalse())
		Expect(predicate.Delete(deleteEvent)).To(BeTrue())
		Expect(predicate.Generic(genericEvent)).To(BeTrue())
	})

	It("should not match on update (old not set)", func() {
		updateEvent.ObjectOld = nil

		predicate := ConditionStatusChanged(conditionType, DefaultConditionChange)

		Expect(predicate.Create(createEvent)).To(BeTrue())
		Expect(predicate.Update(updateEvent)).To(BeFalse())
		Expect(predicate.Delete(deleteEvent)).To(BeTrue())
		Expect(predicate.Generic(genericEvent)).To(BeTrue())
	})

	It("should not match on update (old is not a ManagedResource)", func() {
		updateEvent.ObjectOld = &corev1.Pod{}

		predicate := ConditionStatusChanged(conditionType, DefaultConditionChange)

		Expect(predicate.Create(createEvent)).To(BeTrue())
		Expect(predicate.Update(updateEvent)).To(BeFalse())
		Expect(predicate.Delete(deleteEvent)).To(BeTrue())
		Expect(predicate.Generic(genericEvent)).To(BeTrue())
	})

	It("should not match on update (new not set)", func() {
		updateEvent.ObjectNew = nil

		predicate := ConditionStatusChanged(conditionType, DefaultConditionChange)

		Expect(predicate.Create(createEvent)).To(BeTrue())
		Expect(predicate.Update(updateEvent)).To(BeFalse())
		Expect(predicate.Delete(deleteEvent)).To(BeTrue())
		Expect(predicate.Generic(genericEvent)).To(BeTrue())
	})

	It("should not match on update (new is not a ManagedResource)", func() {
		updateEvent.ObjectNew = &corev1.Pod{}

		predicate := ConditionStatusChanged(conditionType, DefaultConditionChange)

		Expect(predicate.Create(createEvent)).To(BeTrue())
		Expect(predicate.Update(updateEvent)).To(BeFalse())
		Expect(predicate.Delete(deleteEvent)).To(BeTrue())
		Expect(predicate.Generic(genericEvent)).To(BeTrue())
	})

	DescribeTable("DefaultConditionChange",
		func(old, new *resourcesv1alpha1.ManagedResourceCondition, matcher types.GomegaMatcher) {
			managedResourceNew := managedResource.DeepCopy()
			if old != nil {
				managedResource.Status.Conditions = []resourcesv1alpha1.ManagedResourceCondition{*old}
			}
			if new != nil {
				managedResourceNew.Status.Conditions = []resourcesv1alpha1.ManagedResourceCondition{*new}
			}
			updateEvent.ObjectOld = managedResource
			updateEvent.ObjectNew = managedResourceNew

			predicate := ConditionStatusChanged(conditionType, DefaultConditionChange)

			Expect(predicate.Create(createEvent)).To(BeTrue())
			Expect(predicate.Update(updateEvent)).To(matcher)
			Expect(predicate.Delete(deleteEvent)).To(BeTrue())
			Expect(predicate.Generic(genericEvent)).To(BeTrue())
		},
		Entry("should match on update (condition added)",
			nil,
			condition(resourcesv1alpha1.ConditionTrue),
			BeTrue(),
		),
		Entry("should match on update (condition removed)",
			condition(resourcesv1alpha1.ConditionTrue),
			nil,
			BeTrue(),
		),
		Entry("should match on update (condition status changed)",
			condition(resourcesv1alpha1.ConditionProgressing),
			condition(resourcesv1alpha1.ConditionTrue),
			BeTrue(),
		),
	)

	DescribeTable("ConditionChangedToUnhealthy",
		func(old, new *resourcesv1alpha1.ManagedResourceCondition, matcher types.GomegaMatcher) {
			managedResourceNew := managedResource.DeepCopy()
			if old != nil {
				managedResource.Status.Conditions = []resourcesv1alpha1.ManagedResourceCondition{*old}
			}
			if new != nil {
				managedResourceNew.Status.Conditions = []resourcesv1alpha1.ManagedResourceCondition{*new}
			}
			updateEvent.ObjectNew = managedResourceNew

			predicate := ConditionStatusChanged(conditionType, ConditionChangedToUnhealthy)

			Expect(predicate.Create(createEvent)).To(BeTrue())
			Expect(predicate.Update(updateEvent)).To(matcher)
			Expect(predicate.Delete(deleteEvent)).To(BeTrue())
			Expect(predicate.Generic(genericEvent)).To(BeTrue())
		},
		Entry("should not match on update (condition added)",
			nil,
			condition(resourcesv1alpha1.ConditionTrue),
			BeFalse(),
		),
		Entry("should not match on update (status changed to true)",
			condition(resourcesv1alpha1.ConditionFalse),
			condition(resourcesv1alpha1.ConditionTrue),
			BeFalse(),
		),
		Entry("should not match on update (no status change)",
			condition(resourcesv1alpha1.ConditionTrue),
			condition(resourcesv1alpha1.ConditionTrue),
			BeFalse(),
		),
		Entry("should match on update (condition added)",
			nil,
			condition(resourcesv1alpha1.ConditionFalse),
			BeTrue(),
		),
		Entry("should match on update (status changed to false)",
			condition(resourcesv1alpha1.ConditionTrue),
			condition(resourcesv1alpha1.ConditionFalse),
			BeTrue(),
		),
	)
})

func condition(status resourcesv1alpha1.ConditionStatus) *resourcesv1alpha1.ManagedResourceCondition {
	return &resourcesv1alpha1.ManagedResourceCondition{
		Type:   resourcesv1alpha1.ResourcesApplied,
		Status: status,
	}
}

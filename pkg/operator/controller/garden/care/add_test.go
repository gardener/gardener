// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package care_test

import (
	"context"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
	. "github.com/gardener/gardener/pkg/operator/controller/garden/care"
)

var _ = Describe("Add", func() {
	var (
		ctx           context.Context
		runtimeClient client.Client

		reconciler *Reconciler
		garden     *operatorv1alpha1.Garden
	)

	BeforeEach(func() {
		ctx = context.Background()

		runtimeClient = fakeclient.NewClientBuilder().WithScheme(operatorclient.RuntimeScheme).Build()

		garden = &operatorv1alpha1.Garden{
			ObjectMeta: metav1.ObjectMeta{
				Name: gardenName,
			},
		}

		reconciler = &Reconciler{
			RuntimeClient: runtimeClient,
		}
	})

	Describe("#GardenPredicate", func() {
		var p predicate.Predicate

		BeforeEach(func() {
			p = reconciler.GardenPredicate()
		})

		Describe("#Create", func() {
			It("should return true", func() {
				Expect(p.Create(event.CreateEvent{})).To(BeTrue())
			})
		})

		Describe("#Update", func() {
			It("should return false because new object is no garden", func() {
				Expect(p.Update(event.UpdateEvent{})).To(BeFalse())
			})

			It("should return false because old object is no garden", func() {
				Expect(p.Update(event.UpdateEvent{ObjectNew: garden})).To(BeFalse())
			})

			It("should return false because Reconciled condition does not exist", func() {
				Expect(p.Update(event.UpdateEvent{ObjectOld: garden, ObjectNew: garden})).To(BeFalse())
			})

			It("should return false because Reconciled condition was true before", func() {
				garden.Status.Conditions = []gardencorev1beta1.Condition{{Type: operatorv1alpha1.GardenReconciled, Status: gardencorev1beta1.ConditionTrue}}
				Expect(p.Update(event.UpdateEvent{ObjectOld: garden, ObjectNew: garden})).To(BeFalse())
			})

			It("should return false because Reconciled condition is no longer true", func() {
				garden.Status.Conditions = []gardencorev1beta1.Condition{{Type: operatorv1alpha1.GardenReconciled, Status: gardencorev1beta1.ConditionFalse}}
				oldGarden := garden.DeepCopy()
				oldGarden.Status.Conditions[0].Status = gardencorev1beta1.ConditionTrue
				Expect(p.Update(event.UpdateEvent{ObjectOld: oldGarden, ObjectNew: garden})).To(BeFalse())
			})

			It("should return false because Reconciled condition does no longer exist", func() {
				oldGarden := garden.DeepCopy()
				oldGarden.Status.Conditions = []gardencorev1beta1.Condition{{Type: operatorv1alpha1.GardenReconciled, Status: gardencorev1beta1.ConditionTrue}}
				Expect(p.Update(event.UpdateEvent{ObjectOld: oldGarden, ObjectNew: garden})).To(BeFalse())
			})

			It("should return true because Reconciled condition did not exist before", func() {
				garden.Status.Conditions = []gardencorev1beta1.Condition{{Type: operatorv1alpha1.GardenReconciled, Status: gardencorev1beta1.ConditionTrue}}
				oldGarden := garden.DeepCopy()
				oldGarden.Status.Conditions = nil
				Expect(p.Update(event.UpdateEvent{ObjectOld: oldGarden, ObjectNew: garden})).To(BeTrue())
			})

			It("should return true because Reconciled condition was not true before", func() {
				garden.Status.Conditions = []gardencorev1beta1.Condition{{Type: operatorv1alpha1.GardenReconciled, Status: gardencorev1beta1.ConditionTrue}}
				oldGarden := garden.DeepCopy()
				oldGarden.Status.Conditions[0].Status = gardencorev1beta1.ConditionFalse
				Expect(p.Update(event.UpdateEvent{ObjectOld: oldGarden, ObjectNew: garden})).To(BeTrue())
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

	Describe("#MapManagedResourceToGarden", func() {
		JustBeforeEach(func() {
			Expect(runtimeClient.Create(ctx, garden)).To(Succeed())
		})

		It("should return a request with the garden name", func() {
			Expect(reconciler.MapManagedResourceToGarden(ctx, logr.Discard(), nil, nil)).To(ConsistOf(reconcile.Request{NamespacedName: types.NamespacedName{Name: gardenName}}))
		})
	})
})

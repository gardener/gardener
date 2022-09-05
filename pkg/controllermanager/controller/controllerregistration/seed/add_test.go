// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package seed_test

import (
	"context"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/controllermanager/controller/controllerregistration/seed"

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
)

var _ = Describe("Add", func() {
	var reconciler *Reconciler

	BeforeEach(func() {
		reconciler = &Reconciler{}
	})

	Describe("SeedPredicate", func() {
		var p predicate.Predicate

		BeforeEach(func() {
			p = reconciler.SeedPredicate()
		})

		Describe("#Create", func() {
			It("should return true", func() {
				Expect(p.Create(event.CreateEvent{})).To(BeTrue())
			})
		})

		Describe("#Update", func() {
			var seed *gardencorev1beta1.Seed

			BeforeEach(func() {
				seed = &gardencorev1beta1.Seed{}
			})

			It("should return false because new object is no seed", func() {
				Expect(p.Update(event.UpdateEvent{})).To(BeFalse())
			})

			It("should return false because old object is no seed", func() {
				Expect(p.Update(event.UpdateEvent{ObjectNew: seed})).To(BeFalse())
			})

			It("should return false because neither DNS provider changed nor deletion timestamp got set", func() {
				Expect(p.Update(event.UpdateEvent{ObjectNew: seed, ObjectOld: seed})).To(BeFalse())
			})

			It("should return true because DNS provider changed", func() {
				oldSeed := seed.DeepCopy()
				seed.Spec.DNS.Provider = &gardencorev1beta1.SeedDNSProvider{Type: "foo"}
				Expect(p.Update(event.UpdateEvent{ObjectNew: seed, ObjectOld: oldSeed})).To(BeTrue())
			})

			It("should return true because deletion timestamp got set", func() {
				oldSeed := seed.DeepCopy()
				seed.DeletionTimestamp = &metav1.Time{}
				Expect(p.Update(event.UpdateEvent{ObjectNew: seed, ObjectOld: oldSeed})).To(BeTrue())
			})
		})

		Describe("#Delete", func() {
			It("should return true", func() {
				Expect(p.Delete(event.DeleteEvent{})).To(BeTrue())
			})
		})

		Describe("#Generic", func() {
			It("should return true", func() {
				Expect(p.Generic(event.GenericEvent{})).To(BeTrue())
			})
		})
	})

	Describe("ControllerRegistrationPredicate", func() {
		var p predicate.Predicate

		BeforeEach(func() {
			p = reconciler.ControllerRegistrationPredicate()
		})

		Describe("#Create", func() {
			It("should return true", func() {
				Expect(p.Create(event.CreateEvent{})).To(BeTrue())
			})
		})

		Describe("#Update", func() {
			It("should return true", func() {
				Expect(p.Update(event.UpdateEvent{})).To(BeTrue())
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

	Describe("#MapSeedToShoots", func() {
		var (
			ctx        = context.TODO()
			log        logr.Logger
			fakeClient client.Client

			seed1, seed2 *gardencorev1beta1.Seed
		)

		BeforeEach(func() {
			log = logr.Discard()
			fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()

			seed1 = &gardencorev1beta1.Seed{ObjectMeta: metav1.ObjectMeta{Name: "seed1"}}
			seed2 = &gardencorev1beta1.Seed{ObjectMeta: metav1.ObjectMeta{Name: "seed2"}}

			Expect(fakeClient.Create(ctx, seed1)).To(Succeed())
			Expect(fakeClient.Create(ctx, seed2)).To(Succeed())
		})

		It("should map to all seeds", func() {
			Expect(reconciler.MapToAllSeeds(ctx, log, fakeClient, nil)).To(ConsistOf(
				reconcile.Request{NamespacedName: types.NamespacedName{Name: seed1.Name}},
				reconcile.Request{NamespacedName: types.NamespacedName{Name: seed2.Name}},
			))
		})
	})
})

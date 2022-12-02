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

package managedseed

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
)

var _ = Describe("Add", func() {
	var (
		ctx        = context.TODO()
		fakeClient client.Client
		reconciler *Reconciler
		pred       predicate.Predicate

		managedSeedShoot       *seedmanagementv1alpha1.Shoot
		shoot                  *gardencorev1beta1.Shoot
		seedNameFromSeedConfig string
	)

	BeforeEach(func() {
		reconciler = &Reconciler{
			GardenNamespace: v1beta1constants.GardenNamespace,
		}
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()

		shoot = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
		}

		managedSeedShoot = &seedmanagementv1alpha1.Shoot{
			Name: name,
		}

		seedNameFromSeedConfig = "test-seed"
	})

	Describe("#ManagedSeedFilterPredicate", func() {
		var (
			oldManagedSeed, newManagedSeed *seedmanagementv1alpha1.ManagedSeed
		)

		BeforeEach(func() {
			pred = reconciler.ManagedSeedFilterPredicate(seedNameFromSeedConfig)

			oldManagedSeed = &seedmanagementv1alpha1.ManagedSeed{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
			}

			newManagedSeed = &seedmanagementv1alpha1.ManagedSeed{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
			}

			Expect(inject.StopChannelInto(ctx.Done(), pred)).To(BeTrue())
			Expect(inject.ClientInto(fakeClient, pred)).To(BeTrue())
		})

		It("should return false when ManagedSeed does not reference any shoot", func() {
			Expect(pred.Create(event.CreateEvent{Object: newManagedSeed})).To(BeFalse())
			Expect(pred.Update(event.UpdateEvent{ObjectOld: oldManagedSeed, ObjectNew: newManagedSeed})).To(BeFalse())
			Expect(pred.Delete(event.DeleteEvent{Object: newManagedSeed})).To(BeFalse())
			Expect(pred.Generic(event.GenericEvent{})).To(BeFalse())
		})

		It("should return false when shoot referenced by ManagedSeed is not present", func() {
			oldManagedSeed.Spec.Shoot = managedSeedShoot
			newManagedSeed.Spec.Shoot = managedSeedShoot
			Expect(pred.Create(event.CreateEvent{Object: newManagedSeed})).To(BeFalse())
			Expect(pred.Update(event.UpdateEvent{ObjectOld: oldManagedSeed, ObjectNew: newManagedSeed})).To(BeFalse())
			Expect(pred.Delete(event.DeleteEvent{Object: newManagedSeed})).To(BeFalse())
			Expect(pred.Generic(event.GenericEvent{})).To(BeFalse())
		})

		It("should return false when shoot referenced by ManagedSeed does not reference any seed", func() {
			oldManagedSeed.Spec.Shoot = managedSeedShoot
			newManagedSeed.Spec.Shoot = managedSeedShoot
			Expect(fakeClient.Create(ctx, shoot)).To(Succeed())
			Expect(pred.Create(event.CreateEvent{Object: newManagedSeed})).To(BeFalse())
			Expect(pred.Update(event.UpdateEvent{ObjectOld: oldManagedSeed, ObjectNew: newManagedSeed})).To(BeFalse())
			Expect(pred.Delete(event.DeleteEvent{Object: newManagedSeed})).To(BeFalse())
			Expect(pred.Generic(event.GenericEvent{})).To(BeFalse())
		})

		It("should return true when shoot referenced by ManagedSeed references a seed which is same as the seed mentioned in gardenlet configuration", func() {
			oldManagedSeed.Spec.Shoot = managedSeedShoot
			newManagedSeed.Spec.Shoot = managedSeedShoot
			shoot.Spec.SeedName = pointer.String(seedNameFromSeedConfig)
			Expect(fakeClient.Create(ctx, shoot)).To(Succeed())
			Expect(pred.Create(event.CreateEvent{Object: newManagedSeed})).To(BeTrue())
			Expect(pred.Update(event.UpdateEvent{ObjectOld: oldManagedSeed, ObjectNew: newManagedSeed})).To(BeTrue())
			Expect(pred.Delete(event.DeleteEvent{Object: newManagedSeed})).To(BeTrue())
			Expect(pred.Generic(event.GenericEvent{})).To(BeFalse())
		})

		It("should return false when shoot referenced by ManagedSeed references a seed which is not same as the seed mentioned in gardenlet configuration", func() {
			oldManagedSeed.Spec.Shoot = managedSeedShoot
			newManagedSeed.Spec.Shoot = managedSeedShoot
			shoot.Spec.SeedName = pointer.String("test")
			Expect(fakeClient.Create(ctx, shoot)).To(Succeed())
			Expect(pred.Create(event.CreateEvent{Object: newManagedSeed})).To(BeFalse())
			Expect(pred.Update(event.UpdateEvent{ObjectOld: oldManagedSeed, ObjectNew: newManagedSeed})).To(BeFalse())
			Expect(pred.Delete(event.DeleteEvent{Object: newManagedSeed})).To(BeFalse())
			Expect(pred.Generic(event.GenericEvent{})).To(BeFalse())
		})

		It("should return false when shoot referenced by ManagedSeed has seed name in status field which is not same as the seed mentioned in gardenlet configuration", func() {
			oldManagedSeed.Spec.Shoot = managedSeedShoot
			newManagedSeed.Spec.Shoot = managedSeedShoot
			shoot.Spec.SeedName = pointer.String("test")
			shoot.Status.SeedName = pointer.String("other-seed")
			Expect(fakeClient.Create(ctx, shoot)).To(Succeed())
			Expect(pred.Create(event.CreateEvent{Object: newManagedSeed})).To(BeFalse())
			Expect(pred.Update(event.UpdateEvent{ObjectOld: oldManagedSeed, ObjectNew: newManagedSeed})).To(BeFalse())
			Expect(pred.Delete(event.DeleteEvent{Object: newManagedSeed})).To(BeFalse())
			Expect(pred.Generic(event.GenericEvent{})).To(BeFalse())
		})

		It("should return true when shoot referenced by ManagedSeed has seed name in status field which is same as the seed mentioned in gardenlet configuration", func() {
			oldManagedSeed.Spec.Shoot = managedSeedShoot
			newManagedSeed.Spec.Shoot = managedSeedShoot
			shoot.Spec.SeedName = pointer.String("test")
			shoot.Status.SeedName = pointer.String(seedNameFromSeedConfig)
			Expect(fakeClient.Create(ctx, shoot)).To(Succeed())
			Expect(pred.Create(event.CreateEvent{Object: newManagedSeed})).To(BeTrue())
			Expect(pred.Update(event.UpdateEvent{ObjectOld: oldManagedSeed, ObjectNew: newManagedSeed})).To(BeTrue())
			Expect(pred.Delete(event.DeleteEvent{Object: newManagedSeed})).To(BeTrue())
			Expect(pred.Generic(event.GenericEvent{})).To(BeFalse())
		})
	})
})

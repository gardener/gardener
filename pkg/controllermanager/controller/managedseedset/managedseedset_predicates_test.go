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

package managedseedset

import (
	"context"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
)

var _ = Describe("Predicates", func() {
	var (
		reconciler *Reconciler
		ctx        = context.TODO()
		fakeClient client.Client
		pred       predicate.Predicate

		now = metav1.Now()

		set *seedmanagementv1alpha1.ManagedSeedSet
	)

	BeforeEach(func() {
		reconciler = &Reconciler{}
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()

		set = &seedmanagementv1alpha1.ManagedSeedSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Spec: seedmanagementv1alpha1.ManagedSeedSetSpec{
				Selector: metav1.LabelSelector{
					MatchLabels: map[string]string{
						"name": name,
					},
				},
			},
		}
	})

	Describe("#ShootPredicate", func() {
		BeforeEach(func() {
			pred = reconciler.ShootPredicate()
		})

		Describe("#Create", func() {
			It("should return true", func() {
				Expect(pred.Create(event.CreateEvent{})).To(BeTrue())
			})
		})

		Describe("#Update", func() {
			var (
				e event.UpdateEvent

				oldShoot, newShoot *gardencorev1beta1.Shoot
			)

			BeforeEach(func() {
				oldShoot = &gardencorev1beta1.Shoot{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name + "0",
						Namespace: namespace,
						OwnerReferences: []metav1.OwnerReference{
							{
								Kind: "ManagedSeedSet",
								Name: name,
							},
						},
					},
				}

				newShoot = &gardencorev1beta1.Shoot{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name + "0",
						Namespace: namespace,
						OwnerReferences: []metav1.OwnerReference{
							{
								Kind: "ManagedSeedSet",
								Name: name,
							},
						},
					},
				}

				Expect(inject.StopChannelInto(ctx.Done(), pred)).To(BeTrue())
				Expect(inject.ClientInto(fakeClient, pred)).To(BeTrue())
				e = event.UpdateEvent{ObjectOld: oldShoot, ObjectNew: newShoot}
			})

			It("should return false when Shoot doesnot references any ManagedSeedSet", func() {
				newShoot.OwnerReferences = nil
				Expect(pred.Update(e)).To(BeFalse())
			})

			It("should return false when ManagedSeedSet referenced by Shoot is not present", func() {
				Expect(pred.Update(e)).To(BeFalse())
			})

			It("should return false when ManagedSeedSet referenced by Shoot doesnot have any pending replica", func() {
				Expect(fakeClient.Create(ctx, set)).To(Succeed())
				Expect(pred.Update(e)).To(BeFalse())
			})

			It("should return false when ManagedSeedSet referenced by Shoot have other Shoot in pending replica", func() {
				set.Status.PendingReplica = &seedmanagementv1alpha1.PendingReplica{
					Name: "foo",
				}
				Expect(fakeClient.Create(ctx, set)).To(Succeed())
				Expect(pred.Update(e)).To(BeFalse())
			})

			It("should return true when Shoot health status changes", func() {
				newShoot.Labels = map[string]string{
					v1beta1constants.ShootStatus: "foo",
				}
				set.Status.PendingReplica = &seedmanagementv1alpha1.PendingReplica{
					Name: newShoot.Name,
				}
				Expect(fakeClient.Create(ctx, set)).To(Succeed())
				Expect(pred.Update(e)).To(BeTrue())
			})

			It("should return true when pending replica has ShootRecociling status and Shoot reconciliation failed", func() {
				newShoot.Status.LastOperation = &gardencorev1beta1.LastOperation{
					Type:  gardencorev1beta1.LastOperationTypeCreate,
					State: gardencorev1beta1.LastOperationStateFailed,
				}
				set.Status.PendingReplica = &seedmanagementv1alpha1.PendingReplica{
					Name:   newShoot.Name,
					Reason: seedmanagementv1alpha1.ShootReconcilingReason,
				}
				Expect(fakeClient.Create(ctx, set)).To(Succeed())
				Expect(pred.Update(e)).To(BeTrue())
			})

			It("should return true when pending replica has ShootRecociling status and Shoot reconciliation succeeded", func() {
				newShoot.Status.LastOperation = &gardencorev1beta1.LastOperation{
					Type:  gardencorev1beta1.LastOperationTypeCreate,
					State: gardencorev1beta1.LastOperationStateSucceeded,
				}
				set.Status.PendingReplica = &seedmanagementv1alpha1.PendingReplica{
					Name:   newShoot.Name,
					Reason: seedmanagementv1alpha1.ShootReconcilingReason,
				}
				Expect(fakeClient.Create(ctx, set)).To(Succeed())
				Expect(pred.Update(e)).To(BeTrue())
			})

			It("should return true when pending replica has ShootDeleting status and Shoot deletion failed", func() {
				newShoot.DeletionTimestamp = &now
				newShoot.Status.LastOperation = &gardencorev1beta1.LastOperation{
					Type:  gardencorev1beta1.LastOperationTypeDelete,
					State: gardencorev1beta1.LastOperationStateFailed,
				}
				set.Status.PendingReplica = &seedmanagementv1alpha1.PendingReplica{
					Name:   newShoot.Name,
					Reason: seedmanagementv1alpha1.ShootDeletingReason,
				}
				Expect(fakeClient.Create(ctx, set)).To(Succeed())
				Expect(pred.Update(e)).To(BeTrue())
			})

			It("should return false when pending replica has ShootReconcileFailed status and Shoot reconciliation failed", func() {
				newShoot.Status.LastOperation = &gardencorev1beta1.LastOperation{
					Type:  gardencorev1beta1.LastOperationTypeReconcile,
					State: gardencorev1beta1.LastOperationStateFailed,
				}
				set.Status.PendingReplica = &seedmanagementv1alpha1.PendingReplica{
					Name:   newShoot.Name,
					Reason: seedmanagementv1alpha1.ShootReconcileFailedReason,
				}
				Expect(fakeClient.Create(ctx, set)).To(Succeed())
				Expect(pred.Update(e)).To(BeFalse())
			})

			It("should return false when pending replica has ShootDeleteFailed status and Shoot deletion failed", func() {
				newShoot.DeletionTimestamp = &now
				newShoot.Status.LastOperation = &gardencorev1beta1.LastOperation{
					Type:  gardencorev1beta1.LastOperationTypeDelete,
					State: gardencorev1beta1.LastOperationStateFailed,
				}
				set.Status.PendingReplica = &seedmanagementv1alpha1.PendingReplica{
					Name:   newShoot.Name,
					Reason: seedmanagementv1alpha1.ShootDeleteFailedReason,
				}
				Expect(fakeClient.Create(ctx, set)).To(Succeed())
				Expect(pred.Update(e)).To(BeFalse())
			})

			It("should return true when pending replica has ShootNotHealthy status and Shoot got healthy", func() {
				newShoot.Labels = map[string]string{
					v1beta1constants.ShootStatus: "healthy",
				}
				set.Status.PendingReplica = &seedmanagementv1alpha1.PendingReplica{
					Name:   newShoot.Name,
					Reason: seedmanagementv1alpha1.ShootNotHealthyReason,
				}
				Expect(fakeClient.Create(ctx, set)).To(Succeed())
				Expect(pred.Update(e)).To(BeTrue())
			})

			It("should return false in default case", func() {
				set.Status.PendingReplica = &seedmanagementv1alpha1.PendingReplica{
					Name:   newShoot.Name,
					Reason: "foo",
				}
				Expect(fakeClient.Create(ctx, set)).To(Succeed())
				Expect(pred.Update(e)).To(BeFalse())
			})
		})

		Describe("#Delete", func() {
			It("should return true", func() {
				Expect(pred.Delete(event.DeleteEvent{})).To(BeTrue())
			})
		})

		Describe("#Generic", func() {
			It("should return true", func() {
				Expect(pred.Generic(event.GenericEvent{})).To(BeFalse())
			})
		})
	})

	Describe("#ManagedSeedPredicate", func() {
		BeforeEach(func() {
			pred = reconciler.ManagedSeedPredicate()
		})

		Describe("#Create", func() {
			It("should return true", func() {
				Expect(pred.Create(event.CreateEvent{})).To(BeTrue())
			})
		})

		Describe("#Update", func() {
			var (
				e event.UpdateEvent

				oldManagedSeed, newManagedSeed *seedmanagementv1alpha1.ManagedSeed
			)

			BeforeEach(func() {
				oldManagedSeed = &seedmanagementv1alpha1.ManagedSeed{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name + "0",
						Namespace: namespace,
						OwnerReferences: []metav1.OwnerReference{
							{
								Kind: "ManagedSeedSet",
								Name: name,
							},
						},
					},
				}

				newManagedSeed = &seedmanagementv1alpha1.ManagedSeed{
					ObjectMeta: metav1.ObjectMeta{
						Name:      name + "0",
						Namespace: namespace,
						OwnerReferences: []metav1.OwnerReference{
							{
								Kind: "ManagedSeedSet",
								Name: name,
							},
						},
					},
				}

				Expect(inject.StopChannelInto(ctx.Done(), pred)).To(BeTrue())
				Expect(inject.ClientInto(fakeClient, pred)).To(BeTrue())
				e = event.UpdateEvent{ObjectOld: oldManagedSeed, ObjectNew: newManagedSeed}
			})

			It("should return false when ManagedSeed doesnot references any ManagedSeedSet", func() {
				newManagedSeed.OwnerReferences = nil
				Expect(pred.Update(e)).To(BeFalse())
			})

			It("should return false when ManagedSeedSet referenced by ManagedSeed is not present", func() {
				Expect(pred.Update(e)).To(BeFalse())
			})

			It("should return false when ManagedSeedSet referenced by ManagedSeed doesnot have any pending replica", func() {
				Expect(fakeClient.Create(ctx, set)).To(Succeed())
				Expect(pred.Update(e)).To(BeFalse())
			})

			It("should return false when ManagedSeedSet referenced by ManagedSeed have other managed seed in pending replica", func() {
				set.Status.PendingReplica = &seedmanagementv1alpha1.PendingReplica{
					Name: "foo",
				}
				Expect(fakeClient.Create(ctx, set)).To(Succeed())
				Expect(pred.Update(e)).To(BeFalse())
			})

			It("should return true when pending replica has ManagedSeedPreparingReason status and ManagedSeed's Seed is registered", func() {
				newManagedSeed.Status.Conditions = []gardencorev1beta1.Condition{
					{Type: seedmanagementv1alpha1.ManagedSeedSeedRegistered, Status: gardencorev1beta1.ConditionTrue},
				}
				set.Status.PendingReplica = &seedmanagementv1alpha1.PendingReplica{
					Name:   newManagedSeed.Name,
					Reason: seedmanagementv1alpha1.ManagedSeedPreparingReason,
				}
				Expect(fakeClient.Create(ctx, set)).To(Succeed())
				Expect(pred.Update(e)).To(BeTrue())
			})

			It("should return false in default case", func() {
				set.Status.PendingReplica = &seedmanagementv1alpha1.PendingReplica{
					Name:   newManagedSeed.Name,
					Reason: "foo",
				}
				Expect(fakeClient.Create(ctx, set)).To(Succeed())
				Expect(pred.Update(e)).To(BeFalse())
			})
		})

		Describe("#Delete", func() {
			It("should return true", func() {
				Expect(pred.Delete(event.DeleteEvent{})).To(BeTrue())
			})
		})

		Describe("#Generic", func() {
			It("should return true", func() {
				Expect(pred.Generic(event.GenericEvent{})).To(BeFalse())
			})
		})
	})
})

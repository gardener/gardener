// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shootstate_test

import (
	"context"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllermanager/controller/shootstate"
)

var _ = Describe("Add", func() {
	var (
		c          client.Client
		reconciler *shootstate.Reconciler
	)

	BeforeEach(func() {
		c = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()
		reconciler = &shootstate.Reconciler{
			Client: c,
		}
	})

	Describe("#MapShootToShootState", func() {
		var (
			ctx context.Context
			log logr.Logger

			shootName      = "shoot-1"
			shootNamespace = "ns-1"
		)

		BeforeEach(func() {
			ctx = context.Background()
			log = logr.Discard()
		})

		It("should return reconciliation request matching the shoot name", func() {
			shoot := &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      shootName,
					Namespace: shootNamespace,
				},
			}

			reconciliationRequest := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      shootName,
					Namespace: shootNamespace,
				},
			}

			shootState := &gardencorev1beta1.ShootState{
				ObjectMeta: metav1.ObjectMeta{
					Name:      shootName,
					Namespace: shootNamespace,
				},
			}
			Expect(c.Create(ctx, shootState)).To(Succeed())

			requests := reconciler.MapShootToShootState(log)(ctx, shoot)
			Expect(requests).To(ConsistOf(reconciliationRequest))
		})

		It("should return nil if ShootState is not present", func() {
			shoot := &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      shootName,
					Namespace: shootNamespace,
				},
			}
			requests := reconciler.MapShootToShootState(log)(ctx, shoot)
			Expect(requests).To(BeEmpty())
		})

		It("should return nil if argument is not a gardencorev1beta1.Shoot", func() {
			deploymentName := "dep-1"
			deploymentNamespace := "ns-1"
			deployment := appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      deploymentName,
					Namespace: deploymentNamespace,
				},
			}

			requests := reconciler.MapShootToShootState(log)(ctx, &deployment)
			Expect(requests).To(BeNil())
		})
	})

	Describe("ShootPredicate", func() {
		var p predicate.Predicate

		BeforeEach(func() {
			p = reconciler.ShootPredicates()
		})

		Describe("#Update", func() {
			var (
				shoot    *gardencorev1beta1.Shoot
				shootOld *gardencorev1beta1.Shoot
			)

			BeforeEach(func() {
				shoot = &gardencorev1beta1.Shoot{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "shoot-",
						Namespace: "ns-",
					},
				}

				shootOld = shoot.DeepCopy()
			})

			It("should return false because new object is not a shoot", func() {
				Expect(p.Update(event.UpdateEvent{})).To(BeFalse())
			})

			It("should return false because old object is not a shoot", func() {
				Expect(p.Update(event.UpdateEvent{ObjectNew: shoot})).To(BeFalse())
			})

			It("should return true if last operation transitions from Reconcile to Migrate", func() {
				shootOld.Status.LastOperation = &gardencorev1beta1.LastOperation{
					Type: gardencorev1beta1.LastOperationTypeReconcile,
				}
				shoot.Status.LastOperation = &gardencorev1beta1.LastOperation{
					Type: gardencorev1beta1.LastOperationTypeMigrate,
				}
				Expect(p.Update(event.UpdateEvent{ObjectOld: shootOld, ObjectNew: shoot})).To(BeTrue())
			})

			It("should return true if last operation transitions from Migrate to Restore", func() {
				shootOld.Status.LastOperation = &gardencorev1beta1.LastOperation{
					Type: gardencorev1beta1.LastOperationTypeMigrate,
				}
				shoot.Status.LastOperation = &gardencorev1beta1.LastOperation{
					Type: gardencorev1beta1.LastOperationTypeRestore,
				}
				Expect(p.Update(event.UpdateEvent{ObjectOld: shootOld, ObjectNew: shoot})).To(BeTrue())
			})

			It("should return true if Restore operation succeeds", func() {
				shootOld.Status.LastOperation = &gardencorev1beta1.LastOperation{
					Type: gardencorev1beta1.LastOperationTypeRestore,
				}
				shoot.Status.LastOperation = &gardencorev1beta1.LastOperation{
					Type:  gardencorev1beta1.LastOperationTypeRestore,
					State: gardencorev1beta1.LastOperationStateSucceeded,
				}
				Expect(p.Update(event.UpdateEvent{ObjectOld: shootOld, ObjectNew: shoot})).To(BeTrue())
			})

			It("should return true if last operation transitions from Restore to Reconcile", func() {
				shootOld.Status.LastOperation = &gardencorev1beta1.LastOperation{
					Type: gardencorev1beta1.LastOperationTypeRestore,
				}
				shoot.Status.LastOperation = &gardencorev1beta1.LastOperation{
					Type: gardencorev1beta1.LastOperationTypeReconcile,
				}
				Expect(p.Update(event.UpdateEvent{ObjectOld: shootOld, ObjectNew: shoot})).To(BeTrue())
			})
		})
	})
})

// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shootstate_test

import (
	"context"

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

var _ = Describe("AddToManager", func() {
	var (
		ctx        context.Context
		c          client.Client
		reconciler *shootstate.Reconciler
	)

	BeforeEach(func() {
		ctx = context.Background()
		c = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()
		reconciler = &shootstate.Reconciler{
			Client: c,
		}
	})

	Describe("#MapShootToShootState", func() {
		var (
			ctx context.Context
		)

		BeforeEach(func() {
			ctx = context.Background()
		})

		It("should return reconciliation request matching the shoot name", func() {
			shootName := "shoot-1"
			shootNamespace := "ns-1"
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

			requests := reconciler.MapShootToShootState(ctx, shoot)
			Expect(requests).To(ConsistOf(reconciliationRequest))
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

			requests := reconciler.MapShootToShootState(ctx, &deployment)
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

				shootState *gardencorev1beta1.ShootState
			)

			BeforeEach(func() {
				shoot = &gardencorev1beta1.Shoot{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "shoot-",
						Namespace: "ns-",
					},
					Status: gardencorev1beta1.ShootStatus{},
				}
				Expect(c.Create(ctx, shoot)).To(Succeed())

				shootOld = shoot.DeepCopy()

				shootState = &gardencorev1beta1.ShootState{
					ObjectMeta: metav1.ObjectMeta{
						Name:      shoot.Name,
						Namespace: shoot.Namespace,
					},
				}
				Expect(c.Create(ctx, shootState)).To(Succeed())
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

			It("should return true if Restore operation succeeds ", func() {
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

			It("should return false if ShootState is not present", func() {
				Expect(c.Delete(ctx, shootState)).To(Succeed())

				shootOld.Status.LastOperation = &gardencorev1beta1.LastOperation{
					Type: gardencorev1beta1.LastOperationTypeReconcile,
				}
				shoot.Status.LastOperation = &gardencorev1beta1.LastOperation{
					Type: gardencorev1beta1.LastOperationTypeMigrate,
				}
				Expect(p.Update(event.UpdateEvent{ObjectOld: shootOld, ObjectNew: shoot})).To(BeFalse())
			})
		})
	})
})

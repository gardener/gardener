// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bastion_test

import (
	"context"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/gardener/gardener/pkg/api/indexer"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/apis/operations"
	operationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/controllermanager/controller/bastion"
)

var _ = Describe("Add", func() {
	var reconciler *Reconciler

	BeforeEach(func() {
		reconciler = &Reconciler{}
	})

	Describe("ShootPredicate", func() {
		var (
			p   predicate.Predicate
			obj *gardencorev1beta1.Shoot
		)

		BeforeEach(func() {
			p = reconciler.ShootPredicate()
			obj = &gardencorev1beta1.Shoot{}
		})

		Describe("#Create", func() {
			var e event.CreateEvent

			BeforeEach(func() {
				e = event.CreateEvent{Object: obj}
			})

			It("should return false if the object is not deleting", func() {
				Expect(p.Create(e)).To(BeFalse())
			})

			It("should return true if object is deleting", func() {
				obj.DeletionTimestamp = &metav1.Time{}
				Expect(p.Create(e)).To(BeTrue())
			})
		})

		Describe("#Delete", func() {
			var e event.DeleteEvent

			BeforeEach(func() {
				e = event.DeleteEvent{Object: obj}
			})

			It("should return false if the object is not deleting", func() {
				Expect(p.Delete(e)).To(BeFalse())
			})

			It("should return true if object is deleting", func() {
				obj.DeletionTimestamp = &metav1.Time{}
				Expect(p.Delete(e)).To(BeTrue())
			})
		})

		Describe("#Generic", func() {
			var e event.GenericEvent

			BeforeEach(func() {
				e = event.GenericEvent{Object: obj}
			})

			It("should return false if the object is not deleting", func() {
				Expect(p.Generic(e)).To(BeFalse())
			})

			It("should return true if object is deleting", func() {
				obj.DeletionTimestamp = &metav1.Time{}
				Expect(p.Generic(e)).To(BeTrue())
			})
		})

		Describe("#Update", func() {
			var (
				e      event.UpdateEvent
				objNew *gardencorev1beta1.Shoot
			)

			BeforeEach(func() {
				objNew = obj.DeepCopy()
				e = event.UpdateEvent{ObjectOld: obj, ObjectNew: objNew}
			})

			It("should return false if the object is not deleting and seed name did not change", func() {
				Expect(p.Update(e)).To(BeFalse())
			})

			It("should return false when shoot is scheduled for the first time", func() {
				obj.Spec.SeedName = nil
				objNew.Spec.SeedName = ptr.To("some-seed-name")

				Expect(p.Update(e)).To(BeFalse())
			})

			It("should return true when seed name changed", func() {
				obj.Spec.SeedName = ptr.To("old-seed")
				objNew.Spec.SeedName = ptr.To("new-seed")

				Expect(p.Update(e)).To(BeTrue())
			})

			It("should return true if object is deleting", func() {
				objNew.DeletionTimestamp = &metav1.Time{}
				Expect(p.Update(e)).To(BeTrue())
			})
		})
	})

	Describe("MapShootToBastions", func() {
		var (
			ctx        = context.TODO()
			log        logr.Logger
			fakeClient client.Client
		)

		BeforeEach(func() {
			log = logr.Discard()
			fakeClient = fakeclient.NewClientBuilder().
				WithScheme(kubernetes.GardenScheme).
				WithIndex(&operationsv1alpha1.Bastion{}, operations.BastionShootName, indexer.BastionShootNameIndexerFunc).
				Build()
			reconciler.Client = fakeClient
		})

		It("should do nothing if the object is no shoot", func() {
			Expect(reconciler.MapShootToBastions(log)(ctx, &corev1.Secret{})).To(BeEmpty())
		})

		It("should map the shoot to bastions", func() {
			var (
				shoot = &gardencorev1beta1.Shoot{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "foo",
						Namespace: "some-namespace",
					},
				}
				bastion1 = &operationsv1alpha1.Bastion{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "bastion1",
						Namespace: shoot.Namespace,
					},
					Spec: operationsv1alpha1.BastionSpec{
						ShootRef: corev1.LocalObjectReference{
							Name: shoot.Name,
						},
					},
				}
				bastion2 = &operationsv1alpha1.Bastion{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "bastion2",
						Namespace: shoot.Namespace,
					},
					Spec: operationsv1alpha1.BastionSpec{
						ShootRef: corev1.LocalObjectReference{
							// the fake client does not implement the field selector options, so we should better use
							// the same shoot name here (otherwise, we could have tested with a different shoot name)
							Name: shoot.Name,
						},
					},
				}
				bastion3 = &operationsv1alpha1.Bastion{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "bastion3",
						Namespace: shoot.Namespace,
					},
					Spec: operationsv1alpha1.BastionSpec{
						ShootRef: corev1.LocalObjectReference{
							Name: "other",
						},
					},
				}
			)

			Expect(fakeClient.Create(ctx, bastion1)).To(Succeed())
			Expect(fakeClient.Create(ctx, bastion2)).To(Succeed())
			Expect(fakeClient.Create(ctx, bastion3)).To(Succeed())

			Expect(reconciler.MapShootToBastions(log)(ctx, shoot)).To(ConsistOf(
				reconcile.Request{NamespacedName: types.NamespacedName{Name: bastion1.Name, Namespace: bastion1.Namespace}},
				reconcile.Request{NamespacedName: types.NamespacedName{Name: bastion2.Name, Namespace: bastion2.Namespace}},
			))
		})
	})
})

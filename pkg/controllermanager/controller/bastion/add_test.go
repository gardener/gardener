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

package bastion

import (
	"context"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("Add", func() {
	Describe("shootPredicate", func() {
		var obj *gardencorev1beta1.Shoot

		BeforeEach(func() {
			obj = &gardencorev1beta1.Shoot{}
		})

		Describe("#Create", func() {
			var e event.CreateEvent

			JustBeforeEach(func() {
				e = event.CreateEvent{Object: obj}
			})

			It("should return false if the object is not deleting", func() {
				Expect(shootPredicate.Create(e)).To(BeFalse())
			})

			Context("when object is deleting", func() {
				BeforeEach(func() {
					obj.DeletionTimestamp = &metav1.Time{}
				})

				It("should return true", func() {
					Expect(shootPredicate.Create(e)).To(BeTrue())
				})
			})
		})

		Describe("#Delete", func() {
			var e event.DeleteEvent

			JustBeforeEach(func() {
				e = event.DeleteEvent{Object: obj}
			})

			It("should return false if the object is not deleting", func() {
				Expect(shootPredicate.Delete(e)).To(BeFalse())
			})

			Context("when object is deleting", func() {
				BeforeEach(func() {
					obj.DeletionTimestamp = &metav1.Time{}
				})

				It("should return true", func() {
					Expect(shootPredicate.Delete(e)).To(BeTrue())
				})
			})
		})

		Describe("#Generic", func() {
			var e event.GenericEvent

			JustBeforeEach(func() {
				e = event.GenericEvent{Object: obj}
			})

			It("should return false if the object is not deleting", func() {
				Expect(shootPredicate.Generic(e)).To(BeFalse())
			})

			Context("when object is deleting", func() {
				BeforeEach(func() {
					obj.DeletionTimestamp = &metav1.Time{}
				})

				It("should return true", func() {
					Expect(shootPredicate.Generic(e)).To(BeTrue())
				})
			})
		})

		Describe("#Update", func() {
			var (
				e      event.UpdateEvent
				objNew *gardencorev1beta1.Shoot
			)

			BeforeEach(func() {
				objNew = obj.DeepCopy()
			})

			JustBeforeEach(func() {
				e = event.UpdateEvent{ObjectOld: obj, ObjectNew: objNew}
			})

			It("should return false if the object is not deleting and seed name did not change", func() {
				Expect(shootPredicate.Update(e)).To(BeFalse())
			})

			Context("when shoot is scheduled for the first time", func() {
				BeforeEach(func() {
					obj.Spec.SeedName = nil
					objNew.Spec.SeedName = pointer.String("some-seed-name")
				})

				It("should return false", func() {
					Expect(shootPredicate.Update(e)).To(BeFalse())
				})
			})

			Context("when seed name changed", func() {
				BeforeEach(func() {
					obj.Spec.SeedName = pointer.String("old-seed")
					objNew.Spec.SeedName = pointer.String("new-seed")
				})

				It("should return true", func() {
					Expect(shootPredicate.Update(e)).To(BeTrue())
				})
			})

			Context("when object is deleting", func() {
				BeforeEach(func() {
					objNew.DeletionTimestamp = &metav1.Time{}
				})

				It("should return true", func() {
					Expect(shootPredicate.Update(e)).To(BeTrue())
				})
			})
		})
	})

	Describe("mapShootToBastions", func() {
		var (
			ctx        = context.TODO()
			log        logr.Logger
			fakeClient client.Client
		)

		BeforeEach(func() {
			log = logr.Discard()
			fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()
		})

		It("should do nothing if the object is no shoot", func() {
			Expect(mapShootToBastions(ctx, log, fakeClient, &corev1.Secret{})).To(BeEmpty())
		})

		It("should map the shoot to bastions", func() {
			var (
				shoot = &gardencorev1beta1.Shoot{
					ObjectMeta: metav1.ObjectMeta{
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
			)

			Expect(fakeClient.Create(ctx, bastion1)).To(Succeed())
			Expect(fakeClient.Create(ctx, bastion2)).To(Succeed())

			Expect(mapShootToBastions(ctx, log, fakeClient, shoot)).To(ConsistOf(
				reconcile.Request{NamespacedName: types.NamespacedName{Name: bastion1.Name, Namespace: bastion1.Namespace}},
				reconcile.Request{NamespacedName: types.NamespacedName{Name: bastion2.Name, Namespace: bastion2.Namespace}},
			))
		})
	})
})

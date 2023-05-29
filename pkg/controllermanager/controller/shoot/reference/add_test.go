// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package reference_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/controllermanager/controller/shoot/reference"
)

var _ = Describe("Add", func() {
	var reconciler *Reconciler

	BeforeEach(func() {
		reconciler = &Reconciler{}
	})

	Describe("ShootPredicate", func() {
		var (
			p     predicate.Predicate
			shoot *gardencorev1beta1.Shoot
		)

		BeforeEach(func() {
			p = reconciler.ShootPredicate()
			shoot = &gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Kubernetes: gardencorev1beta1.Kubernetes{
						KubeAPIServer: &gardencorev1beta1.KubeAPIServerConfig{},
					},
				},
			}
		})

		Describe("#Create", func() {
			It("should return true", func() {
				Expect(p.Create(event.CreateEvent{})).To(BeTrue())
			})
		})

		Describe("#Update", func() {
			It("should return false because new object is no shoot", func() {
				Expect(p.Update(event.UpdateEvent{})).To(BeFalse())
			})

			It("should return false because old object is no shoot", func() {
				Expect(p.Update(event.UpdateEvent{ObjectNew: shoot})).To(BeFalse())
			})

			Context("shoot has no deletion timestamp", func() {
				It("should return false because there is no ref change", func() {
					Expect(p.Update(event.UpdateEvent{ObjectNew: shoot, ObjectOld: shoot})).To(BeFalse())
				})

				It("should return because the DNS fields changed", func() {
					oldShoot := shoot.DeepCopy()
					shoot.Spec.DNS = &gardencorev1beta1.DNS{
						Providers: []gardencorev1beta1.DNSProvider{{
							SecretName: pointer.String("secret"),
						}},
					}
					Expect(p.Update(event.UpdateEvent{ObjectNew: shoot, ObjectOld: oldShoot})).To(BeTrue())
				})

				It("should return true because the audit policy field changed", func() {
					oldShoot := shoot.DeepCopy()
					shoot.Spec.Kubernetes.KubeAPIServer.AuditConfig = &gardencorev1beta1.AuditConfig{
						AuditPolicy: &gardencorev1beta1.AuditPolicy{
							ConfigMapRef: &corev1.ObjectReference{
								Name: "audit-policy",
							},
						},
					}
					Expect(p.Update(event.UpdateEvent{ObjectNew: shoot, ObjectOld: oldShoot})).To(BeTrue())
				})

				It("should return false because the resources field changed", func() {
					oldShoot := shoot.DeepCopy()
					shoot.Spec.Resources = []gardencorev1beta1.NamedResourceReference{{
						Name: "resource-1",
						ResourceRef: autoscalingv1.CrossVersionObjectReference{
							APIVersion: "v1",
							Kind:       "Secret",
							Name:       "test",
						},
					}}
					Expect(p.Update(event.UpdateEvent{ObjectNew: shoot, ObjectOld: oldShoot})).To(BeTrue())
				})
			})

			Context("shoot has deletion timestamp", func() {
				BeforeEach(func() {
					shoot.DeletionTimestamp = &metav1.Time{}
				})

				It("should return false because shoot contains finalizer", func() {
					oldShoot := shoot.DeepCopy()
					shoot.Finalizers = append(shoot.Finalizers, "gardener")
					Expect(p.Update(event.UpdateEvent{ObjectNew: shoot, ObjectOld: oldShoot})).To(BeFalse())
				})

				It("should return false because shoot does not contain finalizer", func() {
					Expect(p.Update(event.UpdateEvent{ObjectNew: shoot, ObjectOld: shoot})).To(BeTrue())
				})
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
})

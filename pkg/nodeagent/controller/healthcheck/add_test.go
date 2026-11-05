// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package healthcheck_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	. "github.com/gardener/gardener/pkg/nodeagent/controller/healthcheck"
)

var _ = Describe("Add", func() {
	var reconciler *Reconciler

	BeforeEach(func() {
		reconciler = &Reconciler{}
	})

	Describe("#PodToNodeMapper", func() {
		It("should map the pod to its node", func() {
			pod := &corev1.Pod{Spec: corev1.PodSpec{NodeName: "node"}}

			Expect(reconciler.PodToNodeMapper()(context.Background(), pod)).To(ConsistOf(
				reconcile.Request{NamespacedName: types.NamespacedName{Name: "node"}},
			))
		})

		It("should return nil for a non-pod object", func() {
			Expect(reconciler.PodToNodeMapper()(context.Background(), &corev1.Node{})).To(BeNil())
		})
	})

	Describe("#IsCriticalStaticPodPredicate", func() {
		var predicate func() predicate.Predicate

		BeforeEach(func() {
			predicate = reconciler.IsCriticalStaticPodPredicate
		})

		It("should return true for a critical static pod", func() {
			pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"shoot.gardener.cloud/is-critical-static-pod": "true"}}}

			Expect(predicate().Create(event.CreateEvent{Object: pod})).To(BeTrue())
			Expect(predicate().Update(event.UpdateEvent{ObjectNew: pod})).To(BeTrue())
			Expect(predicate().Delete(event.DeleteEvent{Object: pod})).To(BeTrue())
			Expect(predicate().Generic(event.GenericEvent{Object: pod})).To(BeTrue())
		})

		It("should return false for a non-critical pod", func() {
			pod := &corev1.Pod{}

			Expect(predicate().Create(event.CreateEvent{Object: pod})).To(BeFalse())
			Expect(predicate().Update(event.UpdateEvent{ObjectNew: pod})).To(BeFalse())
			Expect(predicate().Delete(event.DeleteEvent{Object: pod})).To(BeFalse())
			Expect(predicate().Generic(event.GenericEvent{Object: pod})).To(BeFalse())
		})
	})

	Describe("#CriticalPodHealthStatusChangedPredicate", func() {
		var (
			predicate func() predicate.Predicate

			pod = func(ready corev1.ConditionStatus, phase corev1.PodPhase) *corev1.Pod {
				return &corev1.Pod{
					Status: corev1.PodStatus{
						Phase:      phase,
						Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: ready}},
					},
				}
			}
		)

		BeforeEach(func() {
			predicate = reconciler.CriticalPodHealthStatusChangedPredicate
		})

		Describe("#Create", func() {
			It("should always return true", func() {
				Expect(predicate().Create(event.CreateEvent{Object: pod(corev1.ConditionTrue, corev1.PodRunning)})).To(BeTrue())
			})
		})

		Describe("#Delete", func() {
			It("should always return true", func() {
				Expect(predicate().Delete(event.DeleteEvent{Object: pod(corev1.ConditionTrue, corev1.PodRunning)})).To(BeTrue())
			})
		})

		Describe("#Generic", func() {
			It("should always return true", func() {
				Expect(predicate().Generic(event.GenericEvent{Object: pod(corev1.ConditionTrue, corev1.PodRunning)})).To(BeTrue())
			})
		})

		Describe("#Update", func() {
			It("should return false when the old object is not a pod", func() {
				Expect(predicate().Update(event.UpdateEvent{ObjectOld: &corev1.Node{}, ObjectNew: pod(corev1.ConditionTrue, corev1.PodRunning)})).To(BeFalse())
			})

			It("should return false when the new object is not a pod", func() {
				Expect(predicate().Update(event.UpdateEvent{ObjectOld: pod(corev1.ConditionTrue, corev1.PodRunning), ObjectNew: &corev1.Node{}})).To(BeFalse())
			})

			It("should return false when nothing relevant changed", func() {
				oldPod := pod(corev1.ConditionTrue, corev1.PodRunning)
				Expect(predicate().Update(event.UpdateEvent{ObjectOld: oldPod, ObjectNew: oldPod.DeepCopy()})).To(BeFalse())
			})

			It("should return true when the phase changed", func() {
				Expect(predicate().Update(event.UpdateEvent{
					ObjectOld: pod(corev1.ConditionTrue, corev1.PodRunning),
					ObjectNew: pod(corev1.ConditionTrue, corev1.PodPending),
				})).To(BeTrue())
			})

			It("should return true when the pod ready condition status changed", func() {
				Expect(predicate().Update(event.UpdateEvent{
					ObjectOld: pod(corev1.ConditionTrue, corev1.PodRunning),
					ObjectNew: pod(corev1.ConditionFalse, corev1.PodRunning),
				})).To(BeTrue())
			})

			It("should return true when the pod ready condition was added", func() {
				oldPod := pod(corev1.ConditionTrue, corev1.PodRunning)
				oldPod.Status.Conditions = nil
				Expect(predicate().Update(event.UpdateEvent{ObjectOld: oldPod, ObjectNew: pod(corev1.ConditionTrue, corev1.PodRunning)})).To(BeTrue())
			})

			It("should return true when the pod ready condition was removed", func() {
				newPod := pod(corev1.ConditionTrue, corev1.PodRunning)
				newPod.Status.Conditions = nil
				Expect(predicate().Update(event.UpdateEvent{ObjectOld: pod(corev1.ConditionTrue, corev1.PodRunning), ObjectNew: newPod})).To(BeTrue())
			})
		})
	})
})

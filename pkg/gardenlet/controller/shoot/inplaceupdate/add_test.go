// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package inplaceupdate_test

import (
	"context"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	. "github.com/gardener/gardener/pkg/gardenlet/controller/shoot/inplaceupdate"
)

var _ = Describe("Add", func() {
	var (
		ctx  context.Context
		node *corev1.Node
	)

	BeforeEach(func() {
		ctx = context.TODO()

		node = &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "node-1",
				Labels: map[string]string{},
			},
		}
	})

	Describe("#MapNodeToPool", func() {
		It("should return a reconcile request for the pool secret name", func() {
			node.Labels[v1beta1constants.LabelWorkerPoolGardenerNodeAgentSecretName] = "pool-secret-abc"

			Expect(MapNodeToPool(ctx, node)).To(ConsistOf(
				reconcile.Request{NamespacedName: client.ObjectKey{Name: "pool-secret-abc"}},
			))
		})

		It("should return nil when the node has no pool-secret label", func() {
			Expect(MapNodeToPool(ctx, node)).To(BeNil())
		})

		It("should return nil when the pool-secret label is empty", func() {
			node.Labels[v1beta1constants.LabelWorkerPoolGardenerNodeAgentSecretName] = ""

			Expect(MapNodeToPool(ctx, node)).To(BeNil())
		})
	})

	Describe("#NodeInPlaceUpdateStatePredicate", func() {
		var p predicate.Predicate

		BeforeEach(func() {
			p = NodeInPlaceUpdateStatePredicate()
		})

		Describe("#Create", func() {
			It("should return true when the node has the needs-drain annotation set to true", func() {
				node.Annotations = map[string]string{v1beta1constants.AnnotationNodeAgentInPlaceUpdateNeedsDrain: "true"}

				Expect(p.Create(event.CreateEvent{Object: node})).To(BeTrue())
			})

			It("should return true when the node has the update-result label set", func() {
				node.Labels = map[string]string{
					machinev1alpha1.LabelKeyNodeUpdateResult: machinev1alpha1.LabelValueNodeUpdateSuccessful,
				}

				Expect(p.Create(event.CreateEvent{Object: node})).To(BeTrue())
			})

			It("should return true when the node has the NodeInPlaceUpdate condition set", func() {
				node.Status.Conditions = []corev1.NodeCondition{{
					Type:   machinev1alpha1.NodeInPlaceUpdate,
					Status: corev1.ConditionTrue,
					Reason: machinev1alpha1.ReadyForUpdate,
				}}

				Expect(p.Create(event.CreateEvent{Object: node})).To(BeTrue())
			})

			It("should return false otherwise", func() {
				Expect(p.Create(event.CreateEvent{Object: node})).To(BeFalse())
			})

			It("should return false when the NodeInPlaceUpdate condition reason is UpdateSuccessful", func() {
				node.Status.Conditions = []corev1.NodeCondition{{
					Type:   machinev1alpha1.NodeInPlaceUpdate,
					Status: corev1.ConditionTrue,
					Reason: machinev1alpha1.UpdateSuccessful,
				}}

				Expect(p.Create(event.CreateEvent{Object: node})).To(BeFalse())
			})
		})

		Describe("#Update", func() {
			It("should return true when the needs-drain annotation transitions from absent to true", func() {
				oldNode := node.DeepCopy()
				newNode := node.DeepCopy()
				newNode.Annotations = map[string]string{v1beta1constants.AnnotationNodeAgentInPlaceUpdateNeedsDrain: "true"}

				Expect(p.Update(event.UpdateEvent{ObjectOld: oldNode, ObjectNew: newNode})).To(BeTrue())
			})

			It("should return true when the update-result label changes", func() {
				oldNode := node.DeepCopy()
				newNode := node.DeepCopy()
				newNode.Labels = map[string]string{
					machinev1alpha1.LabelKeyNodeUpdateResult: machinev1alpha1.LabelValueNodeUpdateSuccessful,
				}

				Expect(p.Update(event.UpdateEvent{ObjectOld: oldNode, ObjectNew: newNode})).To(BeTrue())
			})

			It("should return false when neither needs-drain nor update-result changes", func() {
				oldNode := node.DeepCopy()
				newNode := node.DeepCopy()
				newNode.Status.Conditions = []corev1.NodeCondition{{
					Type:   machinev1alpha1.NodeInPlaceUpdate,
					Status: corev1.ConditionTrue,
					Reason: machinev1alpha1.ReadyForUpdate,
				}}

				Expect(p.Update(event.UpdateEvent{ObjectOld: oldNode, ObjectNew: newNode})).To(BeFalse())
			})

			It("should return false when both old and new have the needs-drain annotation set", func() {
				oldNode := node.DeepCopy()
				oldNode.Annotations = map[string]string{v1beta1constants.AnnotationNodeAgentInPlaceUpdateNeedsDrain: "true"}
				newNode := oldNode.DeepCopy()

				Expect(p.Update(event.UpdateEvent{ObjectOld: oldNode, ObjectNew: newNode})).To(BeFalse())
			})
		})

		Describe("#Delete", func() {
			It("should return false", func() {
				Expect(p.Delete(event.DeleteEvent{Object: node})).To(BeFalse())
			})
		})

		Describe("#Generic", func() {
			It("should return false", func() {
				Expect(p.Generic(event.GenericEvent{Object: node})).To(BeFalse())
			})
		})
	})
})

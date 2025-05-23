// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package criticalcomponents_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	. "github.com/gardener/gardener/pkg/resourcemanager/controller/node/criticalcomponents"
)

var _ = Describe("Add", func() {
	Describe("#NodePredicate", func() {
		var (
			p    predicate.Predicate
			node *corev1.Node
		)

		BeforeEach(func() {
			p = (&Reconciler{}).NodePredicate()
			node = &corev1.Node{}
		})

		Describe("#Create", func() {
			It("should return false if the object is not a Node", func() {
				Expect(p.Create(event.CreateEvent{Object: &corev1.ConfigMap{}})).To(BeFalse())
			})

			It("should return false if Node doesn't have any taints", func() {
				Expect(p.Create(event.CreateEvent{Object: node})).To(BeFalse())
			})

			It("should return false if Node doesn't have critical components taint", func() {
				node.Spec.Taints = []corev1.Taint{{
					Key:    "node.kubernetes.io/not-ready",
					Effect: "NoExecute",
				}, {
					Key:    "other-taint",
					Effect: "NoSchedule",
				}}

				Expect(p.Create(event.CreateEvent{Object: node})).To(BeFalse())
			})

			It("should return true if Node has critical components taint", func() {
				node.Spec.Taints = []corev1.Taint{{
					Key:    "node.gardener.cloud/critical-components-not-ready",
					Effect: "NoExecute",
				}}

				Expect(p.Create(event.CreateEvent{Object: node})).To(BeTrue())
			})
		})

		Describe("#Update", func() {
			It("should return false if the object is not a Node", func() {
				Expect(p.Update(event.UpdateEvent{ObjectNew: &corev1.ConfigMap{}})).To(BeFalse())
			})

			It("should return false if Node doesn't have any taints", func() {
				Expect(p.Update(event.UpdateEvent{ObjectNew: node})).To(BeFalse())
			})

			It("should return false if Node doesn't have critical components taint", func() {
				node.Spec.Taints = []corev1.Taint{{
					Key:    "node.kubernetes.io/not-ready",
					Effect: "NoExecute",
				}, {
					Key:    "other-taint",
					Effect: "NoSchedule",
				}}

				Expect(p.Update(event.UpdateEvent{ObjectNew: node})).To(BeFalse())
			})

			It("should return true if Node has critical components taint", func() {
				node.Spec.Taints = []corev1.Taint{{
					Key:    "node.gardener.cloud/critical-components-not-ready",
					Effect: "NoExecute",
				}}

				Expect(p.Update(event.UpdateEvent{ObjectNew: node})).To(BeTrue())
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

	Describe("#NodeHasCriticalComponentsNotReadyTaint", func() {
		var node *corev1.Node

		BeforeEach(func() {
			node = &corev1.Node{}
		})

		It("should return false if the object is not a Node", func() {
			Expect(NodeHasCriticalComponentsNotReadyTaint(&corev1.ConfigMap{})).To(BeFalse())
		})

		It("should return false if Node doesn't have any taints", func() {
			Expect(NodeHasCriticalComponentsNotReadyTaint(node)).To(BeFalse())
		})

		It("should return false if Node doesn't have critical components taint", func() {
			node.Spec.Taints = []corev1.Taint{{
				Key:    "node.kubernetes.io/not-ready",
				Effect: "NoExecute",
			}, {
				Key:    "other-taint",
				Effect: "NoSchedule",
			}}

			Expect(NodeHasCriticalComponentsNotReadyTaint(node)).To(BeFalse())
		})

		It("should return true if Node has critical components taint", func() {
			node.Spec.Taints = []corev1.Taint{{
				Key:    "node.gardener.cloud/critical-components-not-ready",
				Effect: "NoExecute",
			}}

			Expect(NodeHasCriticalComponentsNotReadyTaint(node)).To(BeTrue())
		})
	})
})

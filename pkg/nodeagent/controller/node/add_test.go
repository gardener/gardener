// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package node_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	. "github.com/gardener/gardener/pkg/nodeagent/controller/node"
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
			It("should return false because annotation is not present", func() {
				Expect(p.Create(event.CreateEvent{Object: node})).To(BeFalse())
			})

			It("should return true because annotation is present", func() {
				metav1.SetMetaDataAnnotation(&node.ObjectMeta, "worker.gardener.cloud/restart-systemd-services", "foo")
				Expect(p.Create(event.CreateEvent{Object: node})).To(BeTrue())
			})
		})

		Describe("#Update", func() {
			It("should return false because annotation is not present", func() {
				Expect(p.Update(event.UpdateEvent{ObjectOld: node, ObjectNew: node})).To(BeFalse())
			})

			It("should return true because annotation got set", func() {
				oldNode := node.DeepCopy()
				metav1.SetMetaDataAnnotation(&node.ObjectMeta, "worker.gardener.cloud/restart-systemd-services", "foo")
				Expect(p.Update(event.UpdateEvent{ObjectOld: oldNode, ObjectNew: node})).To(BeTrue())
			})

			It("should return true because annotation got changed", func() {
				metav1.SetMetaDataAnnotation(&node.ObjectMeta, "worker.gardener.cloud/restart-systemd-services", "foo")
				oldNode := node.DeepCopy()
				metav1.SetMetaDataAnnotation(&node.ObjectMeta, "worker.gardener.cloud/restart-systemd-services", "bar")
				Expect(p.Update(event.UpdateEvent{ObjectOld: oldNode, ObjectNew: node})).To(BeTrue())
			})

			It("should return false because annotation got removed", func() {
				oldNode := node.DeepCopy()
				metav1.SetMetaDataAnnotation(&oldNode.ObjectMeta, "worker.gardener.cloud/restart-systemd-services", "foo")
				Expect(p.Update(event.UpdateEvent{ObjectOld: oldNode, ObjectNew: node})).To(BeFalse())
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

// Copyright (c) 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package node_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	. "github.com/gardener/gardener/pkg/resourcemanager/controller/node"
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
			It("should return false", func() {
				Expect(p.Update(event.UpdateEvent{})).To(BeFalse())
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

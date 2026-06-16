// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package selfhostedshootexposure_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	. "github.com/gardener/gardener/pkg/gardenlet/controller/shoot/selfhostedshootexposure"
)

var _ = Describe("NodePredicate", func() {
	const controlPlaneLabel = "node-role.kubernetes.io/control-plane"

	var p predicate.Predicate

	node := func(controlPlane bool, ready bool, addresses ...corev1.NodeAddress) *corev1.Node {
		n := &corev1.Node{Status: corev1.NodeStatus{Addresses: addresses}}
		if controlPlane {
			n.Labels = map[string]string{controlPlaneLabel: ""}
		}
		status := corev1.ConditionFalse
		if ready {
			status = corev1.ConditionTrue
		}
		n.Status.Conditions = []corev1.NodeCondition{{Type: corev1.NodeReady, Status: status}}
		return n
	}
	ip := func(address string) corev1.NodeAddress {
		return corev1.NodeAddress{Type: corev1.NodeInternalIP, Address: address}
	}

	BeforeEach(func() {
		p = (&Reconciler{}).NodePredicate()
	})

	Describe("Create/Delete", func() {
		It("should accept a control-plane node and reject a worker node", func() {
			cp, worker := node(true, true, ip("10.0.0.1")), node(false, true, ip("10.0.0.2"))

			Expect(p.Create(event.CreateEvent{Object: cp})).To(BeTrue())
			Expect(p.Create(event.CreateEvent{Object: worker})).To(BeFalse())
			Expect(p.Delete(event.DeleteEvent{Object: cp})).To(BeTrue())
			Expect(p.Delete(event.DeleteEvent{Object: worker})).To(BeFalse())
		})
	})

	Describe("Update", func() {
		It("should accept a promotion (label added after registration)", func() {
			// Same Ready node, only the control-plane label is added — the regression case.
			Expect(p.Update(event.UpdateEvent{
				ObjectOld: node(false, true, ip("10.0.0.1")),
				ObjectNew: node(true, true, ip("10.0.0.1")),
			})).To(BeTrue())
		})

		It("should accept a demotion (label removed)", func() {
			Expect(p.Update(event.UpdateEvent{
				ObjectOld: node(true, true, ip("10.0.0.1")),
				ObjectNew: node(false, true, ip("10.0.0.1")),
			})).To(BeTrue())
		})

		It("should accept an address change on a control-plane node", func() {
			Expect(p.Update(event.UpdateEvent{
				ObjectOld: node(true, true, ip("10.0.0.1")),
				ObjectNew: node(true, true, ip("10.0.0.2")),
			})).To(BeTrue())
		})

		It("should accept a health-verdict change on a control-plane node", func() {
			Expect(p.Update(event.UpdateEvent{
				ObjectOld: node(true, true, ip("10.0.0.1")),
				ObjectNew: node(true, false, ip("10.0.0.1")),
			})).To(BeTrue())
		})

		It("should reject an irrelevant change on a control-plane node", func() {
			old := node(true, true, ip("10.0.0.1"))
			updated := node(true, true, ip("10.0.0.1"))
			updated.Annotations = map[string]string{"foo": "bar"}

			Expect(p.Update(event.UpdateEvent{ObjectOld: old, ObjectNew: updated})).To(BeFalse())
		})

		It("should reject any change on a node that is not a control-plane node before or after", func() {
			Expect(p.Update(event.UpdateEvent{
				ObjectOld: node(false, true, ip("10.0.0.1")),
				ObjectNew: node(false, false, ip("10.0.0.2")),
			})).To(BeFalse())
		})

		It("should accept updates with non-Node objects (defensive)", func() {
			Expect(p.Update(event.UpdateEvent{ObjectOld: &corev1.Pod{}, ObjectNew: &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "x"}}})).To(BeTrue())
		})
	})
})

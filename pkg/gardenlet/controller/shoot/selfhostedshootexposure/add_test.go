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

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/gardener/gardener/pkg/gardenlet/controller/shoot/selfhostedshootexposure"
)

var _ = Describe("NodePredicate", func() {
	const controlPlaneLabel = v1beta1constants.LabelNodeRoleControlPlane

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

		It("should reject updates with non-Node objects", func() {
			Expect(p.Update(event.UpdateEvent{ObjectOld: &corev1.Pod{}, ObjectNew: &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "x"}}})).To(BeFalse())
		})
	})
})

var _ = Describe("ShootExposureChangePredicate", func() {
	var p predicate.Predicate

	shoot := func(exposure *gardencorev1beta1.Exposure) *gardencorev1beta1.Shoot {
		return &gardencorev1beta1.Shoot{Spec: gardencorev1beta1.ShootSpec{Provider: gardencorev1beta1.Provider{Workers: []gardencorev1beta1.Worker{
			{ControlPlane: &gardencorev1beta1.WorkerControlPlane{Exposure: exposure}},
		}}}}
	}
	dnsExposure := &gardencorev1beta1.Exposure{DNS: &gardencorev1beta1.DNSExposure{}}
	extensionExposure := &gardencorev1beta1.Exposure{Extension: &gardencorev1beta1.ExtensionExposure{Type: new("local")}}

	BeforeEach(func() {
		p = (&Reconciler{}).ShootExposureChangePredicate()
	})

	It("should accept creates so the initial state is reconciled", func() {
		Expect(p.Create(event.CreateEvent{Object: shoot(dnsExposure)})).To(BeTrue())
	})

	It("should reject deletes", func() {
		Expect(p.Delete(event.DeleteEvent{Object: shoot(dnsExposure)})).To(BeFalse())
	})

	It("should reject generic events", func() {
		Expect(p.Generic(event.GenericEvent{Object: shoot(dnsExposure)})).To(BeFalse())
	})

	It("should accept an exposure mechanism switch", func() {
		Expect(p.Update(event.UpdateEvent{ObjectOld: shoot(extensionExposure), ObjectNew: shoot(dnsExposure)})).To(BeTrue())
	})

	It("should accept exposure being removed", func() {
		Expect(p.Update(event.UpdateEvent{ObjectOld: shoot(extensionExposure), ObjectNew: shoot(nil)})).To(BeTrue())
	})

	It("should reject an unrelated Shoot update", func() {
		old, updated := shoot(extensionExposure), shoot(extensionExposure)
		updated.Labels = map[string]string{"foo": "bar"}
		Expect(p.Update(event.UpdateEvent{ObjectOld: old, ObjectNew: updated})).To(BeFalse())
	})
})

var _ = Describe("ExposureIngressChangePredicate", func() {
	var p predicate.Predicate

	exposure := func(ingress ...corev1.LoadBalancerIngress) *extensionsv1alpha1.SelfHostedShootExposure {
		return &extensionsv1alpha1.SelfHostedShootExposure{Status: extensionsv1alpha1.SelfHostedShootExposureStatus{Ingress: ingress}}
	}

	BeforeEach(func() {
		p = (&Reconciler{}).ExposureIngressChangePredicate()
	})

	It("should reject creates, deletes and generic events", func() {
		Expect(p.Create(event.CreateEvent{Object: exposure()})).To(BeFalse())
		Expect(p.Delete(event.DeleteEvent{Object: exposure()})).To(BeFalse())
		Expect(p.Generic(event.GenericEvent{Object: exposure()})).To(BeFalse())
	})

	It("should accept an ingress change", func() {
		Expect(p.Update(event.UpdateEvent{
			ObjectOld: exposure(corev1.LoadBalancerIngress{IP: "1.2.3.4"}),
			ObjectNew: exposure(corev1.LoadBalancerIngress{IP: "1.2.3.5"}),
		})).To(BeTrue())
	})

	It("should accept the ingress first appearing", func() {
		Expect(p.Update(event.UpdateEvent{
			ObjectOld: exposure(),
			ObjectNew: exposure(corev1.LoadBalancerIngress{IP: "1.2.3.4"}),
		})).To(BeTrue())
	})

	It("should reject an update that does not change the ingress", func() {
		Expect(p.Update(event.UpdateEvent{
			ObjectOld: exposure(corev1.LoadBalancerIngress{IP: "1.2.3.4"}),
			ObjectNew: exposure(corev1.LoadBalancerIngress{IP: "1.2.3.4"}),
		})).To(BeFalse())
	})
})

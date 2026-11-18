// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package etcdmember_test

import (
	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"

	"github.com/gardener/gardener/pkg/gardenlet/controller/shoot/etcdmember"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

var _ = Describe("Predicates", func() {
	var (
		reconciler *etcdmember.Reconciler
		machine    *machinev1alpha1.Machine
	)

	BeforeEach(func() {
		reconciler = &etcdmember.Reconciler{}
		machine = &machinev1alpha1.Machine{}
	})

	Describe("#ControlPlaneMachinePredicate", func() {
		var pred = func() bool {
			return reconciler.ControlPlaneMachinePredicate().Create(event.CreateEvent{Object: machine})
		}

		It("returns false when annotation is absent", func() {
			Expect(pred()).To(BeFalse())
		})

		It("returns false when annotation has wrong value", func() {
			machine.Annotations = map[string]string{gardenerutils.AnnotationKeySuspendInstanceDeletion: "other"}
			Expect(pred()).To(BeFalse())
		})

		It("returns true when annotation has expected value", func() {
			machine.Annotations = map[string]string{gardenerutils.AnnotationKeySuspendInstanceDeletion: gardenerutils.AnnotationValueSuspendInstanceDeletion}
			Expect(pred()).To(BeTrue())
		})

		It("returns the same result for Update, Delete, and Generic as for Create", func() {
			machine.Annotations = map[string]string{gardenerutils.AnnotationKeySuspendInstanceDeletion: gardenerutils.AnnotationValueSuspendInstanceDeletion}
			p := reconciler.ControlPlaneMachinePredicate()
			Expect(p.Update(event.UpdateEvent{ObjectNew: machine})).To(BeTrue())
			Expect(p.Delete(event.DeleteEvent{Object: machine})).To(BeTrue())
			Expect(p.Generic(event.GenericEvent{Object: machine})).To(BeTrue())
		})
	})

	Describe("#MachineAddressesPredicate", func() {
		var (
			oldMachine *machinev1alpha1.Machine
			newMachine *machinev1alpha1.Machine
		)

		BeforeEach(func() {
			oldMachine = &machinev1alpha1.Machine{}
			newMachine = &machinev1alpha1.Machine{}
		})

		It("always returns true on Create", func() {
			Expect(reconciler.MachineAddressesPredicate().Create(event.CreateEvent{})).To(BeTrue())
		})

		It("always returns false on Delete", func() {
			Expect(reconciler.MachineAddressesPredicate().Delete(event.DeleteEvent{})).To(BeFalse())
		})

		It("always returns false on Generic", func() {
			Expect(reconciler.MachineAddressesPredicate().Generic(event.GenericEvent{})).To(BeFalse())
		})

		Context("Update", func() {
			update := func() bool {
				return reconciler.MachineAddressesPredicate().Update(event.UpdateEvent{ObjectOld: oldMachine, ObjectNew: newMachine})
			}

			It("returns false when both old and new have no addresses", func() {
				Expect(update()).To(BeFalse())
			})

			It("returns false when old already had addresses", func() {
				oldMachine.Status.Addresses = []corev1.NodeAddress{{Type: corev1.NodeInternalIP, Address: "1.2.3.4"}}
				newMachine.Status.Addresses = []corev1.NodeAddress{{Type: corev1.NodeInternalIP, Address: "1.2.3.5"}}
				Expect(update()).To(BeFalse())
			})

			It("returns false when new has no addresses", func() {
				oldMachine.Status.Addresses = nil
				newMachine.Status.Addresses = nil
				Expect(update()).To(BeFalse())
			})

			It("returns true when old had no addresses and new has at least one", func() {
				newMachine.Status.Addresses = []corev1.NodeAddress{{Type: corev1.NodeInternalIP, Address: "10.0.0.1"}}
				Expect(update()).To(BeTrue())
			})
		})
	})

	Describe("#InstanceDeletionSuspendedPredicate", func() {
		var (
			oldMachine *machinev1alpha1.Machine
			newMachine *machinev1alpha1.Machine
		)

		BeforeEach(func() {
			oldMachine = &machinev1alpha1.Machine{}
			newMachine = &machinev1alpha1.Machine{}
		})

		It("always returns true on Create", func() {
			Expect(reconciler.InstanceDeletionSuspendedPredicate().Create(event.CreateEvent{})).To(BeTrue())
		})

		It("always returns false on Delete", func() {
			Expect(reconciler.InstanceDeletionSuspendedPredicate().Delete(event.DeleteEvent{})).To(BeFalse())
		})

		It("always returns false on Generic", func() {
			Expect(reconciler.InstanceDeletionSuspendedPredicate().Generic(event.GenericEvent{})).To(BeFalse())
		})

		Context("Update", func() {
			withCondition := func(m *machinev1alpha1.Machine, status corev1.ConditionStatus) {
				m.Status.Conditions = []corev1.NodeCondition{
					// TODO(rfranzke): Use machinev1alpha1.ConditionInstanceDeletionSuspended once MCM is revendored.
					{Type: "InstanceDeletionSuspended", Status: status},
				}
			}

			update := func() bool {
				return reconciler.InstanceDeletionSuspendedPredicate().Update(event.UpdateEvent{ObjectOld: oldMachine, ObjectNew: newMachine})
			}

			It("returns false when condition is absent on both", func() {
				Expect(update()).To(BeFalse())
			})

			It("returns false when condition was already True in old", func() {
				withCondition(oldMachine, corev1.ConditionTrue)
				withCondition(newMachine, corev1.ConditionTrue)
				Expect(update()).To(BeFalse())
			})

			It("returns false when condition is False in new", func() {
				withCondition(newMachine, corev1.ConditionFalse)
				Expect(update()).To(BeFalse())
			})

			It("returns false when condition transitions from True to False", func() {
				withCondition(oldMachine, corev1.ConditionTrue)
				withCondition(newMachine, corev1.ConditionFalse)
				Expect(update()).To(BeFalse())
			})

			It("returns true when condition transitions from absent to True", func() {
				withCondition(newMachine, corev1.ConditionTrue)
				Expect(update()).To(BeTrue())
			})

			It("returns true when condition transitions from False to True", func() {
				withCondition(oldMachine, corev1.ConditionFalse)
				withCondition(newMachine, corev1.ConditionTrue)
				Expect(update()).To(BeTrue())
			})
		})
	})
})

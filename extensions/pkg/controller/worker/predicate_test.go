// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package worker_test

import (
	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"

	"github.com/gardener/gardener/extensions/pkg/controller/worker"
)

var _ = Describe("Worker Predicates", func() {
	Describe("#MachineNodeInfoHasChanged", func() {
		var (
			oldMachine   *machinev1alpha1.Machine
			newMachine   *machinev1alpha1.Machine
			createEvent  event.CreateEvent
			updateEvent  event.UpdateEvent
			deleteEvent  event.DeleteEvent
			genericEvent event.GenericEvent
		)

		BeforeEach(func() {
			oldMachine = &machinev1alpha1.Machine{}
			newMachine = &machinev1alpha1.Machine{}

			createEvent = event.CreateEvent{
				Object: newMachine,
			}
			updateEvent = event.UpdateEvent{
				ObjectOld: oldMachine,
				ObjectNew: newMachine,
			}
			deleteEvent = event.DeleteEvent{
				Object: newMachine,
			}
			genericEvent = event.GenericEvent{
				Object: newMachine,
			}
		})

		It("should notice the change of the provider id", func() {
			predicate := worker.MachineNodeInfoHasChanged()
			newMachine.Spec.ProviderID = "foo"
			Expect(predicate.Create(createEvent)).To(BeTrue())
			Expect(predicate.Update(updateEvent)).To(BeTrue())
			Expect(predicate.Delete(deleteEvent)).To(BeTrue())
			Expect(predicate.Generic(genericEvent)).To(BeFalse())
		})

		It("should notice the change of the node label", func() {
			predicate := worker.MachineNodeInfoHasChanged()
			metav1.SetMetaDataLabel(&newMachine.ObjectMeta, "node", "ip.10-256-18-291.cluster.node")
			Expect(predicate.Create(createEvent)).To(BeTrue())
			Expect(predicate.Update(updateEvent)).To(BeTrue())
			Expect(predicate.Delete(deleteEvent)).To(BeTrue())
			Expect(predicate.Generic(genericEvent)).To(BeFalse())
		})

		It("should not react when there are no changes of the node label", func() {
			predicate := worker.MachineNodeInfoHasChanged()
			metav1.SetMetaDataLabel(&oldMachine.ObjectMeta, "node", "ip.10-256-18-291.cluster.node")
			metav1.SetMetaDataLabel(&newMachine.ObjectMeta, "node", "ip.10-256-18-291.cluster.node")
			Expect(predicate.Create(createEvent)).To(BeTrue())
			Expect(predicate.Update(updateEvent)).To(BeFalse())
			Expect(predicate.Delete(deleteEvent)).To(BeTrue())
			Expect(predicate.Generic(genericEvent)).To(BeFalse())
		})
		It("should not react when there is not specified node label", func() {
			predicate := worker.MachineNodeInfoHasChanged()
			Expect(predicate.Create(createEvent)).To(BeTrue())
			Expect(predicate.Update(updateEvent)).To(BeFalse())
			Expect(predicate.Delete(deleteEvent)).To(BeTrue())
			Expect(predicate.Generic(genericEvent)).To(BeFalse())
		})
	})
})

// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

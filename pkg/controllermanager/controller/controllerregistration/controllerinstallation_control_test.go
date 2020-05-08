// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package controllerregistration

import (
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("Controller", func() {
	var (
		queue *fakeQueue
		c     *Controller

		seedName = "seed"
	)

	BeforeEach(func() {
		queue = &fakeQueue{}
		c = &Controller{
			controllerRegistrationSeedQueue: queue,
		}
	})

	Describe("#controllerInstallationAdd", func() {
		It("should do nothing because object is not a ControllerInstallation", func() {
			obj := &gardencorev1beta1.CloudProfile{}

			c.controllerInstallationAdd(obj)

			Expect(queue.Len()).To(BeZero())
		})

		It("should add the object to the queue", func() {
			obj := &gardencorev1beta1.ControllerInstallation{
				Spec: gardencorev1beta1.ControllerInstallationSpec{
					SeedRef: corev1.ObjectReference{
						Name: seedName,
					},
				},
			}

			c.controllerInstallationAdd(obj)

			Expect(queue.Len()).To(Equal(1))
			Expect(queue.items[0]).To(Equal(seedName))
		})
	})

	Describe("#controllerInstallationUpdate", func() {
		It("should do nothing because old object is not a ControllerInstallation", func() {
			oldObj := &gardencorev1beta1.CloudProfile{}
			newObj := &gardencorev1beta1.ControllerInstallation{}

			c.controllerInstallationUpdate(oldObj, newObj)

			Expect(queue.Len()).To(BeZero())
		})

		It("should do nothing because new object is not a ControllerInstallation", func() {
			oldObj := &gardencorev1beta1.ControllerInstallation{}
			newObj := &gardencorev1beta1.CloudProfile{}

			c.controllerInstallationUpdate(oldObj, newObj)

			Expect(queue.Len()).To(BeZero())
		})

		It("should do nothing because nothing changed", func() {
			oldObj := &gardencorev1beta1.ControllerInstallation{}
			newObj := &gardencorev1beta1.ControllerInstallation{}

			c.controllerInstallationUpdate(oldObj, newObj)

			Expect(queue.Len()).To(BeZero())
		})

		It("should add the new obj to the queue because required condition changed", func() {
			oldObj := &gardencorev1beta1.ControllerInstallation{}
			newObj := &gardencorev1beta1.ControllerInstallation{
				Spec: gardencorev1beta1.ControllerInstallationSpec{
					SeedRef: corev1.ObjectReference{
						Name: seedName,
					},
				},
				Status: gardencorev1beta1.ControllerInstallationStatus{
					Conditions: []gardencorev1beta1.Condition{
						{
							Type:   gardencorev1beta1.ControllerInstallationRequired,
							Status: gardencorev1beta1.ConditionTrue,
						},
					},
				},
			}

			c.controllerInstallationUpdate(oldObj, newObj)

			Expect(queue.Len()).To(Equal(1))
			Expect(queue.items[0]).To(Equal(seedName))
		})

		It("should not add the new obj to the queue because required condition unchanged", func() {
			oldObj := &gardencorev1beta1.ControllerInstallation{}
			newObj := &gardencorev1beta1.ControllerInstallation{}

			c.controllerInstallationUpdate(oldObj, newObj)

			Expect(queue.Len()).To(BeZero())
		})
	})
})

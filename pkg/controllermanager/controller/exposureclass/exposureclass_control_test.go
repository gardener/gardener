// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package exposureclass

import (
	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	"github.com/gardener/gardener/pkg/logger"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Controller", func() {
	var (
		queue *fakeQueue
		c     *Controller
	)

	BeforeEach(func() {
		logger.Logger = logger.NewNopLogger()
		queue = &fakeQueue{}
		c = &Controller{
			exposureClassQueue: queue,
		}
	})

	Describe("#exposureClassAdd", func() {
		It("should add object to the queue", func() {
			exposureClass := &gardencorev1alpha1.ExposureClass{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
			}
			c.exposureClassAdd(exposureClass)

			Expect(c.exposureClassQueue.Len()).To(Equal(1))
			Expect(queue.items[0]).To(Equal(exposureClass.Name))
		})

		It("should not add object of invalid type to the queue", func() {
			c.exposureClassAdd("invalid-type-object")
			Expect(c.exposureClassQueue.Len()).To(BeZero())
		})
	})

	Describe("#exposureClassUpdate", func() {
		It("should add object to the queue", func() {
			var (
				oldObj = &gardencorev1alpha1.ExposureClass{
					ObjectMeta: metav1.ObjectMeta{Name: "test"},
				}
				newObj = &gardencorev1alpha1.ExposureClass{
					ObjectMeta: metav1.ObjectMeta{Name: "test"},
				}
			)
			c.exposureClassUpdate(oldObj, newObj)

			Expect(c.exposureClassQueue.Len()).To(Equal(1))
			Expect(queue.items[0]).To(Equal(newObj.Name))
		})

		It("should not add object of invalid type to the queue", func() {
			var (
				oldObj = &gardencorev1alpha1.ExposureClass{}
				newObj = "invalid-type-object"
			)
			c.exposureClassUpdate(oldObj, newObj)

			Expect(c.exposureClassQueue.Len()).To(BeZero())
		})
	})

	Describe("#exposureClassDelete", func() {
		It("should add object to the queue", func() {
			exposureClass := &gardencorev1alpha1.ExposureClass{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
			}
			c.exposureClassDelete(exposureClass)
			Expect(c.exposureClassQueue.Len()).To(Equal(1))
			Expect(queue.items[0]).To(Equal(exposureClass.Name))
		})

		It("should do nothing as it can not get key for invalid type object", func() {
			c.exposureClassDelete("invalid-type-object")
			Expect(c.exposureClassQueue.Len()).To(BeZero())
		})
	})
})

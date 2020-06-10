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

package project

import (
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/logger"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Controller", func() {
	var (
		queue1, queue2 *fakeQueue
		c              *Controller

		projectName = "foo"
	)

	BeforeEach(func() {
		logger.Logger = logger.NewNopLogger()
		queue1 = &fakeQueue{}
		queue2 = &fakeQueue{}
		c = &Controller{
			projectQueue:      queue1,
			projectStaleQueue: queue2,
		}
	})

	Describe("#projectAdd", func() {
		It("should do nothing because it cannot compute the object key", func() {
			c.projectAdd("foo")

			Expect(queue1.Len()).To(BeZero())
		})

		It("should add the object to the projectQueue and projectStaleQueue", func() {
			obj := &gardencorev1beta1.Project{
				ObjectMeta: metav1.ObjectMeta{Name: projectName},
			}

			c.projectAdd(obj)

			Expect(queue1.Len()).To(Equal(1))
			Expect(queue1.items[0]).To(Equal(projectName))
			Expect(queue2.Len()).To(Equal(1))
			Expect(queue2.items[0]).To(Equal(projectName))
		})
	})

	Describe("#projectUpdate", func() {
		It("should do nothing because new object is not a Project", func() {
			oldObj := &gardencorev1beta1.Project{}
			newObj := &gardencorev1beta1.CloudProfile{}

			c.projectUpdate(oldObj, newObj)

			Expect(queue1.Len()).To(BeZero())
		})

		It("should do nothing because generation is equal observed generation", func() {
			oldObj := &gardencorev1beta1.Project{}
			newObj := &gardencorev1beta1.Project{
				ObjectMeta: metav1.ObjectMeta{Generation: 42},
				Status:     gardencorev1beta1.ProjectStatus{ObservedGeneration: 42},
			}

			c.projectUpdate(oldObj, newObj)

			Expect(queue1.Len()).To(BeZero())
		})

		It("should add the new obj to the projectQueue because generation is not equal observed generation", func() {
			oldObj := &gardencorev1beta1.Project{}
			newObj := &gardencorev1beta1.Project{
				ObjectMeta: metav1.ObjectMeta{Name: projectName, Generation: 43},
				Status:     gardencorev1beta1.ProjectStatus{ObservedGeneration: 42},
			}

			c.projectUpdate(oldObj, newObj)

			Expect(queue1.Len()).To(Equal(1))
			Expect(queue1.items[0]).To(Equal(projectName))
		})
	})

	Describe("#projectDelete", func() {
		It("should do nothing because it cannot compute the object key", func() {
			c.projectDelete("foo")

			Expect(queue1.Len()).To(BeZero())
		})

		It("should add the object to the projectQueue", func() {
			obj := &gardencorev1beta1.Project{
				ObjectMeta: metav1.ObjectMeta{Name: projectName},
			}

			c.projectDelete(obj)

			Expect(queue1.Len()).To(Equal(1))
			Expect(queue1.items[0]).To(Equal(projectName))
		})
	})
})

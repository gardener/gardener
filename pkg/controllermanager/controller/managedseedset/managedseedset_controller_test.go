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

package managedseedset

import (
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	mockworkqueue "github.com/gardener/gardener/pkg/mock/client-go/util/workqueue"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

const (
	namespace = "test"
	name      = "garden"
	key       = namespace + "/" + name
)

var _ = Describe("Controller", func() {
	var (
		ctrl *gomock.Controller

		queue *mockworkqueue.MockRateLimitingInterface

		c *Controller

		set *seedmanagementv1alpha1.ManagedSeedSet
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		queue = mockworkqueue.NewMockRateLimitingInterface(ctrl)

		c = &Controller{
			managedSeedSetQueue: queue,
		}

		set = &seedmanagementv1alpha1.ManagedSeedSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:       name,
				Namespace:  namespace,
				Generation: 1,
			},
		}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#managedSeedSetAdd", func() {
		It("should do nothing because the object is not a ManagedSeedSet", func() {
			c.managedSeedSetAdd(&gardencorev1beta1.Seed{})
		})

		It("should add the object to the queue", func() {
			queue.EXPECT().Add(key)

			c.managedSeedSetAdd(set)
		})
	})

	Describe("#managedSeedSetUpdate", func() {
		It("should do nothing because the new object is not a ManagedSeedSet", func() {
			c.managedSeedSetUpdate(nil, &gardencorev1beta1.Seed{})
		})

		It("should do nothing because the new object generation and observed generation are equal", func() {
			set.Status.ObservedGeneration = 1

			c.managedSeedSetUpdate(nil, set)
		})

		It("should add the object to the queue", func() {
			queue.EXPECT().Add(key)

			c.managedSeedSetUpdate(nil, set)
		})
	})

	Describe("#managedSeedSetDelete", func() {
		It("should do nothing because the object is not a ManagedSeedSet or a tombstone", func() {
			c.managedSeedSetDelete(&gardencorev1beta1.Seed{})
		})

		It("should do nothing because the object is a tombstone of something else than a ManagedSeedSet", func() {
			c.managedSeedSetDelete(cache.DeletedFinalStateUnknown{Key: key, Obj: &gardencorev1beta1.Seed{}})
		})

		It("should add the object to the queue", func() {
			queue.EXPECT().Add(key)

			c.managedSeedSetDelete(set)
		})

		It("should add the object to the queue", func() {
			queue.EXPECT().Add(key)

			c.managedSeedSetDelete(cache.DeletedFinalStateUnknown{Key: key, Obj: set})
		})
	})
})

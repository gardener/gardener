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

package managedseed

import (
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	gardenerlogger "github.com/gardener/gardener/pkg/logger"
	mockworkqueue "github.com/gardener/gardener/pkg/mock/client-go/util/workqueue"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/pointer"
)

const (
	key              = namespace + "/" + name
	syncJitterPeriod = 5 * time.Minute
)

var _ = Describe("Controller", func() {
	var (
		ctrl *gomock.Controller

		queue *mockworkqueue.MockRateLimitingInterface

		c *Controller

		managedSeed *seedmanagementv1alpha1.ManagedSeed
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		queue = mockworkqueue.NewMockRateLimitingInterface(ctrl)

		c = &Controller{
			managedSeedQueue: queue,
			config: &config.GardenletConfiguration{
				Controllers: &config.GardenletControllerConfiguration{
					ManagedSeed: &config.ManagedSeedControllerConfiguration{
						SyncJitterPeriod: &metav1.Duration{Duration: syncJitterPeriod},
						JitterUpdates:    pointer.Bool(false),
					},
				},
			},
			logger: gardenerlogger.NewNopLogger(),
		}

		managedSeed = &seedmanagementv1alpha1.ManagedSeed{
			ObjectMeta: metav1.ObjectMeta{
				Name:       name,
				Namespace:  namespace,
				Generation: 2,
			},
		}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#managedSeedAdd", func() {
		It("should do nothing because the object is not a ManagedSeed", func() {
			c.managedSeedAdd(&gardencorev1beta1.Seed{})
		})

		It("should add the object to the queue", func() {
			queue.EXPECT().Add(key)

			c.managedSeedAdd(managedSeed)
		})

		It("should add the object to the queue (deletion)", func() {
			now := metav1.Now()
			managedSeed.DeletionTimestamp = &now
			queue.EXPECT().Add(key)

			c.managedSeedAdd(managedSeed)
		})

		It("should add new object to the queue without delay", func() {
			managedSeed.Status.ObservedGeneration = 1
			queue.EXPECT().Add(key)

			c.managedSeedAdd(managedSeed)
		})

		It("should add the object to the queue with a jittered delay because object generation and observed generation are not equal and jitterUpdates is set to true", func() {
			managedSeed.Status.ObservedGeneration = 3
			c.config.Controllers.ManagedSeed.JitterUpdates = pointer.Bool(true)
			queue.EXPECT().AddAfter(key, gomock.AssignableToTypeOf(time.Second)).DoAndReturn(
				func(_ interface{}, d time.Duration) {
					Expect(d > 0 && d <= syncJitterPeriod).To(BeTrue())
				},
			)

			c.managedSeedAdd(managedSeed)
		})

		It("should add the object to the queue without jittered delay because object generation and observed generation are not equal and jitterUpdates is set to false", func() {
			managedSeed.Status.ObservedGeneration = 3
			c.config.Controllers.ManagedSeed.JitterUpdates = pointer.Bool(false)
			queue.EXPECT().Add(key)
			c.managedSeedAdd(managedSeed)
		})

		It("should add the object to the queue with a jittered delay because the object generation and observed generation are equal", func() {
			managedSeed.Status.ObservedGeneration = 2

			queue.EXPECT().AddAfter(key, gomock.AssignableToTypeOf(time.Second)).DoAndReturn(
				func(_ interface{}, d time.Duration) {
					Expect(d > 0 && d <= syncJitterPeriod).To(BeTrue())
				},
			)

			c.managedSeedAdd(managedSeed)
		})

	})

	Describe("#managedSeedUpdate", func() {
		It("should do nothing because the new object is not a ManagedSeed", func() {
			c.managedSeedUpdate(nil, &gardencorev1beta1.Seed{})
		})

		It("should add the object to the queue", func() {
			queue.EXPECT().Add(key)

			c.managedSeedUpdate(nil, managedSeed)
		})

		It("should do nothing because the object generation and observed generation are equal", func() {
			managedSeed.Status.ObservedGeneration = 2

			c.managedSeedUpdate(nil, managedSeed)
		})

		It("should add the object to the queue with a jittered delay because object generation and observed generation are not equal and jitterUpdates is set to true", func() {
			managedSeed.Status.ObservedGeneration = 3
			c.config.Controllers.ManagedSeed.JitterUpdates = pointer.Bool(true)
			queue.EXPECT().AddAfter(key, gomock.AssignableToTypeOf(time.Second)).DoAndReturn(
				func(_ interface{}, d time.Duration) {
					Expect(d > 0 && d <= syncJitterPeriod).To(BeTrue())
				},
			)

			c.managedSeedUpdate(nil, managedSeed)
		})

		It("should add the object to the queue without jittered delay because object generation and observed generation are not equal and jitterUpdates is set to false", func() {
			managedSeed.Status.ObservedGeneration = 3
			c.config.Controllers.ManagedSeed.JitterUpdates = pointer.Bool(false)
			queue.EXPECT().Add(key)
			c.managedSeedUpdate(nil, managedSeed)
		})

	})

	Describe("#managedSeedDelete", func() {
		It("should do nothing because the object is not a ManagedSeed or a tombstone", func() {
			c.managedSeedDelete(&gardencorev1beta1.Seed{})
		})

		It("should do nothing because the object is a tombstone of something else than a ManagedSeed", func() {
			c.managedSeedDelete(cache.DeletedFinalStateUnknown{Key: key, Obj: &gardencorev1beta1.Seed{}})
		})

		It("should add the object to the queue", func() {
			queue.EXPECT().Add(key)

			c.managedSeedDelete(managedSeed)
		})

		It("should add the object to the queue", func() {
			queue.EXPECT().Add(key)

			c.managedSeedDelete(cache.DeletedFinalStateUnknown{Key: key, Obj: managedSeed})
		})
	})
})

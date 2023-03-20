// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package controllerutils_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	testclock "k8s.io/utils/clock/testing"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/utils/test"
)

var _ = Describe("EventHandler", func() {
	Describe("#EnqueueCreateEventsOncePer24hDuration", func() {
		var (
			handlerFuncs = handler.Funcs{}
			queue        workqueue.RateLimitingInterface
			fakeClock    *testclock.FakeClock

			backupBucket *gardencorev1beta1.BackupBucket
		)

		BeforeEach(func() {
			fakeClock = testclock.NewFakeClock(time.Now())
			queue = workqueue.NewRateLimitingQueueWithDelayingInterface(workqueue.NewDelayingQueueWithCustomClock(fakeClock, ""), workqueue.DefaultControllerRateLimiter())
			handlerFuncs = EnqueueCreateEventsOncePer24hDuration(fakeClock)

			backupBucket = &gardencorev1beta1.BackupBucket{
				ObjectMeta: metav1.ObjectMeta{
					Name:       "bar",
					Namespace:  "foo",
					Generation: 1,
				},
				Status: gardencorev1beta1.BackupBucketStatus{
					ObservedGeneration: 1,
					LastOperation: &gardencorev1beta1.LastOperation{
						Type:           gardencorev1beta1.LastOperationTypeReconcile,
						State:          gardencorev1beta1.LastOperationStateSucceeded,
						LastUpdateTime: metav1.NewTime(fakeClock.Now()),
					},
				},
			}
		})

		It("should enqueue a Request with the Name / Namespace of the object in the CreateEvent immediately if the last reconciliation was 24H ago", func() {
			evt := event.CreateEvent{
				Object: backupBucket,
			}
			fakeClock.Step(24 * time.Hour)
			handlerFuncs.Create(evt, queue)
			verifyQueue(queue)
		})

		It("should enqueue a Request with the Name / Namespace of the object in the CreateEvent after a random duration if the last reconciliation was not 24H ago", func() {
			DeferCleanup(test.WithVars(&RandomDuration, func(time.Duration) time.Duration { return 250 * time.Millisecond }))
			evt := event.CreateEvent{
				Object: backupBucket,
			}
			handlerFuncs.Create(evt, queue)
			Expect(queue.Len()).To(Equal(0))
			fakeClock.Step(1 * time.Second)
			Eventually(func() int {
				return queue.Len()
			}).Should(Equal(1))
			verifyQueue(queue)
		})

		It("should enqueue a Request with the Name / Namespace of the object in the UpdateEvent.", func() {
			evt := event.UpdateEvent{
				ObjectNew: backupBucket,
				ObjectOld: backupBucket,
			}
			handlerFuncs.Update(evt, queue)
			verifyQueue(queue)
		})

		It("should enqueue a Request with the Name / Namespace of the object in the DeleteEvent.", func() {
			evt := event.DeleteEvent{
				Object: backupBucket,
			}
			handlerFuncs.Delete(evt, queue)
			verifyQueue(queue)
		})
	})
})

func verifyQueue(queue workqueue.RateLimitingInterface) {
	ExpectWithOffset(1, queue.Len()).To(Equal(1))

	i, _ := queue.Get()
	ExpectWithOffset(1, i).NotTo(BeNil())
	req, ok := i.(reconcile.Request)
	ExpectWithOffset(1, ok).To(BeTrue())
	ExpectWithOffset(1, req.NamespacedName).To(Equal(types.NamespacedName{
		Name:      "bar",
		Namespace: "foo",
	}))
}

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
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	. "github.com/gardener/gardener/pkg/controllerutils"
)

var _ = Describe("Source", func() {
	Describe("#EnqueueOnce", func() {
		var (
			ctx   = context.Background()
			queue workqueue.RateLimitingInterface
		)

		BeforeEach(func() {
			queue = workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
		})

		It("should enqueue an empty request", func() {
			Expect(EnqueueOnce(ctx, nil, queue)).To(Succeed())
			Expect(queue.Len()).To(Equal(1))

			item, v := queue.Get()
			Expect(item).To(Equal(reconcile.Request{}))
			Expect(v).To(BeFalse())
		})
	})

	Describe("#HandleOnce", func() {
		var (
			ctx          = context.Background()
			eventHandler = handler.Funcs{
				CreateFunc: func(_ event.CreateEvent, queue workqueue.RateLimitingInterface) {
					queue.Add(reconcile.Request{})
				},
			}
			queue workqueue.RateLimitingInterface
		)

		BeforeEach(func() {
			queue = workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
		})

		It("should enqueue an empty request", func() {
			Expect(HandleOnce(ctx, eventHandler, queue)).To(Succeed())
			Expect(queue.Len()).To(Equal(1))

			item, v := queue.Get()
			Expect(item).To(Equal(reconcile.Request{}))
			Expect(v).To(BeFalse())
		})
	})
})

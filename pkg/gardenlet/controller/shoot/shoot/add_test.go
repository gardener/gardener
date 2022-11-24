// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package shoot_test

import (
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/gardenlet/controller/shoot/shoot"
	mockworkqueue "github.com/gardener/gardener/pkg/mock/client-go/util/workqueue"
)

var _ = Describe("Add", func() {
	Describe("#EventHandler", func() {
		var (
			hdlr  handler.EventHandler
			queue *mockworkqueue.MockRateLimitingInterface
			obj   *gardencorev1beta1.Shoot
			req   reconcile.Request
		)

		BeforeEach(func() {
			hdlr = (&Reconciler{}).EventHandler()
			queue = mockworkqueue.NewMockRateLimitingInterface(gomock.NewController(GinkgoT()))
			obj = &gardencorev1beta1.Shoot{ObjectMeta: metav1.ObjectMeta{Name: "shoot", Namespace: "namespace"}}
			req = reconcile.Request{NamespacedName: types.NamespacedName{Name: obj.Name, Namespace: obj.Namespace}}
		})

		It("should enqueue the object for Create events", func() {
			queue.EXPECT().Add(req)

			hdlr.Create(event.CreateEvent{Object: obj}, queue)
		})

		It("should enqueue the object for Update events", func() {
			queue.EXPECT().Add(req)

			hdlr.Update(event.UpdateEvent{ObjectNew: obj, ObjectOld: obj}, queue)
		})

		It("should forget the backoff and enqueue the object for Update events setting the deletionTimestamp", func() {
			queue.EXPECT().Forget(req)
			queue.EXPECT().Add(req)

			objOld := obj.DeepCopy()
			now := metav1.Now()
			obj.SetDeletionTimestamp(&now)
			hdlr.Update(event.UpdateEvent{ObjectNew: obj, ObjectOld: objOld}, queue)
		})

		It("should not enqueue the object for Delete events", func() {
			hdlr.Delete(event.DeleteEvent{Object: obj}, queue)
		})

		It("should not enqueue the object for Generic events", func() {
			hdlr.Generic(event.GenericEvent{Object: obj}, queue)
		})
	})
})

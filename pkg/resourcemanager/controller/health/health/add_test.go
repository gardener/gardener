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

package health_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	. "github.com/gardener/gardener/pkg/resourcemanager/controller/health/health"
)

var _ = Describe("Add", func() {
	Describe("#EnqueueCreateAndUpdate", func() {
		var (
			hdlr  handler.EventHandler
			queue workqueue.RateLimitingInterface
			obj   *corev1.Secret
		)

		BeforeEach(func() {
			hdlr = (&Reconciler{}).EnqueueCreateAndUpdate()
			queue = workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
			obj = &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "secret", Namespace: "namespace"}}
		})

		It("should enqueue the object for Create events", func() {
			hdlr.Create(event.CreateEvent{Object: obj}, queue)

			Expect(queue.Len()).To(Equal(1))
			item, v := queue.Get()
			Expect(item).To(Equal(reconcile.Request{NamespacedName: types.NamespacedName{Name: obj.Name, Namespace: obj.Namespace}}))
			Expect(v).To(BeFalse())
		})

		It("should enqueue the object for Update events", func() {
			hdlr.Update(event.UpdateEvent{ObjectNew: obj, ObjectOld: obj}, queue)

			Expect(queue.Len()).To(Equal(1))
			item, v := queue.Get()
			Expect(item).To(Equal(reconcile.Request{NamespacedName: types.NamespacedName{Name: obj.Name, Namespace: obj.Namespace}}))
			Expect(v).To(BeFalse())
		})

		It("should not enqueue the object for Delete events", func() {
			hdlr.Delete(event.DeleteEvent{Object: obj}, queue)

			Expect(queue.Len()).To(Equal(0))
		})

		It("should not enqueue the object for Generic events", func() {
			hdlr.Generic(event.GenericEvent{Object: obj}, queue)

			Expect(queue.Len()).To(Equal(0))
		})
	})
})

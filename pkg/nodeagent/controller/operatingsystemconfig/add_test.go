// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package operatingsystemconfig_test

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	mockworkqueue "github.com/gardener/gardener/pkg/mock/client-go/util/workqueue"
	"github.com/gardener/gardener/pkg/nodeagent/apis/config"
	. "github.com/gardener/gardener/pkg/nodeagent/controller/operatingsystemconfig"
	"github.com/gardener/gardener/pkg/utils/test"
)

var _ = Describe("Add", func() {
	Describe("#SecretPredicate", func() {
		var (
			p      predicate.Predicate
			secret *corev1.Secret
		)

		BeforeEach(func() {
			p = (&Reconciler{}).SecretPredicate()
			secret = &corev1.Secret{}
		})

		Describe("#Create", func() {
			It("should return true", func() {
				Expect(p.Create(event.CreateEvent{})).To(BeTrue())
			})
		})

		Describe("#Update", func() {
			It("should return false because old object is not a secret", func() {
				Expect(p.Update(event.UpdateEvent{})).To(BeFalse())
			})

			It("should return false because new object is not a secret", func() {
				Expect(p.Update(event.UpdateEvent{ObjectOld: secret})).To(BeFalse())
			})

			It("should return false because OSC data does not change", func() {
				Expect(p.Update(event.UpdateEvent{ObjectOld: secret, ObjectNew: secret})).To(BeFalse())
			})

			It("should return true because OSC data changes", func() {
				oldSecret := secret.DeepCopy()
				secret.Data = map[string][]byte{"osc.yaml": []byte("foo")}
				Expect(p.Update(event.UpdateEvent{ObjectOld: oldSecret, ObjectNew: secret})).To(BeTrue())
			})
		})

		Describe("#Delete", func() {
			It("should return false", func() {
				Expect(p.Delete(event.DeleteEvent{})).To(BeFalse())
			})
		})

		Describe("#Generic", func() {
			It("should return false", func() {
				Expect(p.Generic(event.GenericEvent{})).To(BeFalse())
			})
		})
	})

	Describe("#EnqueueWithJitterDelay", func() {
		var (
			ctx = context.Background()
			log = logr.Discard()

			hdlr  handler.EventHandler
			queue *mockworkqueue.MockRateLimitingInterface
			obj   *corev1.Secret
			req   reconcile.Request
			cfg   config.OperatingSystemConfigControllerConfig

			randomDuration = 10 * time.Millisecond
		)

		BeforeEach(func() {
			cfg = config.OperatingSystemConfigControllerConfig{
				SyncJitterPeriod: &metav1.Duration{Duration: 50 * time.Millisecond},
			}

			hdlr = (&Reconciler{Config: cfg}).EnqueueWithJitterDelay(log)
			queue = mockworkqueue.NewMockRateLimitingInterface(gomock.NewController(GinkgoT()))
			obj = &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "osc-secret", Namespace: "namespace"}}
			req = reconcile.Request{NamespacedName: types.NamespacedName{Name: obj.Name, Namespace: obj.Namespace}}

			DeferCleanup(func() {
				test.WithVar(&RandomDurationWithMetaDuration, func(_ *metav1.Duration) time.Duration { return randomDuration })
			})
		})

		Context("Create events", func() {
			It("should enqueue the object without delay", func() {
				queue.EXPECT().Add(req)

				hdlr.Create(ctx, event.CreateEvent{Object: obj}, queue)
			})
		})

		Context("Update events", func() {
			It("should not enqueue the object when the OSC did not change", func() {
				hdlr.Update(ctx, event.UpdateEvent{ObjectNew: obj, ObjectOld: obj}, queue)
			})

			It("should not enqueue the object when the OSC is the same", func() {
				obj.Data = map[string][]byte{"osc.yaml": []byte(`{"apiVersion":"extensions.gardener.cloud/v1alpha1","kind":"OperatingSystemConfig"}`)}
				oldObj := obj.DeepCopy()

				hdlr.Update(ctx, event.UpdateEvent{ObjectNew: obj, ObjectOld: oldObj}, queue)
			})

			It("should enqueue the object when the OSC changed", func() {
				queue.EXPECT().AddAfter(req, randomDuration)

				obj.Data = map[string][]byte{"osc.yaml": []byte(`{"apiVersion":"extensions.gardener.cloud/v1alpha1","kind":"OperatingSystemConfig"}`)}
				oldObj := obj.DeepCopy()
				oldObj.Data = map[string][]byte{"osc.yaml": []byte(`{"apiVersion":"extensions.gardener.cloud/v1alpha1","kind":"OperatingSystemConfig","generation":1}`)}

				hdlr.Update(ctx, event.UpdateEvent{ObjectNew: obj, ObjectOld: oldObj}, queue)
			})
		})

		Context("Delete events", func() {
			It("should not enqueue the object", func() {
				hdlr.Delete(ctx, event.DeleteEvent{Object: obj}, queue)
			})
		})

		Context("Generic events", func() {
			It("should not enqueue the object", func() {
				hdlr.Generic(ctx, event.GenericEvent{Object: obj}, queue)
			})
		})
	})
})

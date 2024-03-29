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
	"strconv"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/nodeagent/apis/config"
	. "github.com/gardener/gardener/pkg/nodeagent/controller/operatingsystemconfig"
	mockworkqueue "github.com/gardener/gardener/third_party/mock/client-go/util/workqueue"
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

			fakeClient client.Client
			hdlr       handler.EventHandler
			queue      *mockworkqueue.MockRateLimitingInterface
			obj        *corev1.Secret
			req        reconcile.Request
			cfg        config.OperatingSystemConfigControllerConfig

			nodeName string
		)

		BeforeEach(func() {
			fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.ShootScheme).Build()

			nodeName = ""
		})

		JustBeforeEach(func() {
			cfg = config.OperatingSystemConfigControllerConfig{
				SyncJitterPeriod: &metav1.Duration{Duration: 5 * time.Second},
			}

			hdlr = (&Reconciler{
				Client:   fakeClient,
				Config:   cfg,
				NodeName: nodeName,
			}).EnqueueWithJitterDelay(ctx, log)
			queue = mockworkqueue.NewMockRateLimitingInterface(gomock.NewController(GinkgoT()))
			obj = &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "osc-secret", Namespace: "namespace"}}
			req = reconcile.Request{NamespacedName: types.NamespacedName{Name: obj.Name, Namespace: obj.Namespace}}
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

			Context("when the OSC changed", func() {
				var oldObj *corev1.Secret

				JustBeforeEach(func() {
					obj.Data = map[string][]byte{"osc.yaml": []byte(`{"apiVersion":"extensions.gardener.cloud/v1alpha1","kind":"OperatingSystemConfig"}`)}
					oldObj = obj.DeepCopy()
					oldObj.Data = map[string][]byte{"osc.yaml": []byte(`{"apiVersion":"extensions.gardener.cloud/v1alpha1","kind":"OperatingSystemConfig","generation":1}`)}
				})

				When("node name is not known yet", func() {
					It("should enqueue the object without delay", func() {
						queue.EXPECT().AddAfter(req, time.Duration(0))
						hdlr.Update(ctx, event.UpdateEvent{ObjectNew: obj, ObjectOld: oldObj}, queue)
					})
				})

				When("node name is known", func() {
					var node *corev1.Node

					BeforeEach(func() {
						nodeName = "1"
						node = &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: nodeName}}

						Expect(fakeClient.Create(ctx, node)).To(Succeed())
						DeferCleanup(func() {
							Expect(fakeClient.Delete(ctx, node)).To(Succeed())
						})
					})

					When("number of nodes is not larger than max delay seconds", func() {
						When("there are no other nodes", func() {
							// It("should enqueue the object with a delay in the expected range", func() {
							It("should enqueue the object without delay", func() {
								queue.EXPECT().AddAfter(req, time.Duration(0))
								hdlr.Update(ctx, event.UpdateEvent{ObjectNew: obj, ObjectOld: oldObj}, queue)
							})
						})

						When("there are other nodes", func() {
							BeforeEach(func() {
								for i := 2; i <= 5; i++ {
									otherNode := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: strconv.Itoa(i)}}

									Expect(fakeClient.Create(ctx, otherNode)).To(Succeed(), "create node "+otherNode.Name)
									DeferCleanup(func() {
										Expect(fakeClient.Delete(ctx, otherNode)).To(Succeed(), "delete node "+otherNode.Name)
									})
								}
							})

							test := func(node int) {
								BeforeEach(func() {
									nodeName = strconv.Itoa(node)
								})

								It("should enqueue the object with the expected delay", func() {
									queue.EXPECT().AddAfter(req, time.Duration(node-1)*time.Second)
									hdlr.Update(ctx, event.UpdateEvent{ObjectNew: obj, ObjectOld: oldObj}, queue)
								})
							}

							Context("for the first node", func() {
								test(1)
							})

							Context("for the second node", func() {
								test(2)
							})

							Context("for the third node", func() {
								test(3)
							})

							Context("for the fourth node", func() {
								test(4)
							})

							Context("for the last node", func() {
								test(5)
							})
						})
					})

					When("number of nodes is larger than max delay seconds", func() {
						BeforeEach(func() {
							nodeName = "8"

							for i := 2; i <= 15; i++ {
								otherNode := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: strconv.Itoa(i)}}

								Expect(fakeClient.Create(ctx, otherNode)).To(Succeed(), "create node "+otherNode.Name)
								DeferCleanup(func() {
									Expect(fakeClient.Delete(ctx, otherNode)).To(Succeed(), "delete node "+otherNode.Name)
								})
							}
						})

						It("should enqueue the object with a fractional duration", func() {
							fraction := float64(13) / float64(3)
							queue.EXPECT().AddAfter(req, time.Duration(fraction*float64(time.Second)))
							hdlr.Update(ctx, event.UpdateEvent{ObjectNew: obj, ObjectOld: oldObj}, queue)
						})
					})
				})
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

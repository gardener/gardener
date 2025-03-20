// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package operatingsystemconfig_test

import (
	"context"
	"time"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
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
			queue      *mockworkqueue.MockTypedRateLimitingInterface[reconcile.Request]
			obj        *corev1.Secret
			req        reconcile.Request

			nodeName string
		)

		BeforeEach(func() {
			fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.ShootScheme).Build()

			nodeName = ""
		})

		JustBeforeEach(func() {
			hdlr = (&Reconciler{
				Client:   fakeClient,
				NodeName: nodeName,
			}).EnqueueWithJitterDelay(ctx, log)
			queue = mockworkqueue.NewMockTypedRateLimitingInterface[reconcile.Request](gomock.NewController(GinkgoT()))
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
					BeforeEach(func() {
						nodeName = "1"
					})

					When("node does not exist or cannot be read", func() {
						It("should enqueue the object without delay", func() {
							queue.EXPECT().AddAfter(req, time.Duration(0))
							hdlr.Update(ctx, event.UpdateEvent{ObjectNew: obj, ObjectOld: oldObj}, queue)
						})
					})

					When("node exists", func() {
						var node *corev1.Node

						BeforeEach(func() {
							node = &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: nodeName}}
						})

						JustBeforeEach(func() {
							Expect(fakeClient.Create(ctx, node)).To(Succeed())
							DeferCleanup(func() {
								Expect(fakeClient.Delete(ctx, node)).To(Succeed())
							})
						})

						When("node has no reconciliation delay annotation", func() {
							It("should enqueue the object without delay", func() {
								queue.EXPECT().AddAfter(req, time.Duration(0))
								hdlr.Update(ctx, event.UpdateEvent{ObjectNew: obj, ObjectOld: oldObj}, queue)
							})

							When("node had a reconciliation delay previously", func() {
								It("should enqueue the object with the previous delay", func() {
									metav1.SetMetaDataAnnotation(&node.ObjectMeta, "node-agent.gardener.cloud/reconciliation-delay", "8m")
									Expect(fakeClient.Update(ctx, node)).To(Succeed())

									queue.EXPECT().AddAfter(req, 8*time.Minute)
									hdlr.Update(ctx, event.UpdateEvent{ObjectNew: obj, ObjectOld: oldObj}, queue)

									delete(node.Annotations, "node-agent.gardener.cloud/reconciliation-delay")
									Expect(fakeClient.Update(ctx, node)).To(Succeed())

									queue.EXPECT().AddAfter(req, 8*time.Minute)
									hdlr.Update(ctx, event.UpdateEvent{ObjectNew: obj, ObjectOld: oldObj}, queue)
								})
							})
						})

						When("node has reconciliation annotation but it cannot be parsed", func() {
							It("should enqueue the object without delay", func() {
								metav1.SetMetaDataAnnotation(&node.ObjectMeta, "node-agent.gardener.cloud/reconciliation-delay", "fjj123hi")
								Expect(fakeClient.Update(ctx, node)).To(Succeed())

								queue.EXPECT().AddAfter(req, time.Duration(0))
								hdlr.Update(ctx, event.UpdateEvent{ObjectNew: obj, ObjectOld: oldObj}, queue)
							})

							When("node had a reconciliation delay previously", func() {
								It("should enqueue the object with the previous delay", func() {
									metav1.SetMetaDataAnnotation(&node.ObjectMeta, "node-agent.gardener.cloud/reconciliation-delay", "13s")
									Expect(fakeClient.Update(ctx, node)).To(Succeed())

									queue.EXPECT().AddAfter(req, 13*time.Second)
									hdlr.Update(ctx, event.UpdateEvent{ObjectNew: obj, ObjectOld: oldObj}, queue)

									metav1.SetMetaDataAnnotation(&node.ObjectMeta, "node-agent.gardener.cloud/reconciliation-delay", "fjj123hi")
									Expect(fakeClient.Update(ctx, node)).To(Succeed())

									queue.EXPECT().AddAfter(req, 13*time.Second)
									hdlr.Update(ctx, event.UpdateEvent{ObjectNew: obj, ObjectOld: oldObj}, queue)
								})
							})
						})

						When("node has reconciliation annotation and it can be parsed", func() {
							BeforeEach(func() {
								metav1.SetMetaDataAnnotation(&node.ObjectMeta, "node-agent.gardener.cloud/reconciliation-delay", "12h")
							})

							It("should enqueue the object with expected delay", func() {
								queue.EXPECT().AddAfter(req, 12*time.Hour)
								hdlr.Update(ctx, event.UpdateEvent{ObjectNew: obj, ObjectOld: oldObj}, queue)
							})
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

	Describe("#NodeToSecretMapper", func() {
		var (
			mapper handler.MapFunc
			ctx    context.Context

			node *corev1.Node
		)

		BeforeEach(func() {
			ctx = context.TODO()
			mapper = NodeToSecretMapper()
			node = &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Name: "node",
					Labels: map[string]string{
						"worker.gardener.cloud/gardener-node-agent-secret-name": "secret-1",
					},
				},
			}
		})

		It("should return nil when the object is not a Node", func() {
			Expect(mapper(ctx, &corev1.Secret{})).To(BeNil())
		})

		It("should return nil when the node does not have the secret label", func() {
			delete(node.Labels, "worker.gardener.cloud/gardener-node-agent-secret-name")

			Expect(mapper(ctx, node)).To(BeNil())
		})

		It("should map the node to the secret", func() {
			Expect(mapper(ctx, node)).To(ConsistOf(
				reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      "secret-1",
						Namespace: "kube-system",
					},
				}),
			)
		})
	})

	Describe("#NodeReadyForUpdatePredicate", func() {
		var (
			p    predicate.Predicate
			node *corev1.Node
		)

		BeforeEach(func() {
			p = NodeReadyForUpdate()

			node = &corev1.Node{
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{
						{
							Type:   "InPlaceUpdate",
							Status: corev1.ConditionTrue,
							Reason: machinev1alpha1.ReadyForUpdate,
						},
					},
				},
			}
		})

		Describe("#Create", func() {
			It("should return false when object is not node", func() {
				Expect(p.Create(event.CreateEvent{Object: &corev1.Secret{}})).To(BeFalse())
			})

			It("should return false when node status does not have inPlaceUpdate condition with reason ReadyForUpdate", func() {
				Expect(p.Create(event.CreateEvent{Object: &corev1.Node{}})).To(BeFalse())
			})

			It("should return true when node status has inPlaceUpdate condition with reason ReadyForUpdate", func() {
				Expect(p.Create(event.CreateEvent{Object: node})).To(BeTrue())
			})
		})

		Describe("#Update", func() {
			It("should return false because new object is not node", func() {
				Expect(p.Update(event.UpdateEvent{
					ObjectOld: &corev1.Node{},
					ObjectNew: &corev1.Secret{},
				})).To(BeFalse())
			})

			It("should return false because old object is not node", func() {
				Expect(p.Update(event.UpdateEvent{
					ObjectOld: &corev1.Secret{},
					ObjectNew: &corev1.Node{},
				})).To(BeFalse())
			})

			It("should return false because both new and old object have inPlaceUpdate condition with reason ReadyForUpdate", func() {
				Expect(p.Update(event.UpdateEvent{
					ObjectOld: node,
					ObjectNew: node,
				})).To(BeFalse())
			})

			It("should return false because new object does not have inPlaceUpdate condition with reason ReadyForUpdate", func() {
				newNode := node.DeepCopy()
				newNode.Status.Conditions[0].Reason = machinev1alpha1.SelectedForUpdate

				Expect(p.Update(event.UpdateEvent{
					ObjectOld: node,
					ObjectNew: newNode,
				})).To(BeFalse())
			})

			It("should return true because old object does not have inPlaceUpdate condition with reason ReadyForUpdate and new object has", func() {
				oldNode := node.DeepCopy()
				oldNode.Status.Conditions[0].Reason = machinev1alpha1.SelectedForUpdate

				Expect(p.Update(event.UpdateEvent{
					ObjectOld: oldNode,
					ObjectNew: node,
				})).To(BeTrue())
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
})

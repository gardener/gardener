// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package status_test

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/gardener/gardener/pkg/gardenlet/controller/shoot/status"
)

var _ = Describe("Add", func() {
	Describe("#MapWorkerToShoot", func() {
		var (
			reconciler *Reconciler

			shootName        = "shoot"
			projectName      = "local"
			projectNamespace string
			shootTechnicalID string

			ctx        = context.TODO()
			log        = logr.Discard()
			cluster    *extensionsv1alpha1.Cluster
			worker     *extensionsv1alpha1.Worker
			seedClient client.Client
		)

		BeforeEach(func() {
			testScheme := runtime.NewScheme()
			Expect(extensionsv1alpha1.AddToScheme(testScheme)).To(Succeed())

			seedClient = fakeclient.NewClientBuilder().WithScheme(testScheme).Build()
			reconciler = &Reconciler{
				SeedClient: seedClient,
			}

			projectNamespace = "garden-" + projectName
			shootTechnicalID = fmt.Sprintf("shoot--%s--%s", projectName, shootName)

			cluster = &extensionsv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: shootTechnicalID,
				},
				Spec: extensionsv1alpha1.ClusterSpec{
					Shoot: runtime.RawExtension{
						Object: &gardencorev1beta1.Shoot{
							TypeMeta: metav1.TypeMeta{
								APIVersion: "core.gardener.cloud/v1beta1",
								Kind:       "Shoot",
							},
							ObjectMeta: metav1.ObjectMeta{
								Name:      shootName,
								Namespace: projectNamespace,
							},
						},
					},
				},
			}

			worker = &extensionsv1alpha1.Worker{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "worker-",
					Namespace:    shootTechnicalID,
				},
			}

		})

		It("should return nil if the object has a deletion timestamp", func() {
			worker.DeletionTimestamp = &metav1.Time{Time: time.Now()}

			Expect(reconciler.MapWorkerToShoot(log)(ctx, worker)).To(BeNil())
		})

		It("should return nil when cluster is not found", func() {
			Expect(reconciler.MapWorkerToShoot(log)(ctx, worker)).To(BeNil())
		})

		It("should return nil when shoot is not present in the cluster", func() {
			cluster.Spec.Shoot.Object = nil
			Expect(seedClient.Create(ctx, cluster)).To(Succeed())

			Expect(reconciler.MapWorkerToShoot(log)(ctx, worker)).To(BeNil())
		})

		It("should return a request with the shoot name and namespace", func() {
			Expect(seedClient.Create(ctx, cluster)).To(Succeed())
			DeferCleanup(func() {
				Expect(seedClient.Delete(ctx, cluster)).To(Succeed())
			})

			Expect(reconciler.MapWorkerToShoot(log)(ctx, worker)).To(ConsistOf(
				reconcile.Request{NamespacedName: types.NamespacedName{Name: shootName, Namespace: projectNamespace}},
			))
		})
	})

	Describe("#WorkerStatusChangedPredicate", func() {
		var p predicate.Predicate

		BeforeEach(func() {
			p = WorkerStatusChangedPredicate()
		})

		Describe("#Create", func() {
			It("should return false when object is not worker", func() {
				Expect(p.Create(event.CreateEvent{Object: &corev1.Secret{}})).To(BeFalse())
			})

			It("should return false when worker status is nil", func() {
				Expect(p.Create(event.CreateEvent{Object: &extensionsv1alpha1.Worker{}})).To(BeFalse())
			})

			It("should return false when worker status inPlaceUpdates is nil", func() {
				Expect(p.Create(event.CreateEvent{Object: &extensionsv1alpha1.Worker{Status: extensionsv1alpha1.WorkerStatus{}}})).To(BeFalse())
			})

			It("should return false when worker status inPlaceUpdates workerPoolToHashMap is nil", func() {
				Expect(p.Create(event.CreateEvent{
					Object: &extensionsv1alpha1.Worker{
						Status: extensionsv1alpha1.WorkerStatus{InPlaceUpdates: &extensionsv1alpha1.InPlaceUpdatesWorkerStatus{}},
					},
				})).To(BeFalse())
			})

			It("should return true when worker status inPlaceUpdates workerPoolToHashMap is not nil", func() {
				Expect(p.Create(event.CreateEvent{Object: &extensionsv1alpha1.Worker{
					Status: extensionsv1alpha1.WorkerStatus{
						InPlaceUpdates: &extensionsv1alpha1.InPlaceUpdatesWorkerStatus{
							WorkerPoolToHashMap: map[string]string{"pool": "hash"},
						},
					},
				}})).To(BeTrue())
			})
		})

		Describe("#Update", func() {
			It("should return false because new object is not worker", func() {
				Expect(p.Update(event.UpdateEvent{
					ObjectOld: &extensionsv1alpha1.Worker{},
					ObjectNew: &corev1.Secret{},
				})).To(BeFalse())
			})

			It("should return false because old object is not worker", func() {
				Expect(p.Update(event.UpdateEvent{
					ObjectOld: &corev1.Secret{},
					ObjectNew: &extensionsv1alpha1.Worker{},
				})).To(BeFalse())
			})

			It("should return false because both worker status has the same inPlaceUpdates workerPoolToHashMap", func() {
				Expect(p.Update(event.UpdateEvent{
					ObjectOld: &extensionsv1alpha1.Worker{
						Status: extensionsv1alpha1.WorkerStatus{
							InPlaceUpdates: &extensionsv1alpha1.InPlaceUpdatesWorkerStatus{
								WorkerPoolToHashMap: map[string]string{"pool": "hash"},
							},
						},
					},
					ObjectNew: &extensionsv1alpha1.Worker{
						Status: extensionsv1alpha1.WorkerStatus{
							InPlaceUpdates: &extensionsv1alpha1.InPlaceUpdatesWorkerStatus{
								WorkerPoolToHashMap: map[string]string{"pool": "hash"},
							},
						},
					},
				})).To(BeFalse())
			})

			It("should return true because worker status has different inPlaceUpdates workerPoolToHashMap", func() {
				Expect(p.Update(event.UpdateEvent{
					ObjectOld: &extensionsv1alpha1.Worker{
						Status: extensionsv1alpha1.WorkerStatus{
							InPlaceUpdates: &extensionsv1alpha1.InPlaceUpdatesWorkerStatus{
								WorkerPoolToHashMap: map[string]string{"pool": "hash"},
							},
						},
					},
					ObjectNew: &extensionsv1alpha1.Worker{
						Status: extensionsv1alpha1.WorkerStatus{
							InPlaceUpdates: &extensionsv1alpha1.InPlaceUpdatesWorkerStatus{
								WorkerPoolToHashMap: map[string]string{"pool": "new-hash"},
							},
						},
					},
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

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

package project

import (
	"context"
	"errors"
	"reflect"
	"time"

	"github.com/golang/mock/gomock"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	mockworkqueue "github.com/gardener/gardener/pkg/mock/client-go/util/workqueue"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/clock"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("Project Activity", func() {
	var (
		project *gardencorev1beta1.Project

		reconciler reconcile.Reconciler
		request    reconcile.Request

		fakeClock *clock.FakeClock

		ctrl                   *gomock.Controller
		k8sGardenRuntimeClient *mockclient.MockClient
		ctx                    context.Context
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		k8sGardenRuntimeClient = mockclient.NewMockClient(ctrl)
		ctx = context.TODO()

		project = &gardencorev1beta1.Project{
			ObjectMeta: metav1.ObjectMeta{
				Name: projectName,
			},
			Status: gardencorev1beta1.ProjectStatus{
				LastActivityTimestamp: &metav1.Time{Time: time.Date(1, 1, 1, 1, 1, 1, 1, time.UTC)},
			},
			Spec: gardencorev1beta1.ProjectSpec{
				Namespace: &namespaceName,
			},
		}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("Project Activity Reconcile", func() {
		BeforeEach(func() {

			fakeClock = clock.NewFakeClock(time.Now())
			reconciler = NewActivityReconciler(k8sGardenRuntimeClient, fakeClock)

			k8sGardenRuntimeClient.EXPECT().Get(
				ctx,
				gomock.Any(),
				gomock.AssignableToTypeOf(&gardencorev1beta1.Project{}),
			).DoAndReturn(func(_ context.Context, namespacedName client.ObjectKey, obj *gardencorev1beta1.Project) error {
				if reflect.DeepEqual(namespacedName.Namespace, namespaceName) {
					project.DeepCopyInto(obj)
					return nil
				}
				return errors.New("error retrieving object from store")
			})

			k8sGardenRuntimeClient.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&gardencorev1beta1.Project{}), gomock.Any()).DoAndReturn(
				func(_ context.Context, prj *gardencorev1beta1.Project, _ client.Patch, _ ...client.PatchOption) error {
					*project = *prj
					return nil
				},
			).AnyTimes()
			k8sGardenRuntimeClient.EXPECT().Status().Return(k8sGardenRuntimeClient).AnyTimes()
		})

		Context("Project Activity Reconcile", func() {
			It("should update the lastActivityTimestamp to now", func() {
				request = reconcile.Request{NamespacedName: types.NamespacedName{Name: project.Name, Namespace: namespaceName}}
				_, err := reconciler.Reconcile(ctx, request)
				Expect(err).ToNot(HaveOccurred())

				now := &metav1.Time{Time: fakeClock.Now()}
				Expect(project.Status.LastActivityTimestamp).To(Equal(now))
			})

			It("should fail reconcile because the project can't be retrieved", func() {
				request = reconcile.Request{NamespacedName: types.NamespacedName{Name: project.Name, Namespace: namespaceName + "other"}}
				_, err := reconciler.Reconcile(ctx, request)
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("Project Activity Queue", func() {
		var (
			queue *mockworkqueue.MockRateLimitingInterface
			c     *Controller
		)

		BeforeEach(func() {
			queue = mockworkqueue.NewMockRateLimitingInterface(ctrl)
			fakeClock = clock.NewFakeClock(time.Date(2022, 02, 01, 6, 30, 0, 0, time.UTC))
			c = &Controller{
				gardenClient:         k8sGardenRuntimeClient,
				projectActivityQueue: queue,
				clock:                fakeClock,
			}

			k8sGardenRuntimeClient.EXPECT().List(ctx, gomock.AssignableToTypeOf(&gardencorev1beta1.ProjectList{}), client.MatchingFields{gardencore.ProjectNamespace: namespaceName}).DoAndReturn(func(_ context.Context, list *gardencorev1beta1.ProjectList, _ ...client.ListOption) error {
				(&gardencorev1beta1.ProjectList{Items: []gardencorev1beta1.Project{*project}}).DeepCopyInto(list)
				return nil
			}).AnyTimes()

		})

		Context("BackupEntry activity", func() {
			Context("BackupEntry Add", func() {
				var obj = &gardencorev1beta1.BackupEntry{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "backupEntry",
						Namespace: namespaceName,
					},
				}
				It("should add the project to the queue if the creationTimestamp of object is not old", func() {
					queue.EXPECT().Add(projectName)

					obj.ObjectMeta.CreationTimestamp = metav1.Time{Time: time.Date(2022, 02, 01, 5, 30, 0, 0, time.UTC)}

					c.projectActivityBackupEntryAdd(ctx, obj)
				})

				It("should add the project to the queue if the creationTimestamp of object is old", func() {
					obj.ObjectMeta.CreationTimestamp = metav1.Time{Time: time.Date(2022, 02, 01, 5, 29, 0, 0, time.UTC)}

					c.projectActivityBackupEntryAdd(ctx, obj)
				})
			})

			Context("BackupEntry Update", func() {
				var oldObj = &gardencorev1beta1.BackupEntry{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "backupEntry",
						Namespace: namespaceName,
					},
					Spec: gardencorev1beta1.BackupEntrySpec{
						BucketName: "bucket1",
					},
				}

				It("should add the project to the queue if the spec of the object has changed", func() {
					queue.EXPECT().Add(projectName)

					newObj := oldObj.DeepCopy()
					newObj.Spec.BucketName = "bucket2"

					c.projectActivityBackupEntryUpdate(ctx, oldObj, newObj)
				})

				It("should not add the project to the queue if the spec of the object hasn't changed", func() {
					newObj := oldObj.DeepCopy()

					c.projectActivityBackupEntryUpdate(ctx, oldObj, newObj)
				})
			})
		})

		Context("Plant activity", func() {
			Context("Plant Add", func() {
				var obj = &gardencorev1beta1.Plant{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "plant",
						Namespace: namespaceName,
					},
				}

				It("should add the project to the queue if the creationTimestamp of object is not old", func() {
					queue.EXPECT().Add(projectName)

					obj.ObjectMeta.CreationTimestamp = metav1.Time{Time: time.Date(2022, 02, 01, 5, 45, 0, 0, time.UTC)}

					c.projectActivityPlantAdd(ctx, obj)
				})

				It("should add the project to the queue if the creationTimestamp of object is old", func() {
					obj.ObjectMeta.CreationTimestamp = metav1.Time{Time: time.Date(2021, 01, 01, 4, 45, 0, 0, time.UTC)}

					c.projectActivityPlantAdd(ctx, obj)
				})
			})

			Context("Plant Update", func() {
				var oldObj = &gardencorev1beta1.Plant{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "plant",
						Namespace: namespaceName,
					},
					Spec: gardencorev1beta1.PlantSpec{
						Endpoints: []gardencorev1beta1.Endpoint{
							{
								Name: "endpoint",
							},
						},
					},
				}

				It("should add the project to the queue if the spec of the object has changed", func() {
					queue.EXPECT().Add(projectName)

					newObj := oldObj.DeepCopy()
					newObj.Spec.Endpoints[0].Name = "endpoint2"

					c.projectActivityPlantUpdate(ctx, oldObj, newObj)
				})

				It("should not add the project to the queue if the spec of the object hasn't changed", func() {
					newObj := oldObj.DeepCopy()

					c.projectActivityPlantUpdate(ctx, oldObj, newObj)
				})
			})
		})

		Context("Quota activity", func() {
			Context("Quota Add", func() {
				var obj = &gardencorev1beta1.Quota{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "quota",
						Namespace: namespaceName,
						Labels:    map[string]string{"secretbinding.gardener.cloud/referred": "true"},
					},
				}

				It("should add the project to the queue if the creationTimestamp of object is not old", func() {
					queue.EXPECT().Add(projectName)

					obj.ObjectMeta.CreationTimestamp = metav1.Time{Time: time.Date(2022, 02, 01, 6, 00, 0, 0, time.UTC)}

					c.projectActivityQuotaAdd(ctx, obj)
				})

				It("should add the project to the queue if the creationTimestamp of object is old", func() {
					obj.ObjectMeta.CreationTimestamp = metav1.Time{Time: time.Date(2021, 06, 01, 5, 00, 0, 0, time.UTC)}

					c.projectActivityQuotaAdd(ctx, obj)
				})

				It("should not add the project to the queue if the object doesn't have 'referred by a secretbinding' label", func() {
					obj.ObjectMeta.CreationTimestamp = metav1.Time{Time: time.Date(2022, 02, 01, 6, 00, 0, 0, time.UTC)}
					obj.ObjectMeta.Labels = nil

					c.projectActivityQuotaAdd(ctx, obj)
				})
			})

			Context("Quota Update", func() {
				var oldObj = &gardencorev1beta1.Quota{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "quota",
						Namespace: namespaceName,
						Labels:    map[string]string{"secretbinding.gardener.cloud/referred": "true"},
					},
					Spec: gardencorev1beta1.QuotaSpec{
						ClusterLifetimeDays: pointer.Int32(30),
					},
				}

				It("should add the project to the queue if the data of the object has changed", func() {
					queue.EXPECT().Add(projectName)

					newObj := oldObj.DeepCopy()
					newObj.Spec.ClusterLifetimeDays = pointer.Int32(60)

					c.projectActivityQuotaUpdate(ctx, oldObj, newObj)
				})

				It("should not add the project to the queue if the data of the object hasn't changed", func() {
					newObj := oldObj.DeepCopy()

					c.projectActivityQuotaUpdate(ctx, oldObj, newObj)
				})
			})
		})

		Context("Secret activity", func() {
			Context("Secret Add", func() {
				var obj = &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "secret",
						Namespace: namespaceName,
						Labels:    map[string]string{"secretbinding.gardener.cloud/referred": "true"},
					},
				}

				It("should add the project to the queue if the creationTimestamp of object is not old", func() {
					queue.EXPECT().Add(projectName)

					obj.ObjectMeta.CreationTimestamp = metav1.Time{Time: time.Date(2022, 02, 01, 6, 00, 0, 0, time.UTC)}

					c.projectActivitySecretAdd(ctx, obj)
				})

				It("should add the project to the queue if the creationTimestamp of object is old", func() {
					obj.ObjectMeta.CreationTimestamp = metav1.Time{Time: time.Date(2022, 02, 01, 5, 00, 0, 0, time.UTC)}

					c.projectActivitySecretAdd(ctx, obj)
				})

				It("should not add the project to the queue if the object doesn't have 'referred by a secretbinding' label", func() {
					obj.ObjectMeta.CreationTimestamp = metav1.Time{Time: time.Date(2022, 02, 01, 6, 00, 0, 0, time.UTC)}
					obj.ObjectMeta.Labels = nil

					c.projectActivitySecretAdd(ctx, obj)
				})
			})

			Context("Secret Update", func() {
				var oldObj = &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "secret",
						Namespace: namespaceName,
						Labels:    map[string]string{"secretbinding.gardener.cloud/referred": "true"},
					},
					Data: map[string][]byte{"bar": []byte("foo")},
				}

				It("should add the project to the queue if the data of the object has changed", func() {
					queue.EXPECT().Add(projectName)

					newObj := oldObj.DeepCopy()
					newObj.Data = map[string][]byte{"foo": []byte("dash")}

					c.projectActivitySecretUpdate(ctx, oldObj, newObj)
				})

				It("should not add the project to the queue if the data of the object hasn't changed", func() {
					newObj := oldObj.DeepCopy()

					c.projectActivitySecretUpdate(ctx, oldObj, newObj)
				})
			})
		})

		Context("Shoot activity", func() {
			Context("Shoot Add", func() {
				var obj = &gardencorev1beta1.Shoot{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "shoot",
						Namespace: namespaceName,
					},
				}

				It("should add the project to the queue if the creationTimestamp of object is not old", func() {
					queue.EXPECT().Add(projectName)

					obj.ObjectMeta.CreationTimestamp = metav1.Time{Time: time.Date(2022, 02, 01, 6, 00, 0, 0, time.UTC)}

					c.projectActivityShootAdd(ctx, obj)
				})

				It("should add the project to the queue if the creationTimestamp of object is old", func() {
					obj.ObjectMeta.CreationTimestamp = metav1.Time{Time: time.Date(2022, 01, 31, 6, 00, 0, 0, time.UTC)}

					c.projectActivityShootAdd(ctx, obj)
				})
			})

			Context("Shoot Update", func() {
				var oldObj = &gardencorev1beta1.Shoot{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "shoot",
						Namespace: namespaceName,
					},
					Spec: gardencorev1beta1.ShootSpec{
						Kubernetes: gardencorev1beta1.Kubernetes{
							EnableStaticTokenKubeconfig: pointer.Bool(true),
						},
					},
				}

				It("should add the project to the queue if the spec of the object has changed", func() {
					queue.EXPECT().Add(projectName)

					newObj := oldObj.DeepCopy()
					newObj.Spec.CloudProfileName = "cloudProfile"

					c.projectActivityShootUpdate(ctx, oldObj, newObj)
				})

				It("should not add the project to the queue if the spec of the object hasn't changed", func() {
					newObj := oldObj.DeepCopy()

					c.projectActivityShootUpdate(ctx, oldObj, newObj)
				})
			})
		})
	})
})

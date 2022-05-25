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

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	mockworkqueue "github.com/gardener/gardener/pkg/mock/client-go/util/workqueue"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	testclock "k8s.io/utils/clock/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("Project Activity", func() {
	var (
		project *gardencorev1beta1.Project

		reconciler reconcile.Reconciler
		request    reconcile.Request

		fakeClock *testclock.FakeClock

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

			fakeClock = testclock.NewFakeClock(time.Now())
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
			fakeClock = testclock.NewFakeClock(time.Date(2022, 02, 01, 6, 30, 0, 0, time.UTC))
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
			var backupEntry *gardencorev1beta1.BackupEntry

			BeforeEach(func() {
				backupEntry = &gardencorev1beta1.BackupEntry{
					ObjectMeta: metav1.ObjectMeta{
						Name:       "backupEntry",
						Namespace:  namespaceName,
						Generation: 1,
					},
				}
			})

			Context("BackupEntry Add", func() {
				It("should add the project to the queue if the creationTimestamp of object is not old", func() {
					queue.EXPECT().Add(projectName)

					backupEntry.ObjectMeta.CreationTimestamp = metav1.Time{Time: time.Date(2022, 02, 01, 5, 30, 0, 0, time.UTC)}

					c.projectActivityObjectAddDelete(ctx, backupEntry, false, true)
				})

				It("should not add the project to the queue if the creationTimestamp of object is old", func() {
					backupEntry.ObjectMeta.CreationTimestamp = metav1.Time{Time: time.Date(2022, 02, 01, 5, 29, 0, 0, time.UTC)}

					c.projectActivityObjectAddDelete(ctx, backupEntry, false, true)
				})
			})

			Context("BackupEntry Update", func() {
				It("should add the project to the queue if the generation the object has changed", func() {
					queue.EXPECT().Add(projectName)

					newBackupEntry := backupEntry.DeepCopy()
					newBackupEntry.ObjectMeta.Generation = 2

					c.projectActivityObjectUpdate(ctx, backupEntry, newBackupEntry, false)
				})

				It("should not add the project to the queue if the generation of the object hasn't changed", func() {
					newBackupEntry := backupEntry.DeepCopy()

					c.projectActivityObjectUpdate(ctx, backupEntry, newBackupEntry, false)
				})
			})

			Context("BackupEntry Delete", func() {
				It("should add the project to the queue on object deletion even if creationTimestamp is old", func() {
					queue.EXPECT().Add(projectName)

					backupEntry.ObjectMeta.CreationTimestamp = metav1.Time{Time: time.Date(2022, 01, 01, 5, 29, 0, 0, time.UTC)}

					c.projectActivityObjectAddDelete(ctx, backupEntry, false, false)
				})
			})
		})

		Context("Plant activity", func() {
			var plant *gardencorev1beta1.Plant

			BeforeEach(func() {
				plant = &gardencorev1beta1.Plant{
					ObjectMeta: metav1.ObjectMeta{
						Name:       "plant",
						Namespace:  namespaceName,
						Generation: 1,
					},
				}
			})

			Context("Plant Add", func() {
				It("should add the project to the queue if the creationTimestamp of object is not old", func() {
					queue.EXPECT().Add(projectName)

					plant.ObjectMeta.CreationTimestamp = metav1.Time{Time: time.Date(2022, 02, 01, 5, 45, 0, 0, time.UTC)}

					c.projectActivityObjectAddDelete(ctx, plant, false, true)
				})

				It("should not add the project to the queue if the creationTimestamp of object is old", func() {
					plant.ObjectMeta.CreationTimestamp = metav1.Time{Time: time.Date(2021, 01, 01, 4, 45, 0, 0, time.UTC)}

					c.projectActivityObjectAddDelete(ctx, plant, false, true)
				})
			})

			Context("Plant Update", func() {
				It("should add the project to the queue if the generation of the object has changed", func() {
					queue.EXPECT().Add(projectName)

					newPlant := plant.DeepCopy()
					newPlant.ObjectMeta.Generation = 2

					c.projectActivityObjectUpdate(ctx, plant, newPlant, false)
				})

				It("should not add the project to the queue if the generation of the object hasn't changed", func() {
					newPlant := plant.DeepCopy()

					c.projectActivityObjectUpdate(ctx, plant, newPlant, false)
				})
			})

			Context("Plant Delete", func() {
				It("should add the project to the queue on object deletion even if creationTimestamp is old", func() {
					queue.EXPECT().Add(projectName)

					plant.ObjectMeta.CreationTimestamp = metav1.Time{Time: time.Date(2021, 01, 01, 4, 45, 0, 0, time.UTC)}

					c.projectActivityObjectAddDelete(ctx, plant, false, false)
				})
			})
		})

		Context("Quota activity", func() {
			var quota *gardencorev1beta1.Quota

			BeforeEach(func() {
				quota = &gardencorev1beta1.Quota{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "quota",
						Namespace: namespaceName,
						Labels:    map[string]string{"reference.gardener.cloud/secretbinding": "true"},
					},
				}
			})

			Context("Quota Add", func() {
				It("should add the project to the queue if the creationTimestamp of object is not old", func() {
					queue.EXPECT().Add(projectName)

					quota.ObjectMeta.CreationTimestamp = metav1.Time{Time: time.Date(2022, 02, 01, 6, 00, 0, 0, time.UTC)}

					c.projectActivityObjectAddDelete(ctx, quota, true, true)
				})

				It("should not add the project to the queue if the creationTimestamp of object is old", func() {
					quota.ObjectMeta.CreationTimestamp = metav1.Time{Time: time.Date(2021, 06, 01, 5, 00, 0, 0, time.UTC)}

					c.projectActivityObjectAddDelete(ctx, quota, true, true)
				})

				It("should not add the project to the queue if the object doesn't have 'referred by a secretbinding' label", func() {
					quota.ObjectMeta.CreationTimestamp = metav1.Time{Time: time.Date(2022, 02, 01, 6, 00, 0, 0, time.UTC)}
					quota.ObjectMeta.Labels = nil

					c.projectActivityObjectAddDelete(ctx, quota, true, true)
				})
			})

			Context("Quota Update", func() {
				BeforeEach(func() {
					quota.ObjectMeta.Labels = nil
				})

				It("should add the project to the queue if old object doesn't have the label and new object have it (the quota is referred for the first time)", func() {
					queue.EXPECT().Add(projectName)
					newQuota := quota.DeepCopy()
					newQuota.ObjectMeta.Labels = map[string]string{"reference.gardener.cloud/secretbinding": "true"}

					c.projectActivityObjectUpdate(ctx, quota, newQuota, true)
				})

				It("should add the project to the queue if the old object has the label and new object doesn't (the quota is no longer referred)", func() {
					queue.EXPECT().Add(projectName)
					newQuota := quota.DeepCopy()
					quota.ObjectMeta.Labels = map[string]string{"reference.gardener.cloud/secretbinding": "true"}

					c.projectActivityObjectUpdate(ctx, quota, newQuota, true)
				})

				It("should not add the project to the queue if neither of the objects have the label", func() {
					newQuota := quota.DeepCopy()

					c.projectActivityObjectUpdate(ctx, quota, newQuota, true)
				})
			})

			Context("Quota Delete", func() {
				It("should add the project to the queue on object deletion if the object has label even if creationTimestamp is old", func() {
					queue.EXPECT().Add(projectName)

					quota.ObjectMeta.CreationTimestamp = metav1.Time{Time: time.Date(2021, 01, 01, 4, 45, 0, 0, time.UTC)}

					c.projectActivityObjectAddDelete(ctx, quota, true, false)
				})

				It("should not add the project to the queue on object deletion if the object doesn't have label", func() {
					quota.ObjectMeta.Labels = nil

					c.projectActivityObjectAddDelete(ctx, quota, true, false)
				})
			})
		})

		Context("Secret activity", func() {
			var secret *corev1.Secret

			BeforeEach(func() {
				secret = &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "secret",
						Namespace: namespaceName,
						Labels:    map[string]string{"reference.gardener.cloud/secretbinding": "true"},
					},
				}
			})

			Context("Secret Add", func() {
				It("should add the project to the queue if the creationTimestamp of object is not old", func() {
					queue.EXPECT().Add(projectName)

					secret.ObjectMeta.CreationTimestamp = metav1.Time{Time: time.Date(2022, 02, 01, 6, 00, 0, 0, time.UTC)}

					c.projectActivityObjectAddDelete(ctx, secret, true, true)
				})

				It("should not add the project to the queue if the creationTimestamp of object is old", func() {
					secret.ObjectMeta.CreationTimestamp = metav1.Time{Time: time.Date(2022, 02, 01, 5, 00, 0, 0, time.UTC)}

					c.projectActivityObjectAddDelete(ctx, secret, true, true)
				})

				It("should not add the project to the queue if the object doesn't have 'referred by a secretbinding' label", func() {
					secret.ObjectMeta.CreationTimestamp = metav1.Time{Time: time.Date(2022, 02, 01, 6, 00, 0, 0, time.UTC)}
					secret.ObjectMeta.Labels = nil

					c.projectActivityObjectAddDelete(ctx, secret, true, true)
				})
			})

			Context("Secret Update", func() {
				BeforeEach(func() {
					secret.ObjectMeta.Labels = nil
				})

				It("should add the project to the queue if old object doesn't have the label and new object have it (the secret is referred for the first time)", func() {
					queue.EXPECT().Add(projectName)
					newSecret := secret.DeepCopy()
					newSecret.ObjectMeta.Labels = map[string]string{"reference.gardener.cloud/secretbinding": "true"}

					c.projectActivityObjectUpdate(ctx, secret, newSecret, true)
				})

				It("should add the project to the queue if the old object has the label and new object doesn't (the secret is no longer referred)", func() {
					queue.EXPECT().Add(projectName)
					newSecret := secret.DeepCopy()
					secret.ObjectMeta.Labels = map[string]string{"reference.gardener.cloud/secretbinding": "true"}

					c.projectActivityObjectUpdate(ctx, secret, newSecret, true)
				})

				It("should not add the project to the queue if neither of the objects have the label", func() {
					newSecret := secret.DeepCopy()

					c.projectActivityObjectUpdate(ctx, secret, newSecret, true)
				})
			})

			Context("Secret Delete", func() {
				It("should add the project to the queue on object deletion if the object has label even if creationTimestamp is old", func() {
					queue.EXPECT().Add(projectName)

					secret.ObjectMeta.CreationTimestamp = metav1.Time{Time: time.Date(2022, 02, 01, 5, 00, 0, 0, time.UTC)}

					c.projectActivityObjectAddDelete(ctx, secret, true, false)
				})

				It("should not add the project to the queue on object deletion if the object doesn't have label", func() {
					secret.ObjectMeta.Labels = nil

					c.projectActivityObjectAddDelete(ctx, secret, true, false)
				})
			})
		})

		Context("Shoot activity", func() {
			var shoot *gardencorev1beta1.Shoot

			BeforeEach(func() {
				shoot = &gardencorev1beta1.Shoot{
					ObjectMeta: metav1.ObjectMeta{
						Name:       "shoot",
						Namespace:  namespaceName,
						Generation: 1,
					},
				}
			})

			Context("Shoot Add", func() {
				It("should add the project to the queue if the creationTimestamp of object is not old", func() {
					queue.EXPECT().Add(projectName)

					shoot.ObjectMeta.CreationTimestamp = metav1.Time{Time: time.Date(2022, 02, 01, 6, 00, 0, 0, time.UTC)}

					c.projectActivityObjectAddDelete(ctx, shoot, false, true)
				})

				It("should not add the project to the queue if the creationTimestamp of object is old", func() {
					shoot.ObjectMeta.CreationTimestamp = metav1.Time{Time: time.Date(2022, 01, 31, 6, 00, 0, 0, time.UTC)}

					c.projectActivityObjectAddDelete(ctx, shoot, false, true)
				})
			})

			Context("Shoot Update", func() {
				It("should add the project to the queue if the generation of the object has changed", func() {
					queue.EXPECT().Add(projectName)

					newShoot := shoot.DeepCopy()
					newShoot.ObjectMeta.Generation = 2

					c.projectActivityObjectUpdate(ctx, shoot, newShoot, false)
				})

				It("should not add the project to the queue if the generation of the object hasn't changed", func() {
					newShoot := shoot.DeepCopy()

					c.projectActivityObjectUpdate(ctx, shoot, newShoot, false)
				})
			})

			Context("Shoot Delete", func() {
				It("should add the project to the queue on object deletion even if creationTimestamp is old", func() {
					queue.EXPECT().Add(projectName)

					shoot.ObjectMeta.CreationTimestamp = metav1.Time{Time: time.Date(2022, 01, 31, 6, 00, 0, 0, time.UTC)}

					c.projectActivityObjectAddDelete(ctx, shoot, false, false)
				})
			})
		})
	})
})

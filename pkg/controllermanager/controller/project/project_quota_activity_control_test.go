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

package project

import (
	"context"
	"errors"
	"reflect"
	"time"

	"github.com/golang/mock/gomock"

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("Project Quota Activity Reconcile", func() {
	var (
		project             *gardencorev1beta1.Project
		quota               *gardencorev1beta1.Quota
		quota2              *gardencorev1beta1.Quota
		quota3              *gardencorev1beta1.Quota
		quotaWithoutProject *gardencorev1beta1.Quota
		errorquota          *gardencorev1beta1.Quota

		reconciler reconcile.Reconciler
		request    reconcile.Request

		k8sGardenRuntimeClient *mockclient.MockClient
		ctx                    = context.TODO()
	)

	BeforeEach(func() {
		project = &gardencorev1beta1.Project{
			ObjectMeta: metav1.ObjectMeta{
				Name:      projectName,
				Namespace: namespaceName,
			},
			Status: gardencorev1beta1.ProjectStatus{
				LastActivityTimestamp: &metav1.Time{Time: time.Date(1, 1, 1, 1, 1, 1, 1, time.UTC)},
			},
			Spec: gardencorev1beta1.ProjectSpec{
				Namespace: &namespaceName,
			},
		}
		quota = &gardencorev1beta1.Quota{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "test-quota-1",
				Namespace:         namespaceName,
				CreationTimestamp: metav1.Time{Time: time.Date(1, 2, 1, 1, 1, 1, 1, time.UTC)},
			},
		}
		quota2 = &gardencorev1beta1.Quota{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "test-quota-2",
				Namespace:         namespaceName,
				CreationTimestamp: metav1.Time{Time: time.Date(1, 3, 1, 1, 1, 1, 1, time.UTC)},
			},
		}
		quota3 = &gardencorev1beta1.Quota{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "test-quota-3",
				Namespace:         namespaceName,
				CreationTimestamp: metav1.Time{Time: time.Date(1, 4, 1, 1, 1, 1, 1, time.UTC)},
			},
		}
		quotaWithoutProject = &gardencorev1beta1.Quota{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "test-quota-without-project",
				Namespace:         "fake",
				CreationTimestamp: metav1.Time{Time: time.Date(1, 5, 1, 1, 1, 1, 1, time.UTC)},
			},
		}
		errorquota = &gardencorev1beta1.Quota{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "error-quota",
				Namespace:         "error",
				CreationTimestamp: metav1.Time{Time: time.Date(1, 6, 1, 1, 1, 1, 1, time.UTC)},
			},
		}

		ctrl := gomock.NewController(GinkgoT())
		k8sGardenRuntimeClient = mockclient.NewMockClient(ctrl)
		request = reconcile.Request{NamespacedName: types.NamespacedName{Namespace: namespaceName, Name: quota.Name}}
		reconciler = NewQuotaActivityReconciler(k8sGardenRuntimeClient)

		k8sGardenRuntimeClient.EXPECT().List(
			ctx,
			gomock.AssignableToTypeOf(&gardencorev1beta1.ProjectList{}),
			gomock.Any(),
		).DoAndReturn(func(_ context.Context, list *gardencorev1beta1.ProjectList, opts client.MatchingFields) error {
			obj := &gardencorev1beta1.ProjectList{}
			if reflect.DeepEqual(opts[core.ProjectNamespace], *project.Spec.Namespace) {
				obj = &gardencorev1beta1.ProjectList{
					Items: []gardencorev1beta1.Project{*project},
				}
			} else if reflect.DeepEqual(opts[core.ProjectNamespace], "error") {
				return errors.New("API ERROR")
			}
			obj.DeepCopyInto(list)

			return nil
		}).AnyTimes()

		k8sGardenRuntimeClient.EXPECT().Get(
			ctx,
			gomock.Any(),
			gomock.AssignableToTypeOf(&gardencorev1beta1.Quota{}),
		).DoAndReturn(func(_ context.Context, namespacedName client.ObjectKey, obj *gardencorev1beta1.Quota) error {
			for _, s := range []gardencorev1beta1.Quota{*quota, *quota2, *quotaWithoutProject, *errorquota} {
				if reflect.DeepEqual(namespacedName.Name, s.Name) && reflect.DeepEqual(namespacedName.Namespace, s.Namespace) {
					s.DeepCopyInto(obj)
				}
			}
			return nil
		}).AnyTimes()
	})

	Describe("LastActivityTimestamp updates", func() {
		BeforeEach(func() {
			k8sGardenRuntimeClient.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&gardencorev1beta1.Project{}), gomock.Any()).DoAndReturn(
				func(_ context.Context, prj *gardencorev1beta1.Project, _ client.Patch, _ ...client.PatchOption) error {
					*project = *prj
					return nil
				},
			).AnyTimes()
			k8sGardenRuntimeClient.EXPECT().Status().Return(k8sGardenRuntimeClient).AnyTimes()
		})

		It("should update the lastActivity timestamp with the creation timestamp of the quota", func() {
			reconcileResult, err := reconciler.Reconcile(ctx, request)
			Expect(err).ToNot(HaveOccurred())
			Expect(reconcileResult).To(Equal(reconcile.Result{}))
			Expect(*project.Status.LastActivityTimestamp).To(Equal(quota.CreationTimestamp))

			request = reconcile.Request{NamespacedName: types.NamespacedName{Namespace: namespaceName, Name: quota2.Name}}
			reconcileResult, err = reconciler.Reconcile(ctx, request)
			Expect(err).ToNot(HaveOccurred())
			Expect(reconcileResult).To(Equal(reconcile.Result{}))
			Expect(*project.Status.LastActivityTimestamp).To(Equal(quota2.CreationTimestamp))
		})

		It("the empty LastActivityTimestamp should be set to the newest quota having a secret binding referring it", func() {
			secretBinding1 := &gardencorev1beta1.SecretBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-sb-1",
					Namespace: namespaceName + "other",
				},
				Quotas: []corev1.ObjectReference{
					{
						Name:      quota.Name,
						Namespace: quota.Namespace,
					},
				},
			}

			secretBinding2 := &gardencorev1beta1.SecretBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-sb-2",
					Namespace: namespaceName + "other-2",
				},
				Quotas: []corev1.ObjectReference{
					{
						Name:      quota2.Name,
						Namespace: quota2.Namespace,
					},
				},
			}

			k8sGardenRuntimeClient.EXPECT().List(ctx,
				gomock.AssignableToTypeOf(&gardencorev1beta1.QuotaList{}),
				client.InNamespace(namespaceName),
			).DoAndReturn(func(_ context.Context, list *gardencorev1beta1.QuotaList, _ ...client.ListOption) error {
				obj := &gardencorev1beta1.QuotaList{Items: []gardencorev1beta1.Quota{*quota, *quota2, *quota3}}
				obj.DeepCopyInto(list)
				return nil
			})

			k8sGardenRuntimeClient.EXPECT().List(ctx,
				gomock.AssignableToTypeOf(&gardencorev1beta1.SecretBindingList{}),
				gomock.Any(),
			).DoAndReturn(func(_ context.Context, list *gardencorev1beta1.SecretBindingList, _ ...client.ListOption) error {
				obj := &gardencorev1beta1.SecretBindingList{Items: []gardencorev1beta1.SecretBinding{*secretBinding1, *secretBinding2}}
				obj.DeepCopyInto(list)
				return nil
			}).AnyTimes()

			project.Status.LastActivityTimestamp = nil

			reconcileResult, err := reconciler.Reconcile(ctx, request)
			Expect(err).ToNot(HaveOccurred())
			Expect(reconcileResult).To(Equal(reconcile.Result{}))
			Expect(*project.Status.LastActivityTimestamp).To(Equal(quota2.CreationTimestamp))
		})

		It("should not update the creation timestamp since the quota is not part of this project", func() {
			request = reconcile.Request{NamespacedName: types.NamespacedName{Namespace: quotaWithoutProject.Namespace, Name: quotaWithoutProject.Name}}
			reconcileResult, err := reconciler.Reconcile(ctx, request)
			Expect(err).ToNot(HaveOccurred())
			Expect(reconcileResult).To(Equal(reconcile.Result{Requeue: false}))
			Expect(*project.Status.LastActivityTimestamp).NotTo(Equal(quotaWithoutProject.CreationTimestamp))
		})
	})

	Describe("Unsuccessful reconciles due to different errors", func() {
		It("should not update the lastActivity timestamp since the quota is created before the last activity", func() {
			quota.CreationTimestamp = metav1.Time{Time: time.Date(1, 1, 0, 1, 0, 0, 0, time.UTC)}
			oldLastActivityTimestamp := *project.Status.LastActivityTimestamp
			_, err := reconciler.Reconcile(ctx, request)
			Expect(err).ToNot(HaveOccurred())
			Expect(*project.Status.LastActivityTimestamp).To(Equal(oldLastActivityTimestamp))
		})

		It("should not update the lastActivity timestamp since the quota does not exist", func() {
			oldLastActivityTimestamp := *project.Status.LastActivityTimestamp
			reconcileResult, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: quota.Name, Namespace: "empty"}})
			Expect(err).NotTo(HaveOccurred())
			Expect(reconcileResult).To(Equal(reconcile.Result{}))
			Expect(*project.Status.LastActivityTimestamp).To(Equal(oldLastActivityTimestamp))
		})

		It("should fail the reconcile since the projects can not be listed ", func() {
			reconcileResult, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: errorquota.Name, Namespace: errorquota.Namespace}})
			Expect(err).To(HaveOccurred())
			Expect(reconcileResult).To(Equal(reconcile.Result{}))
		})
	})
})

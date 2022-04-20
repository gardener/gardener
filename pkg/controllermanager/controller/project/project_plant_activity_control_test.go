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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("Project Plant Activity Reconcile", func() {
	var (
		project             *gardencorev1beta1.Project
		plant               *gardencorev1beta1.Plant
		plant2              *gardencorev1beta1.Plant
		plantWithoutProject *gardencorev1beta1.Plant
		errorPlant          *gardencorev1beta1.Plant

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
		plant = &gardencorev1beta1.Plant{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "test-plant-1",
				Namespace:         namespaceName,
				CreationTimestamp: metav1.Time{Time: time.Date(1, 2, 1, 1, 1, 1, 1, time.UTC)},
			},
		}
		plant2 = &gardencorev1beta1.Plant{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "test-plant-2",
				Namespace:         namespaceName,
				CreationTimestamp: metav1.Time{Time: time.Date(1, 3, 1, 1, 1, 1, 1, time.UTC)},
			},
		}
		plantWithoutProject = &gardencorev1beta1.Plant{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "test-plant-without-project",
				Namespace:         "fake",
				CreationTimestamp: metav1.Time{Time: time.Date(1, 5, 1, 1, 1, 1, 1, time.UTC)},
			},
		}
		errorPlant = &gardencorev1beta1.Plant{
			ObjectMeta: metav1.ObjectMeta{
				Name:              "error-plant",
				Namespace:         "error",
				CreationTimestamp: metav1.Time{Time: time.Date(1, 6, 1, 1, 1, 1, 1, time.UTC)},
			},
		}

		ctrl := gomock.NewController(GinkgoT())
		k8sGardenRuntimeClient = mockclient.NewMockClient(ctrl)
		request = reconcile.Request{NamespacedName: types.NamespacedName{Namespace: namespaceName, Name: plant.Name}}
		reconciler = NewPlantActivityReconciler(k8sGardenRuntimeClient)

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
			gomock.AssignableToTypeOf(&gardencorev1beta1.Plant{}),
		).DoAndReturn(func(_ context.Context, namespacedName client.ObjectKey, obj *gardencorev1beta1.Plant) error {
			for _, s := range []gardencorev1beta1.Plant{*plant, *plant2, *plantWithoutProject, *errorPlant} {
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

		It("should update the lastActivity timestamp with the creation timestamp of the plant", func() {
			reconcileResult, err := reconciler.Reconcile(ctx, request)
			Expect(err).ToNot(HaveOccurred())
			Expect(reconcileResult).To(Equal(reconcile.Result{}))
			Expect(*project.Status.LastActivityTimestamp).To(Equal(plant.CreationTimestamp))

			request = reconcile.Request{NamespacedName: types.NamespacedName{Namespace: namespaceName, Name: plant2.Name}}
			reconcileResult, err = reconciler.Reconcile(ctx, request)
			Expect(err).ToNot(HaveOccurred())
			Expect(reconcileResult).To(Equal(reconcile.Result{}))
			Expect(*project.Status.LastActivityTimestamp).To(Equal(plant2.CreationTimestamp))
		})

		It("the empty LastActivityTimestamp should be set to the newest plant", func() {
			plant3 := plant.DeepCopy()
			plant3.CreationTimestamp = metav1.Time{Time: time.Date(1, 3, 2, 1, 1, 1, 1, time.UTC)}

			k8sGardenRuntimeClient.EXPECT().List(ctx,
				gomock.AssignableToTypeOf(&gardencorev1beta1.PlantList{}),
				client.InNamespace(namespaceName),
			).DoAndReturn(func(_ context.Context, list *gardencorev1beta1.PlantList, _ ...client.ListOption) error {
				obj := &gardencorev1beta1.PlantList{Items: []gardencorev1beta1.Plant{*plant, *plant2, *plant3}}
				obj.DeepCopyInto(list)
				return nil
			})
			project.Status.LastActivityTimestamp = nil

			reconcileResult, err := reconciler.Reconcile(ctx, request)
			Expect(err).ToNot(HaveOccurred())
			Expect(reconcileResult).To(Equal(reconcile.Result{}))
			Expect(*project.Status.LastActivityTimestamp).To(Equal(plant3.CreationTimestamp))
		})

		It("should not update the creation timestamp since the plant is not part of this project", func() {
			request = reconcile.Request{NamespacedName: types.NamespacedName{Namespace: plantWithoutProject.Namespace, Name: plantWithoutProject.Name}}
			reconcileResult, err := reconciler.Reconcile(ctx, request)
			Expect(err).ToNot(HaveOccurred())
			Expect(reconcileResult).To(Equal(reconcile.Result{Requeue: false}))
			Expect(*project.Status.LastActivityTimestamp).NotTo(Equal(plantWithoutProject.CreationTimestamp))
		})
	})

	Describe("Unsuccessful reconciles due to different errors", func() {
		It("should not update the lastActivity timestamp since the plant is created before the last activity", func() {
			plant.CreationTimestamp = metav1.Time{Time: time.Date(1, 1, 0, 1, 0, 0, 0, time.UTC)}
			oldLastActivityTimestamp := *project.Status.LastActivityTimestamp
			_, err := reconciler.Reconcile(ctx, request)
			Expect(err).ToNot(HaveOccurred())
			Expect(*project.Status.LastActivityTimestamp).To(Equal(oldLastActivityTimestamp))
		})

		It("should not update the lastActivity timestamp since the plant does not exist", func() {
			oldLastActivityTimestamp := *project.Status.LastActivityTimestamp
			reconcileResult, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: plant.Name, Namespace: "empty"}})
			Expect(err).NotTo(HaveOccurred())
			Expect(reconcileResult).To(Equal(reconcile.Result{}))
			Expect(*project.Status.LastActivityTimestamp).To(Equal(oldLastActivityTimestamp))
		})

		It("should fail the reconcile since the projects can not be listed ", func() {
			reconcileResult, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: errorPlant.Name, Namespace: errorPlant.Namespace}})
			Expect(err).To(HaveOccurred())
			Expect(reconcileResult).To(Equal(reconcile.Result{}))
		})
	})
})

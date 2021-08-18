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

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/logger"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/golang/mock/gomock"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
	namespaceName = "namespace"
	projectName   = "name"
	shootName     = "shoot"
)

var _ = Describe("Project Activity Reconcile", func() {
	var (
		project             *gardencorev1beta1.Project
		shoot               *gardencorev1beta1.Shoot
		shootWithoutProject *gardencorev1beta1.Shoot
		errorShoot          *gardencorev1beta1.Shoot

		reconciler reconcile.Reconciler

		request reconcile.Request

		ctrl                   *gomock.Controller
		k8sGardenRuntimeClient *mockclient.MockClient
		ctx                    = context.TODO()
	)

	BeforeEach(func() {
		project = &gardencorev1beta1.Project{
			ObjectMeta: metav1.ObjectMeta{
				Name:      projectName,
				Namespace: namespaceName,
				UID:       "1",
			},
			Status: gardencorev1beta1.ProjectStatus{
				LastActivityTimestamp: &metav1.Time{Time: time.Date(1, 1, 1, 1, 1, 1, 1, time.UTC)},
			},
			Spec: gardencorev1beta1.ProjectSpec{
				Namespace: &namespaceName,
			},
		}
		shoot = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:              shootName,
				Namespace:         namespaceName,
				UID:               "1",
				CreationTimestamp: metav1.Time{Time: time.Date(1, 2, 1, 1, 1, 1, 1, time.UTC)},
			},
		}
		shootWithoutProject = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:              shootName,
				Namespace:         "fake",
				UID:               "1",
				CreationTimestamp: metav1.Time{Time: time.Date(1, 3, 1, 1, 1, 1, 1, time.UTC)},
			},
		}
		errorShoot = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:              shootName,
				Namespace:         "error",
				UID:               "1",
				CreationTimestamp: metav1.Time{Time: time.Date(1, 4, 1, 1, 1, 1, 1, time.UTC)},
			},
		}

		logger.Logger = logger.NewLogger("info", "")
		ctrl = gomock.NewController(GinkgoT())
		k8sGardenRuntimeClient = mockclient.NewMockClient(ctrl)
		reconciler = NewActivityReconciler(logger.NewNopLogger(), k8sGardenRuntimeClient)
		request = reconcile.Request{NamespacedName: types.NamespacedName{Name: shoot.Name, Namespace: shoot.Namespace}}
		k8sGardenRuntimeClient.EXPECT().List(ctx, gomock.AssignableToTypeOf(&gardencorev1beta1.ProjectList{}), gomock.Any()).DoAndReturn(func(_ context.Context, obj *gardencorev1beta1.ProjectList, opts client.MatchingFields) error {
			if reflect.DeepEqual(opts[core.ProjectNamespace], *project.Spec.Namespace) {
				*obj = gardencorev1beta1.ProjectList{Items: []gardencorev1beta1.Project{*project}}
				return nil
			}
			if reflect.DeepEqual(opts[core.ProjectNamespace], "error") {
				return errors.New("API ERROR")
			}
			logger.Logger.Infof("Project %s not found returning empty", opts[core.ProjectNamespace])
			*obj = gardencorev1beta1.ProjectList{}
			return nil
		})

		k8sGardenRuntimeClient.EXPECT().Get(ctx, gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{})).DoAndReturn(func(_ context.Context, namespacedName client.ObjectKey, obj *gardencorev1beta1.Shoot) error {
			for _, s := range []gardencorev1beta1.Shoot{*shoot, *shootWithoutProject, *errorShoot} {
				if reflect.DeepEqual(namespacedName.Name, s.Name) && reflect.DeepEqual(namespacedName.Namespace, s.Namespace) {
					*obj = s
					return nil
				}
			}
			return apierrors.NewNotFound(gardencorev1beta1.Resource("Project"), "<unknown>")
		})
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

		It("should update the creation timestamp", func() {
			shoot.CreationTimestamp = metav1.Time{Time: time.Date(1, 1, 2, 1, 1, 1, 1, time.UTC)}
			_, err := reconciler.Reconcile(ctx, request)
			Expect(err).ToNot(HaveOccurred())
			Expect(shoot.CreationTimestamp).To(Equal(*project.Status.LastActivityTimestamp))
		})

		It("the empty LastActivityTimestamp should be set to the newest shoot", func() {
			k8sGardenRuntimeClient.EXPECT().List(ctx, gomock.AssignableToTypeOf(&gardencorev1beta1.ShootList{}), gomock.Any()).DoAndReturn(func(_ context.Context, obj *gardencorev1beta1.ShootList, _ ...client.ListOption) error {
				obj.Items = []gardencorev1beta1.Shoot{*shoot, *shootWithoutProject, *errorShoot}
				return nil
			})
			project.Status.LastActivityTimestamp = nil
			reconcileResult, err := reconciler.Reconcile(ctx, request)
			Expect(err).ToNot(HaveOccurred())
			Expect(reconcileResult).To(Equal(reconcile.Result{}))
			Expect(errorShoot.CreationTimestamp).To(Equal(*project.Status.LastActivityTimestamp))
		})

		It("should not update the creation timestamp since the shoot is not part of this project", func() {
			reconcileResult, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: shootWithoutProject.Name, Namespace: shootWithoutProject.Namespace}})
			Expect(err).ToNot(HaveOccurred())
			Expect(reconcileResult).To(Equal(reconcile.Result{Requeue: false}))
			Expect(shoot.CreationTimestamp).ToNot(Equal(*project.Status.LastActivityTimestamp))
		})
	})

	Describe("Unsuccessful reconciles due to different errors", func() {
		It("should not update the creation timestamp since the shoot is created before the last activity", func() {
			shoot.CreationTimestamp = metav1.Time{Time: time.Date(1, 1, 0, 1, 0, 0, 0, time.UTC)}
			_, err := reconciler.Reconcile(ctx, request)
			Expect(err).ToNot(HaveOccurred())
			Expect(shoot.CreationTimestamp).ToNot(Equal(*project.Status.LastActivityTimestamp))
		})

		It("should not update the creation timestamp since the shoot does not exist", func() {
			reconcileResult, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: shoot.Name, Namespace: "empty"}})
			Expect(err).To(HaveOccurred())
			Expect(reconcileResult).To(Equal(reconcile.Result{}))
		})

		It("should fail the reconcile since the projects can not be listed ", func() {
			reconcileResult, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: errorShoot.Name, Namespace: errorShoot.Namespace}})
			Expect(err).To(HaveOccurred())
			Expect(reconcileResult).To(Equal(reconcile.Result{}))
		})
	})
})

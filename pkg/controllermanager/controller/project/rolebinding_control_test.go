// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/logger"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("#roleBindingDelete", func() {
	const ns = "test"

	var (
		ctrl *gomock.Controller
		c    *mockclient.MockClient

		ctx         = context.TODO()
		controller  *Controller
		queue       workqueue.RateLimitingInterface
		proj        *gardencorev1beta1.Project
		rolebinding *rbacv1.RoleBinding
	)

	BeforeEach(func() {
		// This should not be here!!! Hidden dependency!!!
		logger.Logger = logger.NewNopLogger()

		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)

		queue = workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
		proj = &gardencorev1beta1.Project{
			ObjectMeta: metav1.ObjectMeta{Name: "project-1"},
			Spec: gardencorev1beta1.ProjectSpec{
				Namespace: pointer.String(ns),
			},
		}
		rolebinding = &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: "role-1", Namespace: ns},
		}
		controller = &Controller{
			gardenClient: c,
			projectQueue: queue,
		}
	})

	AfterEach(func() {
		ctrl.Finish()
		queue.ShutDown()
	})

	It("should not requeue random rolebinding", func() {
		controller.roleBindingDelete(ctx, rolebinding)

		Expect(queue.Len()).To(Equal(0), "no items in the queue")
	})

	DescribeTable("requeue when rolebinding is",
		func(roleBindingName string) {
			rolebinding.Name = roleBindingName

			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&gardencorev1beta1.ProjectList{}), client.MatchingFields{"spec.namespace": ns}).DoAndReturn(func(_ context.Context, list *gardencorev1beta1.ProjectList, _ ...client.ListOption) error {
				(&gardencorev1beta1.ProjectList{Items: []gardencorev1beta1.Project{*proj}}).DeepCopyInto(list)
				return nil
			})

			controller.roleBindingDelete(ctx, rolebinding)

			Expect(queue.Len()).To(Equal(1), "only one item in queue")
			actual, _ := queue.Get()
			Expect(actual).To(Equal("project-1"))
		},

		Entry("project-member", "gardener.cloud:system:project-member"),
		Entry("project-viewer", "gardener.cloud:system:project-viewer"),
		Entry("custom role", "gardener.cloud:extension:project:project-1:foo"),
	)

	DescribeTable("no requeue when project is being deleted and rolebinding is",
		func(roleBindingName string) {
			now := metav1.Now()
			proj.DeletionTimestamp = &now
			rolebinding.Name = roleBindingName

			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&gardencorev1beta1.ProjectList{}), client.MatchingFields{"spec.namespace": ns}).DoAndReturn(func(_ context.Context, list *gardencorev1beta1.ProjectList, _ ...client.ListOption) error {
				(&gardencorev1beta1.ProjectList{Items: []gardencorev1beta1.Project{*proj}}).DeepCopyInto(list)
				return nil
			})

			controller.roleBindingDelete(ctx, rolebinding)

			Expect(queue.Len()).To(Equal(0), "no projects in queue")
		},

		Entry("project-member", "gardener.cloud:system:project-member"),
		Entry("project-viewer", "gardener.cloud:system:project-viewer"),
		Entry("custom role", "gardener.cloud:extension:project:project-1:foo"),
	)
})

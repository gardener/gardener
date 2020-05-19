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
	"errors"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/gardener/gardener/pkg/operation/common"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("ProjectControlReconcile", func() {
	var (
		ctrl *gomock.Controller
		c    *mockclient.MockClient
		ctx  = context.TODO()
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#deleteStaleExtensionRoles", func() {
		var (
			namespace              = "namespace"
			projectName            = "name"
			prefix                 = "gardener.cloud:extension:project:" + projectName + ":"
			nonStaleExtensionRoles sets.String

			extensionRole1 = "foo"
			extensionRole2 = "bar"

			listOptions = []client.ListOption{
				client.InNamespace(namespace),
				client.MatchingLabels{
					v1beta1constants.GardenRole: v1beta1constants.LabelExtensionProjectRole,
					common.ProjectName:          projectName,
				},
			}

			rolebinding1 = rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      prefix + extensionRole1,
					Namespace: namespace,
				},
			}
			rolebinding2 = rbacv1.RoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      prefix + extensionRole2,
					Namespace: namespace,
				},
			}
			clusterrole1 = rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: prefix + extensionRole1,
				},
			}
			clusterrole2 = rbacv1.ClusterRole{
				ObjectMeta: metav1.ObjectMeta{
					Name: prefix + extensionRole2,
				},
			}

			err = errors.New("error")
		)

		It("should do nothing because neither rolebindings nor clusterroles exist", func() {
			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&rbacv1.RoleBindingList{}), listOptions).Return(nil)
			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&rbacv1.ClusterRoleList{}), listOptions).Return(nil)

			Expect(deleteStaleExtensionRoles(ctx, c, nonStaleExtensionRoles, projectName, namespace)).To(BeNil())
		})

		It("should return an error because listing the rolebindings failed", func() {
			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&rbacv1.RoleBindingList{}), listOptions).Return(err)

			Expect(deleteStaleExtensionRoles(ctx, c, nonStaleExtensionRoles, projectName, namespace)).To(Equal(err))
		})

		It("should return an error because listing the clusterroles failed", func() {
			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&rbacv1.RoleBindingList{}), listOptions).Return(nil)
			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&rbacv1.ClusterRoleList{}), listOptions).Return(err)

			Expect(deleteStaleExtensionRoles(ctx, c, nonStaleExtensionRoles, projectName, namespace)).To(Equal(err))
		})

		It("should return an error because deleting a stale rolebinding failed", func() {
			gomock.InOrder(
				c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&rbacv1.RoleBindingList{}), listOptions).DoAndReturn(func(_ context.Context, list *rbacv1.RoleBindingList, _ ...client.ListOption) error {
					list.Items = []rbacv1.RoleBinding{rolebinding1, rolebinding2}
					return nil
				}),
				c.EXPECT().Delete(ctx, &rolebinding1).Return(err),
			)

			Expect(deleteStaleExtensionRoles(ctx, c, sets.NewString(extensionRole2), projectName, namespace)).To(Equal(err))
		})

		It("should return an error because deleting a stale clusterrole failed", func() {
			gomock.InOrder(
				c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&rbacv1.RoleBindingList{}), listOptions).DoAndReturn(func(_ context.Context, list *rbacv1.RoleBindingList, _ ...client.ListOption) error {
					list.Items = []rbacv1.RoleBinding{rolebinding1, rolebinding2}
					return nil
				}),
				c.EXPECT().Delete(ctx, &rolebinding1),

				c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&rbacv1.ClusterRoleList{}), listOptions).DoAndReturn(func(_ context.Context, list *rbacv1.ClusterRoleList, _ ...client.ListOption) error {
					list.Items = []rbacv1.ClusterRole{clusterrole1, clusterrole2}
					return nil
				}),
				c.EXPECT().Delete(ctx, &clusterrole1).Return(err),
			)

			Expect(deleteStaleExtensionRoles(ctx, c, sets.NewString(extensionRole2), projectName, namespace)).To(Equal(err))
		})

		It("should succeed deleting the stale rolebindings and clusterroles", func() {
			gomock.InOrder(
				c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&rbacv1.RoleBindingList{}), listOptions).DoAndReturn(func(_ context.Context, list *rbacv1.RoleBindingList, _ ...client.ListOption) error {
					list.Items = []rbacv1.RoleBinding{rolebinding1, rolebinding2}
					return nil
				}),
				c.EXPECT().Delete(ctx, &rolebinding2),

				c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&rbacv1.ClusterRoleList{}), listOptions).DoAndReturn(func(_ context.Context, list *rbacv1.ClusterRoleList, _ ...client.ListOption) error {
					list.Items = []rbacv1.ClusterRole{clusterrole1, clusterrole2}
					return nil
				}),
				c.EXPECT().Delete(ctx, &clusterrole2),
			)

			Expect(deleteStaleExtensionRoles(ctx, c, sets.NewString(extensionRole1), projectName, namespace)).To(BeNil())
		})
	})
})

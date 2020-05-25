// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://wwr.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package genericactuator

import (
	"context"
	"errors"
	"fmt"

	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"

	"github.com/golang/mock/gomock"
	"github.com/hashicorp/go-multierror"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("Actuator", func() {
	Describe("#listMachineClassSecrets", func() {
		const (
			ns      = "test-ns"
			purpose = "machineclass"
		)

		var (
			existing    *corev1.Secret
			expected    corev1.Secret
			allExisting []runtime.Object
			allExpected []interface{}
		)

		BeforeEach(func() {
			existing = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "new",
					Namespace: ns,
					Labels:    map[string]string{},
				},
			}
			allExisting = []runtime.Object{}
			allExpected = []interface{}{}
			expected = *existing.DeepCopy()
		})

		AfterEach(func() {
			a := &genericActuator{client: fake.NewFakeClient(allExisting...)}
			sl, err := a.listMachineClassSecrets(context.TODO(), ns)
			Expect(err).ToNot(HaveOccurred())
			Expect(sl).ToNot(BeNil())
			Expect(sl.Items).To(ConsistOf(allExpected...))
		})

		It("only classes with new label exists", func() {
			existing.Labels["gardener.cloud/purpose"] = purpose
			expected = *existing.DeepCopy()

			allExisting = append(allExisting, existing)
			allExpected = append(allExpected, expected)
		})

		It("only classes with old label exists", func() {
			existing.Labels["garden.sapcloud.io/purpose"] = purpose
			expected := *existing.DeepCopy()

			allExisting = append(allExisting, existing)
			allExpected = append(allExpected, expected)
		})

		It("secret is labeled with both labels", func() {
			existing.Labels["garden.sapcloud.io/purpose"] = purpose
			existing.Labels["gardener.cloud/purpose"] = purpose
			expected := *existing.DeepCopy()

			allExisting = append(allExisting, existing)
			allExpected = append(allExpected, expected)
		})

		It("one old and one new secret exists", func() {
			oldExisting := existing.DeepCopy()
			oldExisting.Name = "old-deprecated"
			oldExisting.Labels["garden.sapcloud.io/purpose"] = purpose

			existing.Labels["gardener.cloud/purpose"] = purpose
			expected := *existing.DeepCopy()
			expectedOld := *oldExisting.DeepCopy()

			allExisting = append(allExisting, existing, oldExisting)
			allExpected = append(allExpected, expected, expectedOld)
		})
	})

	Describe("#CleanupLeakedClusterRoles", func() {
		var (
			ctrl *gomock.Controller

			ctx = context.TODO()
			c   *mockclient.MockClient

			providerName = "provider-foo"
			fakeErr      = errors.New("fake")

			namespace1              = "abcd"
			namespace2              = "efgh"
			namespace3              = "ijkl"
			nonMatchingClusterRoles = []rbacv1.ClusterRole{
				{ObjectMeta: metav1.ObjectMeta{Name: "doesnotmatch"}},
				{ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("extensions.gardener.cloud:provider-bar:%s:machine-controller-manager", namespace1)}},
				{ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("extensions.gardener.cloud:%s:%s", providerName, namespace1)}},
				{ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("extensions.gardener.cloud:%s:%s:bar", providerName, namespace1)}},
				{ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("extensions.gardener.cloud:%s:machine-controller-manager", providerName)}},
			}
		)

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
			c = mockclient.NewMockClient(ctrl)
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		It("should return an error while listing the clusterroles", func() {
			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&rbacv1.ClusterRoleList{})).Return(fakeErr)

			Expect(CleanupLeakedClusterRoles(ctx, c, providerName)).To(Equal(fakeErr))
		})

		It("should return an error while listing the namespaces", func() {
			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&rbacv1.ClusterRoleList{}))
			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&corev1.NamespaceList{})).Return(fakeErr)

			Expect(CleanupLeakedClusterRoles(ctx, c, providerName)).To(Equal(fakeErr))
		})

		It("should do nothing because clusterrole list is empty", func() {
			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&rbacv1.ClusterRoleList{}))
			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&corev1.NamespaceList{}))

			Expect(CleanupLeakedClusterRoles(ctx, c, providerName)).To(Succeed())
		})

		It("should do nothing because clusterrole list doesn't contain matches", func() {
			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&rbacv1.ClusterRoleList{})).DoAndReturn(func(_ context.Context, list *rbacv1.ClusterRoleList, _ ...client.ListOption) error {
				*list = rbacv1.ClusterRoleList{Items: nonMatchingClusterRoles}
				return nil
			})
			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&corev1.NamespaceList{}))

			Expect(CleanupLeakedClusterRoles(ctx, c, providerName)).To(Succeed())
		})

		It("should do nothing because no orphaned clusterroles found", func() {
			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&rbacv1.ClusterRoleList{})).DoAndReturn(func(_ context.Context, list *rbacv1.ClusterRoleList, _ ...client.ListOption) error {
				*list = rbacv1.ClusterRoleList{
					Items: append(nonMatchingClusterRoles, rbacv1.ClusterRole{
						ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("extensions.gardener.cloud:%s:%s:machine-controller-manager", providerName, namespace1)},
					}),
				}
				return nil
			})
			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&corev1.NamespaceList{})).DoAndReturn(func(_ context.Context, list *corev1.NamespaceList, _ ...client.ListOption) error {
				*list = corev1.NamespaceList{
					Items: []corev1.Namespace{
						{ObjectMeta: metav1.ObjectMeta{Name: namespace1}},
					},
				}
				return nil
			})

			Expect(CleanupLeakedClusterRoles(ctx, c, providerName)).To(Succeed())
		})

		It("should delete the orphaned clusterroles", func() {
			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&rbacv1.ClusterRoleList{})).DoAndReturn(func(_ context.Context, list *rbacv1.ClusterRoleList, _ ...client.ListOption) error {
				*list = rbacv1.ClusterRoleList{
					Items: append(
						nonMatchingClusterRoles,
						rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("extensions.gardener.cloud:%s:%s:machine-controller-manager", providerName, namespace1)}},
						rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("extensions.gardener.cloud:%s:%s:machine-controller-manager", providerName, namespace2)}},
						rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("extensions.gardener.cloud:%s:%s:machine-controller-manager", providerName, namespace3)}},
					),
				}
				return nil
			})
			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&corev1.NamespaceList{})).DoAndReturn(func(_ context.Context, list *corev1.NamespaceList, _ ...client.ListOption) error {
				*list = corev1.NamespaceList{
					Items: []corev1.Namespace{
						{ObjectMeta: metav1.ObjectMeta{Name: namespace1}},
					},
				}
				return nil
			})
			c.EXPECT().Delete(ctx, &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("extensions.gardener.cloud:%s:%s:machine-controller-manager", providerName, namespace2)}})
			c.EXPECT().Delete(ctx, &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("extensions.gardener.cloud:%s:%s:machine-controller-manager", providerName, namespace3)}})

			Expect(CleanupLeakedClusterRoles(ctx, c, providerName)).To(Succeed())
		})

		It("should return the error occurred during orphaned clusterrole deletion", func() {
			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&rbacv1.ClusterRoleList{})).DoAndReturn(func(_ context.Context, list *rbacv1.ClusterRoleList, _ ...client.ListOption) error {
				*list = rbacv1.ClusterRoleList{
					Items: append(
						nonMatchingClusterRoles,
						rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("extensions.gardener.cloud:%s:%s:machine-controller-manager", providerName, namespace1)}},
						rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("extensions.gardener.cloud:%s:%s:machine-controller-manager", providerName, namespace2)}},
						rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("extensions.gardener.cloud:%s:%s:machine-controller-manager", providerName, namespace3)}},
					),
				}
				return nil
			})
			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&corev1.NamespaceList{})).DoAndReturn(func(_ context.Context, list *corev1.NamespaceList, _ ...client.ListOption) error {
				*list = corev1.NamespaceList{
					Items: []corev1.Namespace{
						{ObjectMeta: metav1.ObjectMeta{Name: namespace1}},
					},
				}
				return nil
			})
			c.EXPECT().Delete(ctx, &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("extensions.gardener.cloud:%s:%s:machine-controller-manager", providerName, namespace2)}})
			c.EXPECT().Delete(ctx, &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("extensions.gardener.cloud:%s:%s:machine-controller-manager", providerName, namespace3)}}).Return(fakeErr)

			err := CleanupLeakedClusterRoles(ctx, c, providerName)

			Expect(err).To(HaveOccurred())
			Expect(err).To(BeAssignableToTypeOf(&multierror.Error{}))
			Expect(err.(*multierror.Error).Errors).To(Equal([]error{fakeErr}))
		})
	})
})

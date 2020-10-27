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

	kutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/gardener/gardener/pkg/operation/common"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("ProjectControlReconcile", func() {
	var (
		ctrl *gomock.Controller
		c    *mockclient.MockClient
		ctx  = context.TODO()

		namespace   = "namespace"
		projectName = "name"
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

	Describe("#quotaConfiguration", func() {
		var (
			project       *gardencorev1beta1.Project
			conf          config.ControllerManagerControllerConfiguration
			fooSelector   *metav1.LabelSelector
			resourceQuota *corev1.ResourceQuota
		)

		BeforeEach(func() {
			project = &gardencorev1beta1.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name:      projectName,
					Namespace: namespace,
					UID:       "1",
				},
			}
			conf = config.ControllerManagerControllerConfiguration{}
			fooSelector, _ = metav1.ParseToLabelSelector("role = foo")
			resourceQuota = &corev1.ResourceQuota{
				Spec: corev1.ResourceQuotaSpec{
					Hard: map[corev1.ResourceName]resource.Quantity{
						"count/foo": resource.MustParse("1"),
					},
				},
			}
		})

		It("should return no quota configuration because no project controller config is specified", func() {
			Expect(quotaConfiguration(conf, project)).To(BeNil())
		})
		It("should return no quota configuration because no quota config is specified", func() {
			conf.Project = &config.ProjectControllerConfiguration{}
			Expect(quotaConfiguration(conf, project)).To(BeNil())
		})
		It("should return no quota configuration because label selector does not match project", func() {
			conf.Project = &config.ProjectControllerConfiguration{
				Quotas: []config.QuotaConfiguration{
					{
						ProjectSelector: fooSelector,
					},
				},
			}
			Expect(quotaConfiguration(conf, project)).To(BeNil())
		})
		It("should return no quota configuration because label selector is invalid", func() {
			conf.Project = &config.ProjectControllerConfiguration{
				Quotas: []config.QuotaConfiguration{
					{
						ProjectSelector: &metav1.LabelSelector{
							MatchExpressions: []metav1.LabelSelectorRequirement{
								{},
							},
						},
					},
				},
			}
			quotaConf, err := quotaConfiguration(conf, project)
			Expect(err).To(HaveOccurred())
			Expect(quotaConf).To(BeNil())
		})
		It("should return no quota configuration because label selector is nil", func() {
			conf.Project = &config.ProjectControllerConfiguration{
				Quotas: []config.QuotaConfiguration{
					{
						ProjectSelector: nil,
					},
				},
			}
			Expect(quotaConfiguration(conf, project)).To(BeNil())
		})
		It("should return the quota configuration because label selector matches project", func() {
			conf.Project = &config.ProjectControllerConfiguration{
				Quotas: []config.QuotaConfiguration{
					{
						Config:          nil,
						ProjectSelector: fooSelector,
					},
					{
						Config:          resourceQuota,
						ProjectSelector: &metav1.LabelSelector{},
					},
				},
			}
			Expect(quotaConfiguration(conf, project)).To(Equal(&conf.Project.Quotas[1]))
		})
		It("should return the first matching quota configuration", func() {
			additionalQuota := *resourceQuota
			additionalQuota.Spec.Hard["count/bar"] = resource.MustParse("2")
			conf.Project = &config.ProjectControllerConfiguration{
				Quotas: []config.QuotaConfiguration{
					{
						Config:          nil,
						ProjectSelector: fooSelector,
					},
					{
						Config:          &additionalQuota,
						ProjectSelector: &metav1.LabelSelector{},
					},
					{
						Config:          resourceQuota,
						ProjectSelector: &metav1.LabelSelector{},
					},
				},
			}
			Expect(quotaConfiguration(conf, project)).To(Equal(&conf.Project.Quotas[1]))
		})
	})

	Describe("#createOrUpdateResourceQuota", func() {
		var (
			project       *gardencorev1beta1.Project
			ownerRef      *metav1.OwnerReference
			resourceQuota *corev1.ResourceQuota
			shoots        corev1.ResourceName
			secrets       corev1.ResourceName
			quantity      resource.Quantity
		)

		BeforeEach(func() {
			project = &gardencorev1beta1.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name:      projectName,
					Namespace: namespace,
					UID:       "1",
				},
			}
			ownerRef = metav1.NewControllerRef(project, gardencorev1beta1.SchemeGroupVersion.WithKind("Project"))
			shoots = "shoots.core.gardener.cloud"
			secrets = "secrets"
			quantity = resource.MustParse("10")
			resourceQuota = &corev1.ResourceQuota{
				Spec: corev1.ResourceQuotaSpec{
					Hard: map[corev1.ResourceName]resource.Quantity{
						shoots:  quantity,
						secrets: quantity,
					},
				},
			}
		})

		It("should create a new ResourceQuota", func() {
			config := config.QuotaConfiguration{
				Config: resourceQuota,
			}

			c.EXPECT().Get(ctx, kutils.Key(namespace, ResourceQuotaName), gomock.AssignableToTypeOf(&corev1.ResourceQuota{})).
				Return(apierrors.NewNotFound(corev1.Resource("resourcequota"), "resourcequota"))

			expectedResourceQuota := resourceQuota.DeepCopy()
			expectedResourceQuota.SetOwnerReferences([]metav1.OwnerReference{*ownerRef})
			expectedResourceQuota.SetName(ResourceQuotaName)
			expectedResourceQuota.SetNamespace(namespace)

			c.EXPECT().Create(ctx, expectedResourceQuota).Return(nil)

			Expect(createOrUpdateResourceQuota(ctx, c, namespace, ownerRef, config)).To(Succeed())
		})

		It("should update a existing ResourceQuota", func() {
			config := config.QuotaConfiguration{
				Config: resourceQuota,
			}

			existingOwnerRef := metav1.OwnerReference{Name: "foo"}
			existingResourceQuota := &corev1.ResourceQuota{
				ObjectMeta: metav1.ObjectMeta{
					Name:            ResourceQuotaName,
					Namespace:       namespace,
					OwnerReferences: []metav1.OwnerReference{existingOwnerRef},
				},
				Spec: corev1.ResourceQuotaSpec{
					Hard: map[corev1.ResourceName]resource.Quantity{
						shoots: resource.MustParse("50"),
					},
				},
			}

			c.EXPECT().Get(ctx, kutils.Key(namespace, ResourceQuotaName), gomock.AssignableToTypeOf(&corev1.ResourceQuota{})).
				DoAndReturn(func(_ context.Context, _ client.ObjectKey, resourceQuota *corev1.ResourceQuota) error {
					*resourceQuota = *existingResourceQuota
					return nil
				})

			expectedResourceQuota := existingResourceQuota.DeepCopy()
			expectedResourceQuota.SetOwnerReferences([]metav1.OwnerReference{existingOwnerRef, *ownerRef})
			expectedResourceQuota.Spec.Hard[secrets] = quantity

			c.EXPECT().Update(ctx, expectedResourceQuota).Return(nil)

			Expect(createOrUpdateResourceQuota(ctx, c, namespace, ownerRef, config)).To(Succeed())
		})

		It("should fail because invalid ResourceQuota specified", func() {
			config := config.QuotaConfiguration{
				Config: nil,
			}

			Expect(createOrUpdateResourceQuota(ctx, c, namespace, ownerRef, config)).NotTo(Succeed())
		})
	})
})

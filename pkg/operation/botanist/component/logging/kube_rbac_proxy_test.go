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

package logging_test

import (
	"context"
	"errors"
	"fmt"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	. "github.com/gardener/gardener/pkg/operation/botanist/component/logging"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/managedresources"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("KubeRBACProxy", func() {
	const (
		namespace = "shoot--test--namespace"
	)

	var (
		ctrl                  *gomock.Controller
		c                     *mockclient.MockClient
		ctx                   = context.TODO()
		kubeRBACProxyOptions  *KubeRBACProxyOptions
		kubeRBACPtoxyDeployer component.Deployer
		secretName            = managedresources.SecretName(ShootNodeLoggingManagedResourceName, true)
		fakeErr               = fmt.Errorf("fake error")
		kubeRBACProxyLabels   = map[string]string{"app": LokiKubeRBACProxyName}
		promtailLabels        = map[string]string{"app": PromtailName}
		keepObjects           = false
	)

	type newKubeRBACProxyArgs struct {
		so  *KubeRBACProxyOptions
		err error
	}

	DescribeTable("#NewKubeRBACProxy",
		func(args newKubeRBACProxyArgs) {
			deployer, err := NewKubeRBACProxy(args.so)
			if args.err != nil {
				Expect(err).To(Equal(args.err))
				Expect(deployer).To(BeNil())
			} else {
				Expect(err).To(BeNil())
				Expect(deployer).ToNot(BeNil())
			}
		},

		Entry("Pass nil options", newKubeRBACProxyArgs{
			so:  nil,
			err: errors.New("options cannot be nil"),
		}),
		Entry("Pass options with nil shoot client", newKubeRBACProxyArgs{
			so: &KubeRBACProxyOptions{
				Client:    nil,
				Namespace: namespace,
			},
			err: errors.New("client cannot be nil"),
		}),
		Entry("Pass options with empty", newKubeRBACProxyArgs{
			so: &KubeRBACProxyOptions{
				Client:    client.NewDryRunClient(nil),
				Namespace: "",
			},
			err: errors.New("namespace cannot be empty"),
		}),
		Entry("Pass valid options", newKubeRBACProxyArgs{
			so: &KubeRBACProxyOptions{
				Client:    client.NewDryRunClient(nil),
				Namespace: namespace,
			},
			err: nil,
		}),
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)
	})

	Describe("#Deploy", func() {
		var (
			secretToGet          *corev1.Secret
			managedResourceToGet *resourcesv1alpha1.ManagedResource
		)

		Context("Tests expecting a failure", func() {
			BeforeEach(func() {
				var err error
				kubeRBACProxyOptions = &KubeRBACProxyOptions{
					Client:                    c,
					Namespace:                 namespace,
					IsShootNodeLoggingEnabled: true,
				}
				kubeRBACPtoxyDeployer, err = NewKubeRBACProxy(kubeRBACProxyOptions)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should fail when the secret cannot be created", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, kutil.Key(namespace, secretName), gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).Return(fakeErr),
				)

				Expect(kubeRBACPtoxyDeployer.Deploy(ctx)).To(MatchError(fakeErr))
			})
			It("should fail when the managedresource cannot be created", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, kutil.Key(namespace, secretName), gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Get(ctx, kutil.Key(namespace, ShootNodeLoggingManagedResourceName), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Return(fakeErr),
				)

				Expect(kubeRBACPtoxyDeployer.Deploy(ctx)).To(MatchError(fakeErr))
			})
		})

		Context("Tests expecting a success", func() {
			BeforeEach(func() {
				secretToGet = &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: namespace},
				}
				managedResourceToGet = &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{Name: ShootNodeLoggingManagedResourceName, Namespace: namespace},
				}
			})

			It("should success when IsShootNodeLoggingEnabled flag is true", func() {
				kubeRBACProxyClusterRolebinding := &rbacv1.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:   KubeRBACProxyUserName,
						Labels: kubeRBACProxyLabels,
					},
					RoleRef: rbacv1.RoleRef{
						APIGroup: rbacv1.GroupName,
						Kind:     "ClusterRole",
						Name:     "system:auth-delegator",
					},
					Subjects: []rbacv1.Subject{{
						Kind: rbacv1.UserKind,
						Name: KubeRBACProxyUserName,
					}},
				}

				promtailClusterRoleBinding := &rbacv1.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:   PromtailRBACName,
						Labels: promtailLabels,
					},
					RoleRef: rbacv1.RoleRef{
						APIGroup: rbacv1.GroupName,
						Kind:     "ClusterRole",
						Name:     PromtailRBACName,
					},
					Subjects: []rbacv1.Subject{{
						Kind: rbacv1.UserKind,
						Name: PromtailRBACName,
					}},
				}

				promtailClusterRole := &rbacv1.ClusterRole{
					ObjectMeta: metav1.ObjectMeta{
						Name:   PromtailRBACName,
						Labels: promtailLabels,
					},
					Rules: []rbacv1.PolicyRule{
						{
							APIGroups: []string{
								"",
							},
							Resources: []string{
								"nodes",
								"nodes/proxy",
								"services",
								"endpoints",
								"pods",
							},
							Verbs: []string{
								"get",
								"watch",
								"list",
							},
						},
						{
							NonResourceURLs: []string{
								"/loki/api/v1/push",
							},
							Verbs: []string{
								"create",
							},
						},
					},
				}

				kubeRBACProxyOptions := &KubeRBACProxyOptions{
					Client:                    c,
					Namespace:                 namespace,
					IsShootNodeLoggingEnabled: true,
				}
				kubeRBACPtoxyDeployer, err := NewKubeRBACProxy(kubeRBACProxyOptions)
				Expect(err).ToNot(HaveOccurred())

				registry := managedresources.NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer)
				resources, err := registry.AddAllAndSerialize(kubeRBACProxyClusterRolebinding, promtailClusterRole, promtailClusterRoleBinding)
				Expect(err).ToNot(HaveOccurred())

				secretToUpdate := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      secretName,
						Namespace: namespace,
					},
					Data: resources,
					Type: corev1.SecretTypeOpaque,
				}

				managedResourceToUpdate := &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:      ShootNodeLoggingManagedResourceName,
						Namespace: namespace,
						Labels: map[string]string{
							"origin": "gardener",
						},
					},
					Spec: resourcesv1alpha1.ManagedResourceSpec{
						SecretRefs: []corev1.LocalObjectReference{
							{Name: secretName},
						},
						InjectLabels: map[string]string{
							"shoot.gardener.cloud/no-cleanup": "true",
						},
						KeepObjects: &keepObjects,
					},
				}

				gomock.InOrder(
					c.EXPECT().Get(ctx, kutil.Key(namespace, secretName), secretToGet),
					c.EXPECT().Update(ctx, secretToUpdate),
					c.EXPECT().Get(ctx, kutil.Key(namespace, ShootNodeLoggingManagedResourceName), managedResourceToGet),
					c.EXPECT().Update(ctx, managedResourceToUpdate),
				)

				Expect(kubeRBACPtoxyDeployer.Deploy(ctx)).To(Succeed())
			})

			It("should switch to Destroy when IsShootNodeLoggingEnabled flag is false", func() {
				kubeRBACProxyOptions := &KubeRBACProxyOptions{
					Client:                    c,
					Namespace:                 namespace,
					IsShootNodeLoggingEnabled: false,
				}
				kubeRBACPtoxyDeployer, err := NewKubeRBACProxy(kubeRBACProxyOptions)
				Expect(err).ToNot(HaveOccurred())

				gomock.InOrder(
					c.EXPECT().Delete(ctx, managedResourceToGet),
					c.EXPECT().Delete(ctx, secretToGet),
				)

				Expect(kubeRBACPtoxyDeployer.Deploy(ctx)).To(Succeed())
			})
		})
	})

	Describe("#Destroy", func() {
		var (
			secretToDelete          *corev1.Secret
			managedResourceToDelete *resourcesv1alpha1.ManagedResource
		)

		BeforeEach(func() {
			secretToDelete = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: secretName, Namespace: namespace},
			}
			managedResourceToDelete = &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{Name: ShootNodeLoggingManagedResourceName, Namespace: namespace},
			}
		})

		Context("Tests expecting a failure", func() {
			BeforeEach(func() {
				var err error
				kubeRBACProxyOptions = &KubeRBACProxyOptions{
					Client:                    c,
					Namespace:                 namespace,
					IsShootNodeLoggingEnabled: false,
				}
				kubeRBACPtoxyDeployer, err = NewKubeRBACProxy(kubeRBACProxyOptions)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should fail when the managed resource cannot be deleted", func() {
				gomock.InOrder(
					c.EXPECT().Delete(ctx, managedResourceToDelete).Return(fakeErr),
				)

				Expect(kubeRBACPtoxyDeployer.Destroy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail when the secret cannot be deleted", func() {
				gomock.InOrder(
					c.EXPECT().Delete(ctx, managedResourceToDelete),
					c.EXPECT().Delete(ctx, secretToDelete).Return(fakeErr),
				)

				Expect(kubeRBACPtoxyDeployer.Destroy(ctx)).To(MatchError(fakeErr))
			})
		})

		Context("Tests expecting a success", func() {
			It("Should delete successfully the managed resource and the secret", func() {
				kubeRBACProxyOptions := &KubeRBACProxyOptions{
					Client:                    c,
					Namespace:                 namespace,
					IsShootNodeLoggingEnabled: false,
				}
				kubeRBACPtoxyDeployer, err := NewKubeRBACProxy(kubeRBACProxyOptions)
				Expect(err).ToNot(HaveOccurred())

				gomock.InOrder(
					c.EXPECT().Delete(ctx, managedResourceToDelete),
					c.EXPECT().Delete(ctx, secretToDelete),
				)

				Expect(kubeRBACPtoxyDeployer.Destroy(ctx)).To(Succeed())
			})

			It("Should delete successfully the managed resource and the secret inspete of the IsShootNodeLoggingEnabled value", func() {
				kubeRBACProxyOptions := &KubeRBACProxyOptions{
					Client:                    c,
					Namespace:                 namespace,
					IsShootNodeLoggingEnabled: true,
				}
				kubeRBACPtoxyDeployer, err := NewKubeRBACProxy(kubeRBACProxyOptions)
				Expect(err).ToNot(HaveOccurred())

				gomock.InOrder(
					c.EXPECT().Delete(ctx, managedResourceToDelete),
					c.EXPECT().Delete(ctx, secretToDelete),
				)

				Expect(kubeRBACPtoxyDeployer.Destroy(ctx)).To(Succeed())
			})
		})
	})
})

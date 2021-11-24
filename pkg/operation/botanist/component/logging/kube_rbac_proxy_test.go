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
	. "github.com/gardener/gardener/pkg/utils/test/matchers"

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
		namespace           = "shoot--test--namespace"
		managedResourceName = "shoot-node-logging"
		kubeRBACProxyName   = "kube-rbac-proxy"
	)

	var (
		ctrl *gomock.Controller
		c    *mockclient.MockClient

		ctx     = context.TODO()
		fakeErr = fmt.Errorf("fake error")

		kubeRBACProxyDeployer component.Deployer
		kubeRBACProxyOptions  *Values

		managedResourceSecretName  = managedresources.SecretName(managedResourceName, true)
		shootAccessSecretName      = "shoot-access-kube-rbac-proxy"
		legacyKubeconfigSecretName = "kube-rbac-proxy-kubeconfig"

		kubeRBACProxyLabels = map[string]string{"app": kubeRBACProxyName}
		promtailLabels      = map[string]string{"app": PromtailName}
		keepObjects         = false

		legacyKubeconfigSecretToDelete *corev1.Secret
	)

	type newKubeRBACProxyArgs struct {
		so  *Values
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
			so: &Values{
				Client:    nil,
				Namespace: namespace,
			},
			err: errors.New("client cannot be nil"),
		}),
		Entry("Pass options with empty", newKubeRBACProxyArgs{
			so: &Values{
				Client:    client.NewDryRunClient(nil),
				Namespace: "",
			},
			err: errors.New("namespace cannot be empty"),
		}),
		Entry("Pass valid options", newKubeRBACProxyArgs{
			so: &Values{
				Client:    client.NewDryRunClient(nil),
				Namespace: namespace,
			},
			err: nil,
		}),
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)

		legacyKubeconfigSecretToDelete = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: legacyKubeconfigSecretName, Namespace: namespace},
		}
	})

	Describe("#Deploy", func() {
		var (
			managedResourceSecretToGet *corev1.Secret
			managedResourceToGet       *resourcesv1alpha1.ManagedResource
			shootAccessSecretToGet     *corev1.Secret
		)

		Context("Tests expecting a failure", func() {
			BeforeEach(func() {
				var err error
				kubeRBACProxyOptions = &Values{
					Client:    c,
					Namespace: namespace,
				}
				kubeRBACProxyDeployer, err = NewKubeRBACProxy(kubeRBACProxyOptions)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should fail when the shoot token secret cannot be created", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, kutil.Key(namespace, shootAccessSecretName), gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Secret{}), gomock.Any()).Return(fakeErr),
				)

				Expect(kubeRBACProxyDeployer.Deploy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail when the managed resource secret cannot be created", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, kutil.Key(namespace, shootAccessSecretName), gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Secret{}), gomock.Any()),
					c.EXPECT().Get(ctx, kutil.Key(namespace, managedResourceSecretName), gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).Return(fakeErr),
				)

				Expect(kubeRBACProxyDeployer.Deploy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail when the managedresource cannot be created", func() {
				gomock.InOrder(
					c.EXPECT().Get(ctx, kutil.Key(namespace, shootAccessSecretName), gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Secret{}), gomock.Any()),
					c.EXPECT().Get(ctx, kutil.Key(namespace, managedResourceSecretName), gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})),
					c.EXPECT().Get(ctx, kutil.Key(namespace, managedResourceName), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Return(fakeErr),
				)

				Expect(kubeRBACProxyDeployer.Deploy(ctx)).To(MatchError(fakeErr))
			})
		})

		Context("Tests expecting a success", func() {
			BeforeEach(func() {
				managedResourceSecretToGet = &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: managedResourceSecretName, Namespace: namespace},
				}
				managedResourceToGet = &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{Name: managedResourceName, Namespace: namespace},
				}
				shootAccessSecretToGet = &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: shootAccessSecretName, Namespace: namespace},
				}
			})

			It("should succeed", func() {
				kubeRBACProxyClusterRoleBinding := &rbacv1.ClusterRoleBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "gardener.cloud:logging:kube-rbac-proxy",
						Labels: kubeRBACProxyLabels,
					},
					RoleRef: rbacv1.RoleRef{
						APIGroup: rbacv1.GroupName,
						Kind:     "ClusterRole",
						Name:     "system:auth-delegator",
					},
					Subjects: []rbacv1.Subject{{
						Kind:      rbacv1.ServiceAccountKind,
						Name:      "kube-rbac-proxy",
						Namespace: "kube-system",
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

				kubeRBACProxyOptions := &Values{
					Client:    c,
					Namespace: namespace,
				}
				kubeRBACProxyDeployer, err := NewKubeRBACProxy(kubeRBACProxyOptions)
				Expect(err).ToNot(HaveOccurred())

				registry := managedresources.NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer)
				resources, err := registry.AddAllAndSerialize(kubeRBACProxyClusterRoleBinding, promtailClusterRole, promtailClusterRoleBinding)
				Expect(err).ToNot(HaveOccurred())

				managedResourceSecretToUpdate := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      managedResourceSecretName,
						Namespace: namespace,
					},
					Data: resources,
					Type: corev1.SecretTypeOpaque,
				}

				managedResourceToUpdate := &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:      managedResourceName,
						Namespace: namespace,
						Labels: map[string]string{
							"origin":   "gardener",
							"priority": "normal",
						},
					},
					Spec: resourcesv1alpha1.ManagedResourceSpec{
						SecretRefs: []corev1.LocalObjectReference{
							{Name: managedResourceSecretName},
						},
						InjectLabels: map[string]string{
							"shoot.gardener.cloud/no-cleanup": "true",
						},
						KeepObjects: &keepObjects,
					},
				}
				shootAccessSecretToPatch := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      shootAccessSecretName,
						Namespace: namespace,
						Annotations: map[string]string{
							"serviceaccount.resources.gardener.cloud/name":      "kube-rbac-proxy",
							"serviceaccount.resources.gardener.cloud/namespace": "kube-system",
						},
						Labels: map[string]string{
							"resources.gardener.cloud/purpose": "token-requestor",
						},
					},
					Type: corev1.SecretTypeOpaque,
				}

				gomock.InOrder(
					c.EXPECT().Get(ctx, kutil.Key(namespace, shootAccessSecretName), shootAccessSecretToGet),
					c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Secret{}), gomock.Any()).
						Do(func(ctx context.Context, obj client.Object, _ client.Patch, _ ...client.PatchOption) {
							Expect(obj).To(DeepEqual(shootAccessSecretToPatch))
						}),
					c.EXPECT().Get(ctx, kutil.Key(namespace, managedResourceSecretName), managedResourceSecretToGet),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).
						Do(func(ctx context.Context, obj client.Object, _ ...client.UpdateOption) {
							Expect(obj).To(DeepEqual(managedResourceSecretToUpdate))
						}),
					c.EXPECT().Get(ctx, kutil.Key(namespace, managedResourceName), managedResourceToGet),
					c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).
						Do(func(ctx context.Context, obj client.Object, _ ...client.UpdateOption) {
							Expect(obj).To(DeepEqual(managedResourceToUpdate))
						}),
					c.EXPECT().Delete(ctx, legacyKubeconfigSecretToDelete),
				)

				Expect(kubeRBACProxyDeployer.Deploy(ctx)).To(Succeed())
			})
		})
	})

	Describe("#Destroy", func() {
		var (
			managedResourceSecretToDelete *corev1.Secret
			managedResourceToDelete       *resourcesv1alpha1.ManagedResource
			shootAccessSecretToDelete     *corev1.Secret
		)

		BeforeEach(func() {
			managedResourceSecretToDelete = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: managedResourceSecretName, Namespace: namespace},
			}
			managedResourceToDelete = &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{Name: managedResourceName, Namespace: namespace},
			}
			shootAccessSecretToDelete = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: shootAccessSecretName, Namespace: namespace},
			}
		})

		Context("Tests expecting a failure", func() {
			BeforeEach(func() {
				var err error
				kubeRBACProxyOptions = &Values{
					Client:    c,
					Namespace: namespace,
				}
				kubeRBACProxyDeployer, err = NewKubeRBACProxy(kubeRBACProxyOptions)
				Expect(err).ToNot(HaveOccurred())
			})

			It("should fail when the managed resource cannot be deleted", func() {
				gomock.InOrder(
					c.EXPECT().Delete(ctx, managedResourceToDelete).Return(fakeErr),
				)

				Expect(kubeRBACProxyDeployer.Destroy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail when the managed resource secret cannot be deleted", func() {
				gomock.InOrder(
					c.EXPECT().Delete(ctx, managedResourceToDelete),
					c.EXPECT().Delete(ctx, managedResourceSecretToDelete).Return(fakeErr),
				)

				Expect(kubeRBACProxyDeployer.Destroy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail when the shoot token secret cannot be deleted", func() {
				gomock.InOrder(
					c.EXPECT().Delete(ctx, managedResourceToDelete),
					c.EXPECT().Delete(ctx, managedResourceSecretToDelete),
					c.EXPECT().Delete(ctx, shootAccessSecretToDelete).Return(fakeErr),
				)

				Expect(kubeRBACProxyDeployer.Destroy(ctx)).To(MatchError(fakeErr))
			})

			It("should fail when the legacy kubeconfig secret cannot be deleted", func() {
				gomock.InOrder(
					c.EXPECT().Delete(ctx, managedResourceToDelete),
					c.EXPECT().Delete(ctx, managedResourceSecretToDelete),
					c.EXPECT().Delete(ctx, shootAccessSecretToDelete),
					c.EXPECT().Delete(ctx, legacyKubeconfigSecretToDelete).Return(fakeErr),
				)

				Expect(kubeRBACProxyDeployer.Destroy(ctx)).To(MatchError(fakeErr))
			})
		})

		Context("Tests expecting a success", func() {
			It("Should delete successfully the managed resource and the secret", func() {
				kubeRBACProxyOptions := &Values{
					Client:    c,
					Namespace: namespace,
				}
				kubeRBACProxyDeployer, err := NewKubeRBACProxy(kubeRBACProxyOptions)
				Expect(err).ToNot(HaveOccurred())

				gomock.InOrder(
					c.EXPECT().Delete(ctx, managedResourceToDelete),
					c.EXPECT().Delete(ctx, managedResourceSecretToDelete),
					c.EXPECT().Delete(ctx, shootAccessSecretToDelete),
					c.EXPECT().Delete(ctx, legacyKubeconfigSecretToDelete),
				)

				Expect(kubeRBACProxyDeployer.Destroy(ctx)).To(Succeed())
			})
		})
	})
})

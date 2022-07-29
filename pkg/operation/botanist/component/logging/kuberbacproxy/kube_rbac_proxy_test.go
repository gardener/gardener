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

package kuberbacproxy_test

import (
	"context"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	. "github.com/gardener/gardener/pkg/operation/botanist/component/logging/kuberbacproxy"
	"github.com/gardener/gardener/pkg/operation/botanist/component/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("KubeRBACProxy", func() {
	const (
		namespace           = "shoot--foo--bar"
		managedResourceName = "shoot-node-logging"
		kubeRBACProxyName   = "kube-rbac-proxy"
		promtailName        = "gardener-promtail"
	)

	var (
		c                     client.Client
		ctx                   = context.TODO()
		kubeRBACProxyDeployer component.Deployer
		managedResource       *resourcesv1alpha1.ManagedResource
		managedResourceSecret *corev1.Secret
	)

	type newKubeRBACProxyArgs struct {
		client    client.Client
		namespace string
	}

	DescribeTable("#New",
		func(args newKubeRBACProxyArgs, matchError, matchDeployer types.GomegaMatcher) {
			deployer, err := New(args.client, args.namespace)

			Expect(err).To(matchError)
			Expect(deployer).To(matchDeployer)
		},
		Entry("pass options with nil shoot client", newKubeRBACProxyArgs{
			client:    nil,
			namespace: namespace,
		},
			MatchError(ContainSubstring("client cannot be nil")),
			BeNil()),
		Entry("pass options with empty", newKubeRBACProxyArgs{
			client:    client.NewDryRunClient(nil),
			namespace: "",
		},
			MatchError(ContainSubstring("namespace cannot be empty")),
			BeNil()),
		Entry("pass valid options", newKubeRBACProxyArgs{
			client:    client.NewDryRunClient(nil),
			namespace: namespace,
		},
			BeNil(),
			Not(BeNil())),
	)

	BeforeEach(func() {
		var err error
		c = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()

		kubeRBACProxyDeployer, err = New(c, namespace)
		Expect(err).ToNot(HaveOccurred())

		By("creating secrets managed outside of this package for which secretsmanager.Get() will be called")
		Expect(c.Create(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "generic-token-kubeconfig", Namespace: namespace}})).To(Succeed())
	})

	JustBeforeEach(func() {
		managedResource = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      managedResourceName,
				Namespace: namespace,
			},
		}
		managedResourceSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "managedresource-" + managedResource.Name,
				Namespace: namespace,
			},
		}
	})

	Describe("#Deploy", func() {
		It("should successfully deploy all resources", func() {
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(BeNotFoundError())

			Expect(kubeRBACProxyDeployer.Deploy(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
			Expect(managedResource).To(DeepEqual(&resourcesv1alpha1.ManagedResource{
				TypeMeta: metav1.TypeMeta{
					APIVersion: resourcesv1alpha1.SchemeGroupVersion.String(),
					Kind:       "ManagedResource",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:            managedResourceName,
					Namespace:       namespace,
					ResourceVersion: "1",
					Labels:          map[string]string{"origin": "gardener"},
				},
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					InjectLabels: map[string]string{"shoot.gardener.cloud/no-cleanup": "true"},
					SecretRefs: []corev1.LocalObjectReference{{
						Name: managedResourceSecret.Name,
					}},
					KeepObjects: pointer.Bool(false),
				},
			}))

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())
			Expect(managedResourceSecret.Type).To(Equal(corev1.SecretTypeOpaque))
			Expect(managedResourceSecret.Data).To(HaveLen(3))

			Expect(string(managedResourceSecret.Data["clusterrole____gardener.cloud_logging_promtail.yaml"])).To(Equal(test.Serialize(&rbacv1.ClusterRole{
				TypeMeta: metav1.TypeMeta{
					APIVersion: rbacv1.SchemeGroupVersion.String(),
					Kind:       "ClusterRole",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "gardener.cloud:logging:promtail",
					Labels: map[string]string{
						"app": promtailName,
					},
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
							"list",
							"watch",
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
			})))
			Expect(string(managedResourceSecret.Data["clusterrolebinding____gardener.cloud_logging_kube-rbac-proxy.yaml"])).To(Equal(test.Serialize(&rbacv1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "gardener.cloud:logging:kube-rbac-proxy",
					Labels: map[string]string{
						"app": kubeRBACProxyName,
					},
				},
				RoleRef: rbacv1.RoleRef{
					APIGroup: rbacv1.GroupName,
					Kind:     "ClusterRole",
					Name:     "system:auth-delegator",
				},
				Subjects: []rbacv1.Subject{{
					Kind:      rbacv1.ServiceAccountKind,
					Name:      kubeRBACProxyName,
					Namespace: metav1.NamespaceSystem,
				}},
			})))
			Expect(string(managedResourceSecret.Data["clusterrolebinding____gardener.cloud_logging_promtail.yaml"])).To(Equal(test.Serialize(&rbacv1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: "gardener.cloud:logging:promtail",
					Labels: map[string]string{
						"app": promtailName,
					},
				},
				RoleRef: rbacv1.RoleRef{
					APIGroup: rbacv1.GroupName,
					Kind:     "ClusterRole",
					Name:     "gardener.cloud:logging:promtail",
				},
				Subjects: []rbacv1.Subject{{
					Kind:      rbacv1.ServiceAccountKind,
					Name:      promtailName,
					Namespace: metav1.NamespaceSystem,
				}},
			})))
		})
	})

	Describe("#Destroy", func() {
		It("should successfully destroy all resources", func() {
			Expect(c.Create(ctx, managedResource)).To(Succeed())
			Expect(c.Create(ctx, managedResourceSecret)).To(Succeed())

			Expect(kubeRBACProxyDeployer.Destroy(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(BeNotFoundError())
		})
	})
})

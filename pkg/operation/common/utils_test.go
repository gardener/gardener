// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package common_test

import (
	"context"
	"fmt"
	"net"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/gardener/gardener/pkg/operation/botanist/component/logging"
	. "github.com/gardener/gardener/pkg/operation/common"

	hvpav1alpha1 "github.com/gardener/hvpa-controller/api/v1alpha1"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	schedulingv1 "k8s.io/api/scheduling/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("common", func() {
	Describe("utils", func() {
		Describe("#ComputeOffsetIP", func() {
			Context("IPv4", func() {
				It("should return a cluster IPv4 IP", func() {
					_, subnet, _ := net.ParseCIDR("100.64.0.0/13")
					result, err := ComputeOffsetIP(subnet, 10)

					Expect(err).NotTo(HaveOccurred())

					Expect(result).To(HaveLen(net.IPv4len))
					Expect(result).To(Equal(net.ParseIP("100.64.0.10").To4()))
				})

				It("should return error if subnet nil is passed", func() {
					result, err := ComputeOffsetIP(nil, 10)

					Expect(err).To(HaveOccurred())
					Expect(result).To(BeNil())
				})

				It("should return error if subnet is not big enough is passed", func() {
					_, subnet, _ := net.ParseCIDR("100.64.0.0/32")
					result, err := ComputeOffsetIP(subnet, 10)

					Expect(err).To(HaveOccurred())
					Expect(result).To(BeNil())
				})

				It("should return error if ip address is broadcast ip", func() {
					_, subnet, _ := net.ParseCIDR("10.0.0.0/24")
					result, err := ComputeOffsetIP(subnet, 255)

					Expect(err).To(HaveOccurred())
					Expect(result).To(BeNil())
				})
			})

			Context("IPv6", func() {
				It("should return a cluster IPv6 IP", func() {
					_, subnet, _ := net.ParseCIDR("fc00::/8")
					result, err := ComputeOffsetIP(subnet, 10)

					Expect(err).NotTo(HaveOccurred())
					Expect(result).To(HaveLen(net.IPv6len))
					Expect(result).To(Equal(net.ParseIP("fc00::a")))
				})

				It("should return error if subnet nil is passed", func() {
					result, err := ComputeOffsetIP(nil, 10)

					Expect(err).To(HaveOccurred())
					Expect(result).To(BeNil())
				})

				It("should return error if subnet is not big enough is passed", func() {
					_, subnet, _ := net.ParseCIDR("fc00::/128")
					result, err := ComputeOffsetIP(subnet, 10)

					Expect(err).To(HaveOccurred())
					Expect(result).To(BeNil())
				})
			})
		})

		Describe("#GenerateAddonConfig", func() {
			Context("values=nil and enabled=false", func() {
				It("should return a map with key enabled=false", func() {
					var (
						values  map[string]interface{}
						enabled = false
					)

					result := GenerateAddonConfig(values, enabled)

					Expect(result).To(SatisfyAll(
						HaveKeyWithValue("enabled", enabled),
						HaveLen(1),
					))
				})
			})

			Context("values=nil and enabled=true", func() {
				It("should return a map with key enabled=true", func() {
					var (
						values  map[string]interface{}
						enabled = true
					)

					result := GenerateAddonConfig(values, enabled)

					Expect(result).To(SatisfyAll(
						HaveKeyWithValue("enabled", enabled),
						HaveLen(1),
					))
				})
			})

			Context("values=<empty map> and enabled=true", func() {
				It("should return a map with key enabled=true", func() {
					var (
						values  = map[string]interface{}{}
						enabled = true
					)

					result := GenerateAddonConfig(values, enabled)

					Expect(result).To(SatisfyAll(
						HaveKeyWithValue("enabled", enabled),
						HaveLen(1),
					))
				})
			})

			Context("values=<non-empty map> and enabled=true", func() {
				It("should return a map with the values and key enabled=true", func() {
					var (
						values = map[string]interface{}{
							"foo": "bar",
						}
						enabled = true
					)

					result := GenerateAddonConfig(values, enabled)

					for key := range values {
						_, ok := result[key]
						Expect(ok).To(BeTrue())
					}
					Expect(result).To(SatisfyAll(
						HaveKeyWithValue("enabled", enabled),
						HaveLen(1+len(values)),
					))
				})
			})

			Context("values=<non-empty map> and enabled=false", func() {
				It("should return a map with key enabled=false", func() {
					var (
						values = map[string]interface{}{
							"foo": "bar",
						}
						enabled = false
					)

					result := GenerateAddonConfig(values, enabled)

					Expect(result).To(SatisfyAll(
						HaveKeyWithValue("enabled", enabled),
						HaveLen(1),
					))
				})
			})
		})
	})

	Describe("#DeleteDeploymentsHavingDeprecatedRoleLabelKey", func() {
		var (
			ctrl *gomock.Controller
			c    *mockclient.MockClient

			ctx     context.Context
			deploy1 *appsv1.Deployment
			deploy2 *appsv1.Deployment
			key1    client.ObjectKey
			key2    client.ObjectKey
		)

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
			c = mockclient.NewMockClient(ctrl)

			ctx = context.TODO()
			deploy1 = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: v1beta1constants.GardenNamespace,
				},
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"app": "foo"},
					},
				},
			}
			deploy2 = &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "bar",
					Namespace: v1beta1constants.GardenNamespace,
				},
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"app": "bar"},
					},
				},
			}
			key1 = client.ObjectKey{Name: deploy1.Name, Namespace: deploy1.Namespace}
			key2 = client.ObjectKey{Name: deploy2.Name, Namespace: deploy2.Namespace}
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		It("should return error if error occurs during get of deployment", func() {
			fakeErr := fmt.Errorf("fake err")

			c.EXPECT().Get(ctx, key1, gomock.AssignableToTypeOf(&appsv1.Deployment{})).Return(fakeErr)

			err := DeleteDeploymentsHavingDeprecatedRoleLabelKey(ctx, c, []client.ObjectKey{key1, key2})
			Expect(err).To(MatchError(fakeErr))
		})

		It("should do nothing when the deployments are missing", func() {
			c.EXPECT().Get(ctx, key1, gomock.AssignableToTypeOf(&appsv1.Deployment{})).
				Return(apierrors.NewNotFound(appsv1.Resource("Deployment"), deploy1.Name))
			c.EXPECT().Get(ctx, key2, gomock.AssignableToTypeOf(&appsv1.Deployment{})).
				Return(apierrors.NewNotFound(appsv1.Resource("Deployment"), deploy2.Name))

			err := DeleteDeploymentsHavingDeprecatedRoleLabelKey(ctx, c, []client.ObjectKey{key1, key2})
			Expect(err).NotTo(HaveOccurred())
		})

		It("should do nothing when .spec.selector does not have the label key", func() {
			c.EXPECT().Get(ctx, key1, gomock.AssignableToTypeOf(&appsv1.Deployment{})).SetArg(2, *deploy1)
			c.EXPECT().Get(ctx, key2, gomock.AssignableToTypeOf(&appsv1.Deployment{})).SetArg(2, *deploy2)

			err := DeleteDeploymentsHavingDeprecatedRoleLabelKey(ctx, c, []client.ObjectKey{key1, key2})
			Expect(err).NotTo(HaveOccurred())
		})

		It("should delete the deployments when .spec.selector has the label key", func() {
			labelSelector := &metav1.LabelSelector{
				MatchLabels: map[string]string{v1beta1constants.DeprecatedGardenRole: "bar"},
			}
			deploy1.Spec.Selector = labelSelector
			deploy2.Spec.Selector = labelSelector

			gomock.InOrder(
				// deploy1
				c.EXPECT().Get(ctx, key1, gomock.AssignableToTypeOf(&appsv1.Deployment{})).SetArg(2, *deploy1),
				c.EXPECT().Delete(ctx, deploy1),
				c.EXPECT().Get(ctx, key1, gomock.AssignableToTypeOf(&appsv1.Deployment{})).SetArg(2, *deploy1).
					Return(apierrors.NewNotFound(appsv1.Resource("Deployment"), deploy1.Name)),
				// deploy2
				c.EXPECT().Get(ctx, key2, gomock.AssignableToTypeOf(&appsv1.Deployment{})).SetArg(2, *deploy2),
				c.EXPECT().Delete(ctx, deploy2),
				c.EXPECT().Get(ctx, key2, gomock.AssignableToTypeOf(&appsv1.Deployment{})).SetArg(2, *deploy2).
					Return(apierrors.NewNotFound(appsv1.Resource("Deployment"), deploy2.Name)),
			)

			err := DeleteDeploymentsHavingDeprecatedRoleLabelKey(ctx, c, []client.ObjectKey{key1, key2})
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("#DeleteSeedLoggingStack", func() {
		var (
			ctrl *gomock.Controller
			c    *mockclient.MockClient
			ctx  context.Context
		)

		resources := []client.Object{
			//seed components
			&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "fluent-bit-config", Namespace: v1beta1constants.GardenNamespace}},
			&appsv1.DaemonSet{ObjectMeta: metav1.ObjectMeta{Name: "fluent-bit", Namespace: v1beta1constants.GardenNamespace}},
			&networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-fluentbit", Namespace: v1beta1constants.GardenNamespace}},
			&schedulingv1.PriorityClass{ObjectMeta: metav1.ObjectMeta{Name: "fluent-bit"}},
			&schedulingv1.PriorityClass{ObjectMeta: metav1.ObjectMeta{Name: "loki"}},
			&schedulingv1.PriorityClass{ObjectMeta: metav1.ObjectMeta{Name: GardenLokiPriorityClassName}},
			&rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "fluent-bit-read"}},
			&rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "fluent-bit-read"}},
			&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "fluent-bit", Namespace: v1beta1constants.GardenNamespace}},
			&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "fluent-bit", Namespace: v1beta1constants.GardenNamespace}},
			//shoot components
			&networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-loki", Namespace: v1beta1constants.GardenNamespace}},
			&networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-to-loki", Namespace: v1beta1constants.GardenNamespace}},
			&hvpav1alpha1.Hvpa{ObjectMeta: metav1.ObjectMeta{Name: "loki", Namespace: v1beta1constants.GardenNamespace}},
			&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "loki-config", Namespace: v1beta1constants.GardenNamespace}},
			&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "loki", Namespace: v1beta1constants.GardenNamespace}},
			&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "loki", Namespace: v1beta1constants.GardenNamespace}},
			&appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "loki", Namespace: v1beta1constants.GardenNamespace}},
			&corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "loki-loki-0", Namespace: v1beta1constants.GardenNamespace}},
		}

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
			c = mockclient.NewMockClient(ctrl)

			ctx = context.TODO()
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		It("should delete all seed logging stack components", func() {
			for _, resource := range resources {
				c.EXPECT().Delete(ctx, resource)
			}

			err := DeleteSeedLoggingStack(ctx, c)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("#DeleteShootLoggingStack", func() {
		var (
			ctrl *gomock.Controller
			c    *mockclient.MockClient
			ctx  context.Context
		)

		resources := []client.Object{
			//shoot components
			&networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-loki", Namespace: v1beta1constants.GardenNamespace}},
			&networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-to-loki", Namespace: v1beta1constants.GardenNamespace}},
			&hvpav1alpha1.Hvpa{ObjectMeta: metav1.ObjectMeta{Name: "loki", Namespace: v1beta1constants.GardenNamespace}},
			&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "loki-config", Namespace: v1beta1constants.GardenNamespace}},
			&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "loki", Namespace: v1beta1constants.GardenNamespace}},
			&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "loki", Namespace: v1beta1constants.GardenNamespace}},
			&appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "loki", Namespace: v1beta1constants.GardenNamespace}},
			&corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "loki-loki-0", Namespace: v1beta1constants.GardenNamespace}},
			&networkingv1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: "loki", Namespace: v1beta1constants.GardenNamespace}},
			&extensionsv1beta1.Ingress{ObjectMeta: metav1.ObjectMeta{Name: "loki", Namespace: v1beta1constants.GardenNamespace}},
			&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: logging.SecretNameLokiKubeRBACProxyKubeconfig, Namespace: v1beta1constants.GardenNamespace}},
			&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: LokiTLS, Namespace: v1beta1constants.GardenNamespace}},
			&networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-from-prometheus-to-loki-telegraf", Namespace: v1beta1constants.GardenNamespace}},
			&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "telegraf-config", Namespace: v1beta1constants.GardenNamespace}},
		}

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
			c = mockclient.NewMockClient(ctrl)

			ctx = context.TODO()
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		It("should delete all shoot logging stack components", func() {
			for _, resource := range resources {
				c.EXPECT().Delete(ctx, resource)
			}

			err := DeleteShootLoggingStack(ctx, c, v1beta1constants.GardenNamespace)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("#FilterEntriesByPrefix", func() {
		var (
			prefix  string
			entries []string
		)

		BeforeEach(func() {
			prefix = "role"
			entries = []string{
				"foo",
				"bar",
			}
		})

		It("should only return entries with prefix", func() {
			expectedEntries := []string{
				fmt.Sprintf("%s-%s", prefix, "foo"),
				fmt.Sprintf("%s-%s", prefix, "bar"),
			}

			entries = append(entries, expectedEntries...)

			result := FilterEntriesByPrefix(prefix, entries)
			Expect(result).To(ContainElements(expectedEntries))
		})

		It("should return all entries", func() {
			expectedEntries := []string{
				fmt.Sprintf("%s-%s", prefix, "foo"),
				fmt.Sprintf("%s-%s", prefix, "bar"),
			}

			entries = expectedEntries

			result := FilterEntriesByPrefix(prefix, entries)
			Expect(result).To(ContainElements(expectedEntries))
		})

		It("should return no entries", func() {
			result := FilterEntriesByPrefix(prefix, entries)
			Expect(result).To(BeEmpty())
		})
	})
})

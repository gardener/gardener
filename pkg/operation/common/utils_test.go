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
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	mocktime "github.com/gardener/gardener/pkg/mock/go/time"
	. "github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils/test"

	hvpav1alpha1 "github.com/gardener/hvpa-controller/api/v1alpha1"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
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

	DescribeTable("#ReplaceCloudProviderConfigKey",
		func(key, oldValue, newValue string) {
			var (
				separator = ": "

				configWithoutQuotes = fmt.Sprintf("%s%s%s", key, separator, oldValue)
				configWithQuotes    = fmt.Sprintf("%s%s\"%s\"", key, separator, strings.Replace(oldValue, `"`, `\"`, -1))
				expected            = fmt.Sprintf("%s%s\"%s\"", key, separator, strings.Replace(newValue, `"`, `\"`, -1))
			)

			Expect(ReplaceCloudProviderConfigKey(configWithoutQuotes, separator, key, newValue)).To(Equal(expected))
			Expect(ReplaceCloudProviderConfigKey(configWithQuotes, separator, key, newValue)).To(Equal(expected))
		},

		Entry("no special characters", "foo", "bar", "baz"),
		Entry("no special characters", "foo", "bar", "baz"),
		Entry("with special characters", "foo", `C*ko4P++$"x`, `"$++*ab*$c4k`),
		Entry("with special characters", "foo", "P+*4", `P*$8uOkv6+4`),
	)

	DescribeTable("#GetDomainInfoFromAnnotations",
		func(annotations map[string]string, expectedProvider, expectedDomain, expectedIncludeZones, expectedExcludeZones, expectedErr gomegatypes.GomegaMatcher) {
			provider, domain, includeZones, excludeZones, err := GetDomainInfoFromAnnotations(annotations)
			Expect(provider).To(expectedProvider)
			Expect(domain).To(expectedDomain)
			Expect(includeZones).To(expectedIncludeZones)
			Expect(excludeZones).To(expectedExcludeZones)
			Expect(err).To(expectedErr)
		},

		Entry("no annotations", nil, BeEmpty(), BeEmpty(), BeEmpty(), BeEmpty(), HaveOccurred()),
		Entry("no domain", map[string]string{
			DNSProvider: "bar",
		}, BeEmpty(), BeEmpty(), BeEmpty(), BeEmpty(), HaveOccurred()),
		Entry("no provider", map[string]string{
			DNSProvider: "bar",
		}, BeEmpty(), BeEmpty(), BeEmpty(), BeEmpty(), HaveOccurred()),
		Entry("all present", map[string]string{
			DNSProvider:     "bar",
			DNSDomain:       "foo",
			DNSIncludeZones: "a,b,c",
			DNSExcludeZones: "d,e,f",
		}, Equal("bar"), Equal("foo"), Equal([]string{"a", "b", "c"}), Equal([]string{"d", "e", "f"}), Not(HaveOccurred())),
	)

	Describe("#CheckIfDeletionIsConfirmed", func() {
		It("should prevent the deletion due to missing annotations", func() {
			obj := &corev1.Namespace{}

			Expect(CheckIfDeletionIsConfirmed(obj)).To(HaveOccurred())
		})

		It("should prevent the deletion due annotation value != true", func() {
			obj := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						ConfirmationDeletion: "false",
					},
				},
			}

			Expect(CheckIfDeletionIsConfirmed(obj)).To(HaveOccurred())
		})

		It("should allow the deletion due annotation value == true", func() {
			obj := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						ConfirmationDeletion: "true",
					},
				},
			}

			Expect(CheckIfDeletionIsConfirmed(obj)).To(Succeed())
		})
	})

	Describe("#ConfirmDeletion", func() {
		var (
			ctrl    *gomock.Controller
			c       *mockclient.MockClient
			now     time.Time
			mockNow *mocktime.MockNow
		)

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
			mockNow = mocktime.NewMockNow(ctrl)
			c = mockclient.NewMockClient(ctrl)
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		It("should add the deletion confirmation annotation for an object without annotations", func() {
			var (
				ctx = context.TODO()
				obj = &corev1.Namespace{}
			)

			defer test.WithVars(
				&TimeNow, mockNow.Do,
			)()

			expectedObj := obj.DeepCopy()
			expectedObj.Annotations = map[string]string{ConfirmationDeletion: "true", v1beta1constants.GardenerTimestamp: now.UTC().String()}

			mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

			c.EXPECT().Get(ctx, gomock.AssignableToTypeOf(client.ObjectKey{}), obj)
			c.EXPECT().Update(ctx, expectedObj)

			Expect(ConfirmDeletion(ctx, c, obj)).To(Succeed())
		})

		It("should add the deletion confirmation annotation for an object with annotations", func() {
			var (
				ctx = context.TODO()
				obj = &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							"foo": "bar",
						},
					},
				}
			)

			defer test.WithVars(
				&TimeNow, mockNow.Do,
			)()

			expectedObj := obj.DeepCopy()
			expectedObj.Annotations[ConfirmationDeletion] = "true"
			expectedObj.Annotations[v1beta1constants.GardenerTimestamp] = now.UTC().String()

			mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()
			c.EXPECT().Get(ctx, gomock.AssignableToTypeOf(client.ObjectKey{}), obj)
			c.EXPECT().Update(ctx, expectedObj)

			Expect(ConfirmDeletion(ctx, c, obj)).To(Succeed())
		})

		It("should ignore non-existing objects", func() {
			var (
				ctx         = context.TODO()
				obj         = &corev1.Namespace{}
				expectedObj = obj.DeepCopy()
			)

			c.EXPECT().Get(ctx, gomock.AssignableToTypeOf(client.ObjectKey{}), obj).Return(apierrors.NewNotFound(corev1.Resource("namespaces"), ""))

			Expect(ConfirmDeletion(ctx, c, obj)).To(Succeed())
			Expect(obj).To(Equal(expectedObj))
		})

		It("should retry on conflict and add the deletion confirmation annotation", func() {
			var (
				ctx     = context.TODO()
				baseObj = &corev1.Namespace{}
				obj     = baseObj.DeepCopy()
			)

			defer test.WithVars(
				&TimeNow, mockNow.Do,
			)()

			expectedObj := obj.DeepCopy()
			expectedObj.Annotations = map[string]string{ConfirmationDeletion: "true", v1beta1constants.GardenerTimestamp: now.UTC().String()}

			mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()
			c.EXPECT().Get(ctx, gomock.AssignableToTypeOf(client.ObjectKey{}), obj)
			c.EXPECT().Update(ctx, expectedObj).Return(apierrors.NewConflict(corev1.Resource("namespaces"), "", errors.New("conflict")))
			c.EXPECT().Get(ctx, gomock.AssignableToTypeOf(client.ObjectKey{}), expectedObj).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
				baseObj.DeepCopyInto(obj.(*corev1.Namespace))
				return nil
			})
			c.EXPECT().Update(ctx, expectedObj)

			Expect(ConfirmDeletion(ctx, c, obj)).To(Succeed())
		})
	})

	Describe("#ExtensionID", func() {
		It("should return the expected identifier", func() {
			Expect(ExtensionID("foo", "bar")).To(Equal("foo/bar"))
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
})

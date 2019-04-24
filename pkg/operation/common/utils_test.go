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
	"strings"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	. "github.com/gardener/gardener/pkg/operation/common"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/golang/mock/gomock"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
)

var _ = Describe("common", func() {
	var (
		ctrl *gomock.Controller
		c    *mockclient.MockClient
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("utils", func() {
		Describe("#DistributePercentOverZones", func() {
			It("should return unmodified percentage if total is evenly dividable", func() {
				var (
					total      = 6
					noOfZones  = 3
					percentage = "40%"
				)

				percentages := make([]string, 0, noOfZones)
				for i := 0; i < noOfZones; i++ {
					percentages = append(percentages, DistributePercentOverZones(i, percentage, noOfZones, total))
				}
				Expect(percentages).To(Equal([]string{percentage, percentage, percentage}))
			})

			It("should return correct percentage if total is not evenly dividable", func() {
				var (
					total      = 7
					noOfZones  = 3
					percentage = "40%"
				)

				percentages := make([]string, 0, noOfZones)
				for i := 0; i < noOfZones; i++ {
					percentages = append(percentages, DistributePercentOverZones(i, percentage, noOfZones, total))
				}
				Expect(percentages).To(Equal([]string{"52%", "35%", "35%"}))
			})
		})

		Describe("#ComputeClusterIP", func() {
			It("should return a cluster IP as string", func() {
				var (
					ip   = "100.64.0.0"
					cidr = gardencorev1alpha1.CIDR(ip + "/13")
				)

				result := ComputeClusterIP(cidr, 10)

				Expect(result).To(Equal("100.64.0.10"))
			})
		})

		Describe("#DiskSize", func() {
			It("should return a string", func() {
				var (
					size    = "10"
					sizeInt = 10
				)

				result := DiskSize(size + "Gi")

				Expect(result).To(Equal(sizeInt))
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

	Describe("#MergeOwnerReferences", func() {
		It("should merge the new references into the list of existing references", func() {
			var (
				references = []metav1.OwnerReference{
					{
						UID: types.UID("1234"),
					},
				}
				newReferences = []metav1.OwnerReference{
					{
						UID: types.UID("1234"),
					},
					{
						UID: types.UID("1235"),
					},
				}
			)

			result := MergeOwnerReferences(references, newReferences...)

			Expect(result).To(ConsistOf(newReferences))
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

	Describe("#GetLoadBalancerIngress", func() {
		var (
			namespace = "foo"
			name      = "bar"
			key       = kutil.Key(namespace, name)
		)

		It("should return an unexpected client error", func() {
			ctx := context.TODO()
			expectedErr := fmt.Errorf("unexpected")

			c.EXPECT().Get(ctx, key, gomock.AssignableToTypeOf(&corev1.Service{})).Return(expectedErr)

			_, err := GetLoadBalancerIngress(ctx, c, namespace, name)

			Expect(err).To(HaveOccurred())
			Expect(err).To(BeIdenticalTo(expectedErr))
		})

		It("should return an error because no ingresses found", func() {
			ctx := context.TODO()

			c.EXPECT().Get(ctx, key, gomock.AssignableToTypeOf(&corev1.Service{}))

			_, err := GetLoadBalancerIngress(ctx, c, namespace, name)

			Expect(err).To(MatchError("`.status.loadBalancer.ingress[]` has no elements yet, i.e. external load balancer has not been created (is your quota limit exceeded/reached?)"))
		})

		It("should return an ip address", func() {
			var (
				ctx        = context.TODO()
				expectedIP = "1.2.3.4"
			)

			c.EXPECT().Get(ctx, key, gomock.AssignableToTypeOf(&corev1.Service{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, service *corev1.Service) error {
				service.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{{IP: expectedIP}}
				return nil
			})

			ingress, err := GetLoadBalancerIngress(ctx, c, namespace, name)

			Expect(ingress).To(Equal(expectedIP))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return an hostname address", func() {
			var (
				ctx              = context.TODO()
				expectedHostname = "cluster.local"
			)

			c.EXPECT().Get(ctx, key, gomock.AssignableToTypeOf(&corev1.Service{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, service *corev1.Service) error {
				service.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{{Hostname: expectedHostname}}
				return nil
			})

			ingress, err := GetLoadBalancerIngress(ctx, c, namespace, name)

			Expect(ingress).To(Equal(expectedHostname))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return an error if neither ip nor hostname were set", func() {
			ctx := context.TODO()

			c.EXPECT().Get(ctx, key, gomock.AssignableToTypeOf(&corev1.Service{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, service *corev1.Service) error {
				service.Status.LoadBalancer.Ingress = []corev1.LoadBalancerIngress{{}}
				return nil
			})

			_, err := GetLoadBalancerIngress(ctx, c, namespace, name)

			Expect(err).To(MatchError("`.status.loadBalancer.ingress[]` has an element which does neither contain `.ip` nor `.hostname`"))
		})
	})

	DescribeTable("#GetDomainInfoFromAnnotations",
		func(annotations map[string]string, expectedProvider, expectedDomain, expectedErr gomegatypes.GomegaMatcher) {
			provider, domain, err := GetDomainInfoFromAnnotations(annotations)
			Expect(provider).To(expectedProvider)
			Expect(domain).To(expectedDomain)
			Expect(err).To(expectedErr)
		},

		Entry("no annotations", nil, BeEmpty(), BeEmpty(), HaveOccurred()),

		Entry("no domain", map[string]string{
			DNSProvider: "bar",
		}, BeEmpty(), BeEmpty(), HaveOccurred()),

		Entry("no provider", map[string]string{
			DNSProvider: "bar",
		}, BeEmpty(), BeEmpty(), HaveOccurred()),

		Entry("all present", map[string]string{
			DNSProvider: "bar",
			DNSDomain:   "foo",
		}, Equal("bar"), Equal("foo"), Not(HaveOccurred())),
	)

	Describe("#InjectCSIFeatureGates", func() {
		csiFG := map[string]bool{
			"VolumeSnapshotDataSource": true,
			"KubeletPluginsWatcher":    true,
			"CSINodeInfo":              true,
			"CSIDriverRegistry":        true,
		}

		It("should return nil because version is < 1.13", func() {
			fg, err := InjectCSIFeatureGates("1.12.1", nil)

			Expect(err).To(Not(HaveOccurred()))
			Expect(fg).To(BeNil())
		})

		It("should return the CSI FG because version is >= 1.13", func() {
			fg, err := InjectCSIFeatureGates("1.13.1", nil)

			Expect(err).To(Not(HaveOccurred()))
			Expect(fg).To(Equal(csiFG))
		})

		It("should return the CSI FG because version is >= 1.13 (with other features)", func() {
			featureGates := map[string]bool{"foo": false, "bar": true}

			result := make(map[string]bool, len(featureGates)+len(csiFG))
			for k, v := range featureGates {
				result[k] = v
			}
			for k, v := range csiFG {
				result[k] = v
			}

			fg, err := InjectCSIFeatureGates("1.13.1", featureGates)

			Expect(err).To(Not(HaveOccurred()))
			Expect(fg).To(Equal(result))
		})
	})
})

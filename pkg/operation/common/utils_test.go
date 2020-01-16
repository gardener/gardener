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
	"fmt"
	"strings"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	. "github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/version"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("common", func() {
	Describe("utils", func() {
		Describe("#ComputeClusterIP", func() {
			It("should return a cluster IP as string", func() {
				var (
					ip   = "100.64.0.0"
					cidr = ip + "/13"
				)

				result := ComputeClusterIP(cidr, 10)

				Expect(result).To(Equal("100.64.0.10"))
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

	DescribeTable("#RespectSyncPeriodOverwrite",
		func(respectSyncPeriodOverwrite bool, shoot *gardencorev1beta1.Shoot, match gomegatypes.GomegaMatcher) {
			Expect(RespectShootSyncPeriodOverwrite(respectSyncPeriodOverwrite, shoot)).To(match)
		},

		Entry("respect overwrite",
			true,
			&gardencorev1beta1.Shoot{},
			BeTrue()),
		Entry("don't respect overwrite",
			false,
			&gardencorev1beta1.Shoot{},
			BeFalse()),
		Entry("don't respect overwrite but garden namespace",
			false,
			&gardencorev1beta1.Shoot{ObjectMeta: kutil.ObjectMeta(v1beta1constants.GardenNamespace, "foo")},
			BeTrue()),
	)

	DescribeTable("#ShouldIgnoreShoot",
		func(respectSyncPeriodOverwrite bool, shoot *gardencorev1beta1.Shoot, match gomegatypes.GomegaMatcher) {
			Expect(ShouldIgnoreShoot(respectSyncPeriodOverwrite, shoot)).To(match)
		},

		Entry("respect overwrite with annotation",
			true,
			&gardencorev1beta1.Shoot{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{ShootIgnore: "true"}}},
			BeTrue()),
		Entry("respect overwrite with wrong annotation",
			true,
			&gardencorev1beta1.Shoot{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{ShootIgnore: "foo"}}},
			BeFalse()),
		Entry("respect overwrite with no annotation",
			true,
			&gardencorev1beta1.Shoot{},
			BeFalse()),
	)

	DescribeTable("#IsShootFailed",
		func(shoot *gardencorev1beta1.Shoot, match gomegatypes.GomegaMatcher) {
			Expect(IsShootFailed(shoot)).To(match)
		},

		Entry("no last operation",
			&gardencorev1beta1.Shoot{},
			BeFalse()),
		Entry("with last operation but not in failed state",
			&gardencorev1beta1.Shoot{
				Status: gardencorev1beta1.ShootStatus{
					LastOperation: &gardencorev1beta1.LastOperation{
						State: gardencorev1beta1.LastOperationStateSucceeded,
					},
				},
			},
			BeFalse()),
		Entry("with last operation in failed state but not at latest generation",
			&gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{Generation: 1},
				Status: gardencorev1beta1.ShootStatus{
					LastOperation: &gardencorev1beta1.LastOperation{
						State: gardencorev1beta1.LastOperationStateFailed,
					},
				},
			},
			BeFalse()),
		Entry("with last operation in failed state and matching generation but not latest gardener version",
			&gardencorev1beta1.Shoot{
				Status: gardencorev1beta1.ShootStatus{
					LastOperation: &gardencorev1beta1.LastOperation{
						State: gardencorev1beta1.LastOperationStateFailed,
					},
					Gardener: gardencorev1beta1.Gardener{
						Version: version.Get().GitVersion + "foo",
					},
				},
			},
			BeFalse()),
		Entry("with last operation in failed state and matching generation and latest gardener version",
			&gardencorev1beta1.Shoot{
				Status: gardencorev1beta1.ShootStatus{
					LastOperation: &gardencorev1beta1.LastOperation{
						State: gardencorev1beta1.LastOperationStateFailed,
					},
					Gardener: gardencorev1beta1.Gardener{
						Version: version.Get().GitVersion,
					},
				},
			},
			BeTrue()),
	)

	DescribeTable("#IsObservedAtLatestGenerationAndSucceeded",
		func(shoot *gardencorev1beta1.Shoot, match gomegatypes.GomegaMatcher) {
			Expect(IsObservedAtLatestGenerationAndSucceeded(shoot)).To(match)
		},

		Entry("not at observed generation",
			&gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{Generation: 1},
			},
			BeFalse()),
		Entry("last operation state not succeeded",
			&gardencorev1beta1.Shoot{
				Status: gardencorev1beta1.ShootStatus{
					LastOperation: &gardencorev1beta1.LastOperation{
						State: gardencorev1beta1.LastOperationStateError,
					},
				},
			},
			BeFalse()),
		Entry("observed at latest generation and no last operation state",
			&gardencorev1beta1.Shoot{},
			BeFalse()),
		Entry("observed at latest generation and last operation state succeeded",
			&gardencorev1beta1.Shoot{
				Status: gardencorev1beta1.ShootStatus{
					LastOperation: &gardencorev1beta1.LastOperation{
						State: gardencorev1beta1.LastOperationStateSucceeded,
					},
				},
			},
			BeTrue()),
	)

	DescribeTable("#SyncPeriodOfShoot",
		func(respectSyncPeriodOverwrite bool, defaultMinSyncPeriod time.Duration, shoot *gardencorev1beta1.Shoot, expected time.Duration) {
			Expect(SyncPeriodOfShoot(respectSyncPeriodOverwrite, defaultMinSyncPeriod, shoot)).To(Equal(expected))
		},

		Entry("don't respect overwrite",
			false,
			1*time.Second,
			&gardencorev1beta1.Shoot{},
			1*time.Second),
		Entry("respect overwrite but no overwrite",
			true,
			1*time.Second,
			&gardencorev1beta1.Shoot{},
			1*time.Second),
		Entry("respect overwrite but overwrite invalid",
			true,
			1*time.Second,
			&gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{ShootSyncPeriod: "foo"},
				},
			},
			1*time.Second),
		Entry("respect overwrite but overwrite too short",
			true,
			2*time.Second,
			&gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{ShootSyncPeriod: (1 * time.Second).String()},
				},
			},
			2*time.Second),
		Entry("respect overwrite with longer overwrite",
			true,
			2*time.Second,
			&gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{ShootSyncPeriod: (3 * time.Second).String()},
				},
			},
			3*time.Second),
	)

	Describe("#EffectiveMaintenanceTimeWindow", func() {
		It("should shorten the end of the time window by 15 minutes", func() {
			var (
				begin = utils.NewMaintenanceTime(0, 0, 0)
				end   = utils.NewMaintenanceTime(1, 0, 0)
			)

			Expect(EffectiveMaintenanceTimeWindow(utils.NewMaintenanceTimeWindow(begin, end))).
				To(Equal(utils.NewMaintenanceTimeWindow(begin, utils.NewMaintenanceTime(0, 45, 0))))
		})
	})

	DescribeTable("#EffectiveShootMaintenanceTimeWindow",
		func(shoot *gardencorev1beta1.Shoot, window *utils.MaintenanceTimeWindow) {
			Expect(EffectiveShootMaintenanceTimeWindow(shoot)).To(Equal(window))
		},

		Entry("no maintenance section",
			&gardencorev1beta1.Shoot{},
			utils.AlwaysTimeWindow),
		Entry("no time window",
			&gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Maintenance: &gardencorev1beta1.Maintenance{},
				},
			},
			utils.AlwaysTimeWindow),
		Entry("invalid time window",
			&gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Maintenance: &gardencorev1beta1.Maintenance{
						TimeWindow: &gardencorev1beta1.MaintenanceTimeWindow{},
					},
				},
			},
			utils.AlwaysTimeWindow),
		Entry("valid time window",
			&gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Maintenance: &gardencorev1beta1.Maintenance{
						TimeWindow: &gardencorev1beta1.MaintenanceTimeWindow{
							Begin: "010000+0000",
							End:   "020000+0000",
						},
					},
				},
			},
			utils.NewMaintenanceTimeWindow(
				utils.NewMaintenanceTime(1, 0, 0),
				utils.NewMaintenanceTime(1, 45, 0))),
	)
})

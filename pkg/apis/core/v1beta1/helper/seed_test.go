// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper_test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/ptr"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
)

var _ = Describe("Helper", func() {
	DescribeTable("#TaintsHave",
		func(taints []gardencorev1beta1.SeedTaint, key string, expectation bool) {
			Expect(TaintsHave(taints, key)).To(Equal(expectation))
		},
		Entry("taint exists", []gardencorev1beta1.SeedTaint{{Key: "foo"}}, "foo", true),
		Entry("taint does not exist", []gardencorev1beta1.SeedTaint{{Key: "foo"}}, "bar", false),
	)

	DescribeTable("#TaintsAreTolerated",
		func(taints []gardencorev1beta1.SeedTaint, tolerations []gardencorev1beta1.Toleration, expectation bool) {
			Expect(TaintsAreTolerated(taints, tolerations)).To(Equal(expectation))
		},

		Entry("no taints",
			nil,
			[]gardencorev1beta1.Toleration{{Key: "foo"}},
			true,
		),
		Entry("no tolerations",
			[]gardencorev1beta1.SeedTaint{{Key: "foo"}},
			nil,
			false,
		),
		Entry("taints with keys only, tolerations with keys only (tolerated)",
			[]gardencorev1beta1.SeedTaint{{Key: "foo"}},
			[]gardencorev1beta1.Toleration{{Key: "foo"}},
			true,
		),
		Entry("taints with keys only, tolerations with keys only (non-tolerated)",
			[]gardencorev1beta1.SeedTaint{{Key: "foo"}},
			[]gardencorev1beta1.Toleration{{Key: "bar"}},
			false,
		),
		Entry("taints with keys+values only, tolerations with keys+values only (tolerated)",
			[]gardencorev1beta1.SeedTaint{{Key: "foo", Value: ptr.To("bar")}},
			[]gardencorev1beta1.Toleration{{Key: "foo", Value: ptr.To("bar")}},
			true,
		),
		Entry("taints with keys+values only, tolerations with keys+values only (non-tolerated)",
			[]gardencorev1beta1.SeedTaint{{Key: "foo", Value: ptr.To("bar")}},
			[]gardencorev1beta1.Toleration{{Key: "bar", Value: ptr.To("foo")}},
			false,
		),
		Entry("taints with mixed key(+values), tolerations with mixed key(+values) (tolerated)",
			[]gardencorev1beta1.SeedTaint{
				{Key: "foo"},
				{Key: "bar", Value: ptr.To("baz")},
			},
			[]gardencorev1beta1.Toleration{
				{Key: "foo"},
				{Key: "bar", Value: ptr.To("baz")},
			},
			true,
		),
		Entry("taints with mixed key(+values), tolerations with mixed key(+values) (non-tolerated)",
			[]gardencorev1beta1.SeedTaint{
				{Key: "foo"},
				{Key: "bar", Value: ptr.To("baz")},
			},
			[]gardencorev1beta1.Toleration{
				{Key: "bar"},
				{Key: "foo", Value: ptr.To("baz")},
			},
			false,
		),
		Entry("taints with mixed key(+values), tolerations with key+values only (tolerated)",
			[]gardencorev1beta1.SeedTaint{
				{Key: "foo"},
				{Key: "bar", Value: ptr.To("baz")},
			},
			[]gardencorev1beta1.Toleration{
				{Key: "foo", Value: ptr.To("bar")},
				{Key: "bar", Value: ptr.To("baz")},
			},
			true,
		),
		Entry("taints with mixed key(+values), tolerations with key+values only (not tolerated)",
			[]gardencorev1beta1.SeedTaint{
				{Key: "foo"},
				{Key: "bar", Value: ptr.To("baz")},
			},
			[]gardencorev1beta1.Toleration{
				{Key: "foo", Value: ptr.To("bar")},
				{Key: "bar", Value: ptr.To("foo")},
			},
			false,
		),
		Entry("taints > tolerations",
			[]gardencorev1beta1.SeedTaint{
				{Key: "foo"},
				{Key: "bar", Value: ptr.To("baz")},
			},
			[]gardencorev1beta1.Toleration{
				{Key: "bar", Value: ptr.To("baz")},
			},
			false,
		),
		Entry("tolerations > taints",
			[]gardencorev1beta1.SeedTaint{
				{Key: "foo"},
				{Key: "bar", Value: ptr.To("baz")},
			},
			[]gardencorev1beta1.Toleration{
				{Key: "foo", Value: ptr.To("bar")},
				{Key: "bar", Value: ptr.To("baz")},
				{Key: "baz", Value: ptr.To("foo")},
			},
			true,
		),
	)

	DescribeTable("#SeedSettingExcessCapacityReservationEnabled",
		func(settings *gardencorev1beta1.SeedSettings, expected bool) {
			Expect(SeedSettingExcessCapacityReservationEnabled(settings)).To(Equal(expected))
		},

		Entry("setting is nil", nil, true),
		Entry("excess capacity reservation is nil", &gardencorev1beta1.SeedSettings{}, true),
		Entry("excess capacity reservation 'enabled' is nil", &gardencorev1beta1.SeedSettings{ExcessCapacityReservation: &gardencorev1beta1.SeedSettingExcessCapacityReservation{Enabled: nil}}, true),
		Entry("excess capacity reservation 'enabled' is false", &gardencorev1beta1.SeedSettings{ExcessCapacityReservation: &gardencorev1beta1.SeedSettingExcessCapacityReservation{Enabled: ptr.To(false)}}, false),
		Entry("excess capacity reservation 'enabled' is true", &gardencorev1beta1.SeedSettings{ExcessCapacityReservation: &gardencorev1beta1.SeedSettingExcessCapacityReservation{Enabled: ptr.To(true)}}, true),
	)

	DescribeTable("#SeedSettingVerticalPodAutoscalerEnabled",
		func(settings *gardencorev1beta1.SeedSettings, expected bool) {
			Expect(SeedSettingVerticalPodAutoscalerEnabled(settings)).To(Equal(expected))
		},

		Entry("no settings", nil, true),
		Entry("no vertical pod autocaler setting", &gardencorev1beta1.SeedSettings{}, true),
		Entry("vertical pod autoscaler enabled", &gardencorev1beta1.SeedSettings{VerticalPodAutoscaler: &gardencorev1beta1.SeedSettingVerticalPodAutoscaler{Enabled: true}}, true),
		Entry("vertical pod autoscaler disabled", &gardencorev1beta1.SeedSettings{VerticalPodAutoscaler: &gardencorev1beta1.SeedSettingVerticalPodAutoscaler{Enabled: false}}, false),
	)

	DescribeTable("#SeedSettingVerticalPodAutoscalerMaxAllowed",
		func(settings *gardencorev1beta1.SeedSettings, expected corev1.ResourceList) {
			Expect(SeedSettingVerticalPodAutoscalerMaxAllowed(settings)).To(Equal(expected))
		},

		Entry("no settings", nil, nil),
		Entry("no vertical pod autocaler setting", &gardencorev1beta1.SeedSettings{}, nil),
		Entry("vertical pod autocaler max allowed setting exists",
			&gardencorev1beta1.SeedSettings{VerticalPodAutoscaler: &gardencorev1beta1.SeedSettingVerticalPodAutoscaler{MaxAllowed: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("2")}}},
			corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("2")},
		),
	)

	DescribeTable("#SeedSettingDependencyWatchdogWeederEnabled",
		func(settings *gardencorev1beta1.SeedSettings, expected bool) {
			Expect(SeedSettingDependencyWatchdogWeederEnabled(settings)).To(Equal(expected))
		},

		Entry("no settings", nil, true),
		Entry("no dwd setting", &gardencorev1beta1.SeedSettings{}, true),
		Entry("no dwd weeder setting", &gardencorev1beta1.SeedSettings{DependencyWatchdog: &gardencorev1beta1.SeedSettingDependencyWatchdog{}}, true),
		Entry("dwd weeder enabled", &gardencorev1beta1.SeedSettings{DependencyWatchdog: &gardencorev1beta1.SeedSettingDependencyWatchdog{Weeder: &gardencorev1beta1.SeedSettingDependencyWatchdogWeeder{Enabled: true}}}, true),
		Entry("dwd weeder disabled", &gardencorev1beta1.SeedSettings{DependencyWatchdog: &gardencorev1beta1.SeedSettingDependencyWatchdog{Weeder: &gardencorev1beta1.SeedSettingDependencyWatchdogWeeder{Enabled: false}}}, false),
	)

	DescribeTable("#SeedSettingDependencyWatchdogProberEnabled",
		func(settings *gardencorev1beta1.SeedSettings, expected bool) {
			Expect(SeedSettingDependencyWatchdogProberEnabled(settings)).To(Equal(expected))
		},

		Entry("no settings", nil, true),
		Entry("no dwd setting", &gardencorev1beta1.SeedSettings{}, true),
		Entry("no dwd prober setting", &gardencorev1beta1.SeedSettings{DependencyWatchdog: &gardencorev1beta1.SeedSettingDependencyWatchdog{}}, true),
		Entry("dwd prober enabled", &gardencorev1beta1.SeedSettings{DependencyWatchdog: &gardencorev1beta1.SeedSettingDependencyWatchdog{Prober: &gardencorev1beta1.SeedSettingDependencyWatchdogProber{Enabled: true}}}, true),
		Entry("dwd prober disabled", &gardencorev1beta1.SeedSettings{DependencyWatchdog: &gardencorev1beta1.SeedSettingDependencyWatchdog{Prober: &gardencorev1beta1.SeedSettingDependencyWatchdogProber{Enabled: false}}}, false),
	)

	DescribeTable("#SeedSettingTopologyAwareRoutingEnabled",
		func(settings *gardencorev1beta1.SeedSettings, expected bool) {
			Expect(SeedSettingTopologyAwareRoutingEnabled(settings)).To(Equal(expected))
		},

		Entry("no settings", nil, false),
		Entry("no topology-aware routing setting", &gardencorev1beta1.SeedSettings{}, false),
		Entry("topology-aware routing enabled", &gardencorev1beta1.SeedSettings{TopologyAwareRouting: &gardencorev1beta1.SeedSettingTopologyAwareRouting{Enabled: true}}, true),
		Entry("topology-aware routing disabled", &gardencorev1beta1.SeedSettings{TopologyAwareRouting: &gardencorev1beta1.SeedSettingTopologyAwareRouting{Enabled: false}}, false),
	)

	DescribeTable("#SeedBackupCredentialsRefEqual",
		func(oldBackup, newBackup *gardencorev1beta1.Backup, matcher gomegatypes.GomegaMatcher) {
			Expect(SeedBackupCredentialsRefEqual(oldBackup, newBackup)).To(matcher)
		},

		Entry("both nil", nil, nil, BeTrue()),
		Entry("old nil, new empty", nil, &gardencorev1beta1.Backup{}, BeTrue()),
		Entry("old empty, new nil", &gardencorev1beta1.Backup{}, nil, BeTrue()),
		Entry("both empty", &gardencorev1beta1.Backup{}, &gardencorev1beta1.Backup{}, BeTrue()),
		Entry("difference",
			&gardencorev1beta1.Backup{CredentialsRef: &corev1.ObjectReference{APIVersion: "v1", Kind: "Secret", Name: "foo", Namespace: "bar"}},
			&gardencorev1beta1.Backup{CredentialsRef: &corev1.ObjectReference{APIVersion: "v1", Kind: "Secret", Name: "bar", Namespace: "foo"}},
			BeFalse(),
		),
		Entry("difference",
			&gardencorev1beta1.Backup{CredentialsRef: &corev1.ObjectReference{APIVersion: "security.gardener.cloud/v1alpha1", Kind: "WorkloadIdentity", Name: "foo", Namespace: "bar"}},
			&gardencorev1beta1.Backup{CredentialsRef: &corev1.ObjectReference{APIVersion: "security.gardener.cloud/v1alpha1", Kind: "WorkloadIdentity", Name: "bar", Namespace: "foo"}},
			BeFalse(),
		),
		Entry("equality",
			&gardencorev1beta1.Backup{CredentialsRef: &corev1.ObjectReference{APIVersion: "v1", Kind: "Secret", Name: "foo", Namespace: "bar"}},
			&gardencorev1beta1.Backup{CredentialsRef: &corev1.ObjectReference{APIVersion: "v1", Kind: "Secret", Name: "foo", Namespace: "bar"}},
			BeTrue(),
		),
	)

	Describe("#CalculateSeedUsage", func() {
		type shootCase struct {
			specSeedName, statusSeedName string
		}

		test := func(shoots []shootCase, expectedUsage map[string]int) {
			var shootList []*gardencorev1beta1.Shoot

			for i, shoot := range shoots {
				s := &gardencorev1beta1.Shoot{}
				s.Name = fmt.Sprintf("shoot-%d", i)
				if shoot.specSeedName != "" {
					s.Spec.SeedName = ptr.To(shoot.specSeedName)
				}
				if shoot.statusSeedName != "" {
					s.Status.SeedName = ptr.To(shoot.statusSeedName)
				}
				shootList = append(shootList, s)
			}

			ExpectWithOffset(1, CalculateSeedUsage(shootList)).To(Equal(expectedUsage))
		}

		It("no shoots", func() {
			test([]shootCase{}, map[string]int{})
		})
		It("shoot with both fields unset", func() {
			test([]shootCase{{}}, map[string]int{})
		})
		It("shoot with only spec set", func() {
			test([]shootCase{{specSeedName: "seed"}}, map[string]int{"seed": 1})
		})
		It("shoot with only status set", func() {
			test([]shootCase{{statusSeedName: "seed"}}, map[string]int{"seed": 1})
		})
		It("shoot with both fields set to same seed", func() {
			test([]shootCase{{specSeedName: "seed", statusSeedName: "seed"}}, map[string]int{"seed": 1})
		})
		It("shoot with fields set to different seeds", func() {
			test([]shootCase{{specSeedName: "seed", statusSeedName: "seed2"}}, map[string]int{"seed": 1, "seed2": 1})
		})
		It("multiple shoots", func() {
			test([]shootCase{
				{},
				{specSeedName: "seed", statusSeedName: "seed2"},
				{specSeedName: "seed2", statusSeedName: "seed2"},
				{specSeedName: "seed3", statusSeedName: "seed2"},
				{specSeedName: "seed3", statusSeedName: "seed4"},
			}, map[string]int{"seed": 1, "seed2": 3, "seed3": 2, "seed4": 1})
		})
	})

	DescribeTable("#InternalDNSProviderCredentialsRefEqual",
		func(oldDNSProvider, newDNSProvider *gardencorev1beta1.SeedDNSProviderConfig, equal bool) {
			Expect(InternalDNSProviderCredentialsRefEqual(oldDNSProvider, newDNSProvider)).To(Equal(equal))
		},

		Entry("both nil", nil, nil, true),
		Entry("old nil, new empty", nil, &gardencorev1beta1.SeedDNSProviderConfig{}, false),
		Entry("old empty, new nil", &gardencorev1beta1.SeedDNSProviderConfig{}, nil, false),
		Entry("both empty", &gardencorev1beta1.SeedDNSProviderConfig{}, &gardencorev1beta1.SeedDNSProviderConfig{}, true),
		Entry("different credentials refs",
			&gardencorev1beta1.SeedDNSProviderConfig{CredentialsRef: corev1.ObjectReference{APIVersion: "v1", Kind: "Secret", Name: "foo", Namespace: "bar"}},
			&gardencorev1beta1.SeedDNSProviderConfig{CredentialsRef: corev1.ObjectReference{APIVersion: "v1", Kind: "Secret", Name: "bar", Namespace: "foo"}},
			false,
		),
		Entry("different API group in credentials ref",
			&gardencorev1beta1.SeedDNSProviderConfig{CredentialsRef: corev1.ObjectReference{APIVersion: "v1", Kind: "Secret", Name: "foo", Namespace: "bar"}},
			&gardencorev1beta1.SeedDNSProviderConfig{CredentialsRef: corev1.ObjectReference{APIVersion: "security.gardener.cloud/v1alpha1", Kind: "WorkloadIdentity", Name: "foo", Namespace: "bar"}},
			false,
		),
		Entry("equal credentials refs",
			&gardencorev1beta1.SeedDNSProviderConfig{CredentialsRef: corev1.ObjectReference{APIVersion: "v1", Kind: "Secret", Name: "foo", Namespace: "bar"}},
			&gardencorev1beta1.SeedDNSProviderConfig{CredentialsRef: corev1.ObjectReference{APIVersion: "v1", Kind: "Secret", Name: "foo", Namespace: "bar"}},
			true,
		),
	)
})

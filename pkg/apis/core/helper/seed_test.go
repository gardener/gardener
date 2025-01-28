// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper_test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/apis/core"
	. "github.com/gardener/gardener/pkg/apis/core/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

var _ = Describe("Helper", func() {
	DescribeTable("#TaintsHave",
		func(taints []core.SeedTaint, key string, expectation bool) {
			Expect(TaintsHave(taints, key)).To(Equal(expectation))
		},

		Entry("taint exists", []core.SeedTaint{{Key: "foo"}}, "foo", true),
		Entry("taint does not exist", []core.SeedTaint{{Key: "foo"}}, "bar", false),
	)

	DescribeTable("#TaintsAreTolerated",
		func(taints []core.SeedTaint, tolerations []core.Toleration, expectation bool) {
			Expect(TaintsAreTolerated(taints, tolerations)).To(Equal(expectation))
		},

		Entry("no taints",
			nil,
			[]core.Toleration{{Key: "foo"}},
			true,
		),
		Entry("no tolerations",
			[]core.SeedTaint{{Key: "foo"}},
			nil,
			false,
		),
		Entry("taints with keys only, tolerations with keys only (tolerated)",
			[]core.SeedTaint{{Key: "foo"}},
			[]core.Toleration{{Key: "foo"}},
			true,
		),
		Entry("taints with keys only, tolerations with keys only (non-tolerated)",
			[]core.SeedTaint{{Key: "foo"}},
			[]core.Toleration{{Key: "bar"}},
			false,
		),
		Entry("taints with keys+values only, tolerations with keys+values only (tolerated)",
			[]core.SeedTaint{{Key: "foo", Value: ptr.To("bar")}},
			[]core.Toleration{{Key: "foo", Value: ptr.To("bar")}},
			true,
		),
		Entry("taints with keys+values only, tolerations with keys+values only (non-tolerated)",
			[]core.SeedTaint{{Key: "foo", Value: ptr.To("bar")}},
			[]core.Toleration{{Key: "bar", Value: ptr.To("foo")}},
			false,
		),
		Entry("taints with mixed key(+values), tolerations with mixed key(+values) (tolerated)",
			[]core.SeedTaint{
				{Key: "foo"},
				{Key: "bar", Value: ptr.To("baz")},
			},
			[]core.Toleration{
				{Key: "foo"},
				{Key: "bar", Value: ptr.To("baz")},
			},
			true,
		),
		Entry("taints with mixed key(+values), tolerations with mixed key(+values) (non-tolerated)",
			[]core.SeedTaint{
				{Key: "foo"},
				{Key: "bar", Value: ptr.To("baz")},
			},
			[]core.Toleration{
				{Key: "bar"},
				{Key: "foo", Value: ptr.To("baz")},
			},
			false,
		),
		Entry("taints with mixed key(+values), tolerations with key+values only (tolerated)",
			[]core.SeedTaint{
				{Key: "foo"},
				{Key: "bar", Value: ptr.To("baz")},
			},
			[]core.Toleration{
				{Key: "foo", Value: ptr.To("bar")},
				{Key: "bar", Value: ptr.To("baz")},
			},
			true,
		),
		Entry("taints with mixed key(+values), tolerations with key+values only (untolerated)",
			[]core.SeedTaint{
				{Key: "foo"},
				{Key: "bar", Value: ptr.To("baz")},
			},
			[]core.Toleration{
				{Key: "foo", Value: ptr.To("bar")},
				{Key: "bar", Value: ptr.To("foo")},
			},
			false,
		),
		Entry("taints > tolerations",
			[]core.SeedTaint{
				{Key: "foo"},
				{Key: "bar", Value: ptr.To("baz")},
			},
			[]core.Toleration{
				{Key: "bar", Value: ptr.To("baz")},
			},
			false,
		),
		Entry("tolerations > taints",
			[]core.SeedTaint{
				{Key: "foo"},
				{Key: "bar", Value: ptr.To("baz")},
			},
			[]core.Toleration{
				{Key: "foo", Value: ptr.To("bar")},
				{Key: "bar", Value: ptr.To("baz")},
				{Key: "baz", Value: ptr.To("foo")},
			},
			true,
		),
	)

	DescribeTable("#SeedSettingSchedulingVisible",
		func(settings *core.SeedSettings, expectation bool) {
			Expect(SeedSettingSchedulingVisible(settings)).To(Equal(expectation))
		},

		Entry("setting is nil", nil, true),
		Entry("scheduling is nil", &core.SeedSettings{}, true),
		Entry("scheduling 'visible' is false", &core.SeedSettings{Scheduling: &core.SeedSettingScheduling{Visible: false}}, false),
		Entry("scheduling 'visible' is true", &core.SeedSettings{Scheduling: &core.SeedSettingScheduling{Visible: true}}, true),
	)

	DescribeTable("#SeedSettingTopologyAwareRoutingEnabled",
		func(settings *core.SeedSettings, expected bool) {
			Expect(SeedSettingTopologyAwareRoutingEnabled(settings)).To(Equal(expected))
		},

		Entry("no settings", nil, false),
		Entry("no topology-aware routing setting", &core.SeedSettings{}, false),
		Entry("topology-aware routing enabled", &core.SeedSettings{TopologyAwareRouting: &core.SeedSettingTopologyAwareRouting{Enabled: true}}, true),
		Entry("topology-aware routing disabled", &core.SeedSettings{TopologyAwareRouting: &core.SeedSettingTopologyAwareRouting{Enabled: false}}, false),
	)

	Describe("#CalculateSeedUsage", func() {
		type shootCase struct {
			specSeedName, statusSeedName string
		}

		test := func(shoots []shootCase, expectedUsage map[string]int) {
			var shootList []*core.Shoot

			for i, shoot := range shoots {
				s := &core.Shoot{}
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

	Describe("#ConvertSeed", func() {
		It("should convert the external Seed version to an internal one", func() {
			result, err := ConvertSeed(&gardencorev1beta1.Seed{
				TypeMeta: metav1.TypeMeta{
					APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
					Kind:       "Seed",
				},
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(&core.Seed{}))
		})
	})

	Describe("#ConvertSeedExternal", func() {
		It("should convert the internal Seed version to an external one", func() {
			result, err := ConvertSeedExternal(&core.Seed{})

			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(&gardencorev1beta1.Seed{
				TypeMeta: metav1.TypeMeta{
					APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
					Kind:       "Seed",
				},
			}))
		})
	})

	Describe("#ConvertSeedTemplate", func() {
		It("should convert the external SeedTemplate version to an internal one", func() {
			Expect(ConvertSeedTemplate(&gardencorev1beta1.SeedTemplate{
				Spec: gardencorev1beta1.SeedSpec{
					Provider: gardencorev1beta1.SeedProvider{
						Type: "local",
					},
				},
			})).To(Equal(&core.SeedTemplate{
				Spec: core.SeedSpec{
					Provider: core.SeedProvider{
						Type: "local",
					},
				},
			}))
		})
	})

	Describe("#ConvertSeedTemplateExternal", func() {
		It("should convert the internal SeedTemplate version to an external one", func() {
			Expect(ConvertSeedTemplateExternal(&core.SeedTemplate{
				Spec: core.SeedSpec{
					Provider: core.SeedProvider{
						Type: "local",
					},
				},
			})).To(Equal(&gardencorev1beta1.SeedTemplate{
				Spec: gardencorev1beta1.SeedSpec{
					Provider: gardencorev1beta1.SeedProvider{
						Type: "local",
					},
				},
			}))
		})
	})
})

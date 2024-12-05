// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package health_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
)

var _ = Describe("Seed", func() {
	Describe("CheckSeed", func() {
		DescribeTable("seeds",
			func(seed *gardencorev1beta1.Seed, identity *gardencorev1beta1.Gardener, matcher types.GomegaMatcher) {
				Expect(health.CheckSeed(seed, identity)).To(matcher)
			},
			Entry("healthy", &gardencorev1beta1.Seed{
				Status: gardencorev1beta1.SeedStatus{
					Gardener: &gardencorev1beta1.Gardener{},
					Conditions: []gardencorev1beta1.Condition{
						{Type: gardencorev1beta1.SeedGardenletReady, Status: gardencorev1beta1.ConditionTrue},
						{Type: gardencorev1beta1.SeedSystemComponentsHealthy, Status: gardencorev1beta1.ConditionTrue},
					},
				},
			}, &gardencorev1beta1.Gardener{}, Succeed()),
			Entry("healthy with non-default identity", &gardencorev1beta1.Seed{
				Status: gardencorev1beta1.SeedStatus{
					Gardener: &gardencorev1beta1.Gardener{ID: "thegardener"},
					Conditions: []gardencorev1beta1.Condition{
						{Type: gardencorev1beta1.SeedGardenletReady, Status: gardencorev1beta1.ConditionTrue},
						{Type: gardencorev1beta1.SeedSystemComponentsHealthy, Status: gardencorev1beta1.ConditionTrue},
					},
				},
			}, &gardencorev1beta1.Gardener{ID: "thegardener"}, Succeed()),
			Entry("unhealthy available condition (gardenlet ready)", &gardencorev1beta1.Seed{
				Status: gardencorev1beta1.SeedStatus{
					Conditions: []gardencorev1beta1.Condition{
						{Type: gardencorev1beta1.SeedGardenletReady, Status: gardencorev1beta1.ConditionFalse},
						{Type: gardencorev1beta1.SeedSystemComponentsHealthy, Status: gardencorev1beta1.ConditionTrue},
					},
				},
			}, &gardencorev1beta1.Gardener{}, HaveOccurred()),
			Entry("unhealthy available condition (seed system components healthy)", &gardencorev1beta1.Seed{
				Status: gardencorev1beta1.SeedStatus{
					Conditions: []gardencorev1beta1.Condition{
						{Type: gardencorev1beta1.SeedGardenletReady, Status: gardencorev1beta1.ConditionTrue},
						{Type: gardencorev1beta1.SeedSystemComponentsHealthy, Status: gardencorev1beta1.ConditionFalse},
					},
				},
			}, &gardencorev1beta1.Gardener{}, HaveOccurred()),
			Entry("unhealthy due to missing all conditions", &gardencorev1beta1.Seed{}, &gardencorev1beta1.Gardener{}, HaveOccurred()),
			Entry("unhealthy due to non-matching identity", &gardencorev1beta1.Seed{
				Status: gardencorev1beta1.SeedStatus{
					Gardener: &gardencorev1beta1.Gardener{ID: "thegardener"},
					Conditions: []gardencorev1beta1.Condition{
						{Type: gardencorev1beta1.SeedGardenletReady, Status: gardencorev1beta1.ConditionTrue},
						{Type: gardencorev1beta1.SeedSystemComponentsHealthy, Status: gardencorev1beta1.ConditionTrue},
					},
				},
			}, &gardencorev1beta1.Gardener{}, HaveOccurred()),
			Entry("not observed at latest generation", &gardencorev1beta1.Seed{
				ObjectMeta: metav1.ObjectMeta{
					Generation: 1,
				},
				Status: gardencorev1beta1.SeedStatus{
					Conditions: []gardencorev1beta1.Condition{
						{Type: gardencorev1beta1.SeedGardenletReady, Status: gardencorev1beta1.ConditionTrue},
						{Type: gardencorev1beta1.SeedSystemComponentsHealthy, Status: gardencorev1beta1.ConditionTrue},
					},
				},
			}, &gardencorev1beta1.Gardener{}, HaveOccurred()),
		)
	})

	Describe("CheckSeedForMigration", func() {
		DescribeTable("seeds",
			func(seed *gardencorev1beta1.Seed, identity *gardencorev1beta1.Gardener, matcher types.GomegaMatcher) {
				Expect(health.CheckSeedForMigration(seed, identity)).To(matcher)
			},
			Entry("unhealthy with gardener info nil", &gardencorev1beta1.Seed{}, nil, MatchError(ContainSubstring("missing Gardener version information on source or destination seed"))),
			Entry("unhealthy with identity info nil", &gardencorev1beta1.Seed{
				Status: gardencorev1beta1.SeedStatus{
					Gardener: &gardencorev1beta1.Gardener{Version: "1.12.8"},
				},
			}, nil, MatchError(ContainSubstring("missing Gardener version information on source or destination seed"))),
			Entry("healthy with matching version", &gardencorev1beta1.Seed{
				Status: gardencorev1beta1.SeedStatus{
					Gardener: &gardencorev1beta1.Gardener{Version: "1.12.8"},
					Conditions: []gardencorev1beta1.Condition{
						{Type: gardencorev1beta1.SeedGardenletReady, Status: gardencorev1beta1.ConditionTrue},
						{Type: gardencorev1beta1.SeedSystemComponentsHealthy, Status: gardencorev1beta1.ConditionTrue},
					},
				},
			}, &gardencorev1beta1.Gardener{Version: "1.12.8"}, Succeed()),
			Entry("healthy with non-matching version", &gardencorev1beta1.Seed{
				Status: gardencorev1beta1.SeedStatus{
					Gardener: &gardencorev1beta1.Gardener{Version: "1.12.8"},
					Conditions: []gardencorev1beta1.Condition{
						{Type: gardencorev1beta1.SeedGardenletReady, Status: gardencorev1beta1.ConditionTrue},
						{Type: gardencorev1beta1.SeedSystemComponentsHealthy, Status: gardencorev1beta1.ConditionTrue},
					},
				},
			}, &gardencorev1beta1.Gardener{Version: "1.13.8"}, HaveOccurred()),
			Entry("unhealthy available condition (gardenlet ready) and matching version", &gardencorev1beta1.Seed{
				Status: gardencorev1beta1.SeedStatus{
					Gardener: &gardencorev1beta1.Gardener{Version: "1.12.8"},
					Conditions: []gardencorev1beta1.Condition{
						{Type: gardencorev1beta1.SeedGardenletReady, Status: gardencorev1beta1.ConditionFalse},
						{Type: gardencorev1beta1.SeedSystemComponentsHealthy, Status: gardencorev1beta1.ConditionTrue},
					},
				},
			}, &gardencorev1beta1.Gardener{Version: "1.12.8"}, HaveOccurred()),
			Entry("unhealthy available condition (seed system components healthy) and matching version", &gardencorev1beta1.Seed{
				Status: gardencorev1beta1.SeedStatus{
					Gardener: &gardencorev1beta1.Gardener{Version: "1.12.8"},
					Conditions: []gardencorev1beta1.Condition{
						{Type: gardencorev1beta1.SeedGardenletReady, Status: gardencorev1beta1.ConditionTrue},
						{Type: gardencorev1beta1.SeedSystemComponentsHealthy, Status: gardencorev1beta1.ConditionFalse},
					},
				},
			}, &gardencorev1beta1.Gardener{Version: "1.12.8"}, HaveOccurred()),
			Entry("unhealthy available condition (all conditions) and matching version", &gardencorev1beta1.Seed{
				Status: gardencorev1beta1.SeedStatus{
					Gardener: &gardencorev1beta1.Gardener{Version: "1.12.8"},
					Conditions: []gardencorev1beta1.Condition{
						{Type: gardencorev1beta1.SeedGardenletReady, Status: gardencorev1beta1.ConditionFalse},
						{Type: gardencorev1beta1.SeedSystemComponentsHealthy, Status: gardencorev1beta1.ConditionFalse},
					},
				},
			}, &gardencorev1beta1.Gardener{Version: "1.12.8"}, HaveOccurred()),
		)
	})
})

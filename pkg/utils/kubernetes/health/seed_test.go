// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
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
						{Type: gardencorev1beta1.GardenletReady, Status: gardencorev1beta1.ConditionTrue},
						{Type: gardencorev1beta1.SeedSystemComponentsHealthy, Status: gardencorev1beta1.ConditionTrue},
					},
				},
			}, &gardencorev1beta1.Gardener{}, Succeed()),
			Entry("healthy with non-default identity", &gardencorev1beta1.Seed{
				Status: gardencorev1beta1.SeedStatus{
					Gardener: &gardencorev1beta1.Gardener{ID: "thegardener"},
					Conditions: []gardencorev1beta1.Condition{
						{Type: gardencorev1beta1.GardenletReady, Status: gardencorev1beta1.ConditionTrue},
						{Type: gardencorev1beta1.SeedSystemComponentsHealthy, Status: gardencorev1beta1.ConditionTrue},
					},
				},
			}, &gardencorev1beta1.Gardener{ID: "thegardener"}, Succeed()),
			Entry("unhealthy available condition (gardenlet ready)", &gardencorev1beta1.Seed{
				Status: gardencorev1beta1.SeedStatus{
					Conditions: []gardencorev1beta1.Condition{
						{Type: gardencorev1beta1.GardenletReady, Status: gardencorev1beta1.ConditionFalse},
						{Type: gardencorev1beta1.SeedSystemComponentsHealthy, Status: gardencorev1beta1.ConditionTrue},
					},
				},
			}, &gardencorev1beta1.Gardener{}, HaveOccurred()),
			Entry("unhealthy available condition (seed system components healthy)", &gardencorev1beta1.Seed{
				Status: gardencorev1beta1.SeedStatus{
					Conditions: []gardencorev1beta1.Condition{
						{Type: gardencorev1beta1.GardenletReady, Status: gardencorev1beta1.ConditionTrue},
						{Type: gardencorev1beta1.SeedSystemComponentsHealthy, Status: gardencorev1beta1.ConditionFalse},
					},
				},
			}, &gardencorev1beta1.Gardener{}, HaveOccurred()),
			Entry("unhealthy due to missing all conditions", &gardencorev1beta1.Seed{}, &gardencorev1beta1.Gardener{}, HaveOccurred()),
			Entry("unhealthy due to non-matching identity", &gardencorev1beta1.Seed{
				Status: gardencorev1beta1.SeedStatus{
					Gardener: &gardencorev1beta1.Gardener{ID: "thegardener"},
					Conditions: []gardencorev1beta1.Condition{
						{Type: gardencorev1beta1.GardenletReady, Status: gardencorev1beta1.ConditionTrue},
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
						{Type: gardencorev1beta1.GardenletReady, Status: gardencorev1beta1.ConditionTrue},
						{Type: gardencorev1beta1.SeedSystemComponentsHealthy, Status: gardencorev1beta1.ConditionTrue},
					},
				},
			}, &gardencorev1beta1.Gardener{}, HaveOccurred()),
		)
	})

	Describe("CheckSeedIsUpToDate", func() {
		DescribeTable("seeds",
			func(seed *gardencorev1beta1.Seed, identity *gardencorev1beta1.Gardener, matcher types.GomegaMatcher) {
				Expect(health.CheckSeedIsUpToDate(seed, identity)).To(matcher)
			},
			Entry("up-to-date with empty identity", &gardencorev1beta1.Seed{
				Status: gardencorev1beta1.SeedStatus{
					Gardener: &gardencorev1beta1.Gardener{},
				},
			},
				&gardencorev1beta1.Gardener{},
				Succeed(),
			),
			Entry("up-to-date with matching identity",
				&gardencorev1beta1.Seed{
					Status: gardencorev1beta1.SeedStatus{
						Gardener: &gardencorev1beta1.Gardener{
							ID:      "test-gardener",
							Name:    "gardener",
							Version: "1.80.0",
						},
					},
				},
				&gardencorev1beta1.Gardener{
					ID:      "test-gardener",
					Name:    "gardener",
					Version: "1.80.0",
				},
				Succeed(),
			),
			Entry("up-to-date with matching generation",
				&gardencorev1beta1.Seed{
					ObjectMeta: metav1.ObjectMeta{
						Generation: 5,
					},
					Status: gardencorev1beta1.SeedStatus{
						Gardener:           &gardencorev1beta1.Gardener{},
						ObservedGeneration: 5,
					},
				},
				&gardencorev1beta1.Gardener{},
				Succeed(),
			),
			Entry("outdated - non-matching identity ID",
				&gardencorev1beta1.Seed{
					Status: gardencorev1beta1.SeedStatus{
						Gardener: &gardencorev1beta1.Gardener{
							ID:      "old-gardener",
							Version: "1.80.0",
						},
					},
				},
				&gardencorev1beta1.Gardener{
					ID:      "new-gardener",
					Version: "1.80.0",
				},
				MatchError(ContainSubstring("observing Gardener version not up to date")),
			),
			Entry("outdated - non-matching identity version",
				&gardencorev1beta1.Seed{
					Status: gardencorev1beta1.SeedStatus{
						Gardener: &gardencorev1beta1.Gardener{
							Version: "1.80.0",
						},
					},
				},
				&gardencorev1beta1.Gardener{
					Version: "1.81.0",
				},
				MatchError(ContainSubstring("observing Gardener version not up to date")),
			),
			Entry("outdated - observed generation behind",
				&gardencorev1beta1.Seed{
					ObjectMeta: metav1.ObjectMeta{
						Generation: 10,
					},
					Status: gardencorev1beta1.SeedStatus{
						Gardener:           &gardencorev1beta1.Gardener{},
						ObservedGeneration: 9,
					},
				},
				&gardencorev1beta1.Gardener{},
				MatchError(ContainSubstring("observed generation outdated (9/10)")),
			),
			Entry("outdated - zero observed generation",
				&gardencorev1beta1.Seed{
					ObjectMeta: metav1.ObjectMeta{
						Generation: 1,
					},
					Status: gardencorev1beta1.SeedStatus{
						Gardener:           &gardencorev1beta1.Gardener{},
						ObservedGeneration: 0,
					},
				},
				&gardencorev1beta1.Gardener{},
				MatchError(ContainSubstring("observed generation outdated (0/1)")),
			),
			Entry("outdated - nil gardener status",
				&gardencorev1beta1.Seed{
					Status: gardencorev1beta1.SeedStatus{
						Gardener: nil,
					},
				},
				&gardencorev1beta1.Gardener{},
				MatchError(ContainSubstring("observing Gardener version not up to date")),
			),
		)
	})

	Describe("CheckRequiredSeedConditions", func() {
		DescribeTable("seeds",
			func(seed *gardencorev1beta1.Seed, expectError bool, expectedFailedConditionsCount int) {
				failedConditions, err := health.CheckRequiredSeedConditions(seed)
				if expectError {
					Expect(err).To(HaveOccurred())
				} else {
					Expect(err).NotTo(HaveOccurred())
					Expect(failedConditions).To(HaveLen(expectedFailedConditionsCount))
				}
			},
			Entry("all conditions healthy", &gardencorev1beta1.Seed{
				Status: gardencorev1beta1.SeedStatus{
					Conditions: []gardencorev1beta1.Condition{
						{Type: gardencorev1beta1.GardenletReady, Status: gardencorev1beta1.ConditionTrue},
						{Type: gardencorev1beta1.SeedSystemComponentsHealthy, Status: gardencorev1beta1.ConditionTrue},
					},
				},
			}, false, 0),
			Entry("gardenlet ready condition false", &gardencorev1beta1.Seed{
				Status: gardencorev1beta1.SeedStatus{
					Conditions: []gardencorev1beta1.Condition{
						{Type: gardencorev1beta1.GardenletReady, Status: gardencorev1beta1.ConditionFalse, Reason: "NotReady", Message: "Gardenlet is not ready"},
						{Type: gardencorev1beta1.SeedSystemComponentsHealthy, Status: gardencorev1beta1.ConditionTrue},
					},
				},
			}, false, 1),
			Entry("seed system components healthy condition false", &gardencorev1beta1.Seed{
				Status: gardencorev1beta1.SeedStatus{
					Conditions: []gardencorev1beta1.Condition{
						{Type: gardencorev1beta1.GardenletReady, Status: gardencorev1beta1.ConditionTrue},
						{Type: gardencorev1beta1.SeedSystemComponentsHealthy, Status: gardencorev1beta1.ConditionFalse, Reason: "Unhealthy", Message: "Component failed"},
					},
				},
			}, false, 1),
			Entry("all conditions false", &gardencorev1beta1.Seed{
				Status: gardencorev1beta1.SeedStatus{
					Conditions: []gardencorev1beta1.Condition{
						{Type: gardencorev1beta1.GardenletReady, Status: gardencorev1beta1.ConditionFalse},
						{Type: gardencorev1beta1.SeedSystemComponentsHealthy, Status: gardencorev1beta1.ConditionFalse},
					},
				},
			}, false, 2),
			Entry("gardenlet ready condition unknown", &gardencorev1beta1.Seed{
				Status: gardencorev1beta1.SeedStatus{
					Conditions: []gardencorev1beta1.Condition{
						{Type: gardencorev1beta1.GardenletReady, Status: gardencorev1beta1.ConditionUnknown},
						{Type: gardencorev1beta1.SeedSystemComponentsHealthy, Status: gardencorev1beta1.ConditionTrue},
					},
				},
			}, false, 1),
			Entry("seed system components healthy condition progressing", &gardencorev1beta1.Seed{
				Status: gardencorev1beta1.SeedStatus{
					Conditions: []gardencorev1beta1.Condition{
						{Type: gardencorev1beta1.GardenletReady, Status: gardencorev1beta1.ConditionTrue},
						{Type: gardencorev1beta1.SeedSystemComponentsHealthy, Status: gardencorev1beta1.ConditionProgressing},
					},
				},
			}, false, 1),
			Entry("missing gardenlet ready condition", &gardencorev1beta1.Seed{
				Status: gardencorev1beta1.SeedStatus{
					Conditions: []gardencorev1beta1.Condition{
						{Type: gardencorev1beta1.SeedSystemComponentsHealthy, Status: gardencorev1beta1.ConditionTrue},
					},
				},
			}, true, 0),
			Entry("missing seed system components healthy condition", &gardencorev1beta1.Seed{
				Status: gardencorev1beta1.SeedStatus{
					Conditions: []gardencorev1beta1.Condition{
						{Type: gardencorev1beta1.GardenletReady, Status: gardencorev1beta1.ConditionTrue},
					},
				},
			}, true, 0),
			Entry("missing all conditions", &gardencorev1beta1.Seed{
				Status: gardencorev1beta1.SeedStatus{
					Conditions: []gardencorev1beta1.Condition{},
				},
			}, true, 0),
			Entry("nil conditions", &gardencorev1beta1.Seed{
				Status: gardencorev1beta1.SeedStatus{
					Conditions: nil,
				},
			}, true, 0),
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
						{Type: gardencorev1beta1.GardenletReady, Status: gardencorev1beta1.ConditionTrue},
						{Type: gardencorev1beta1.SeedSystemComponentsHealthy, Status: gardencorev1beta1.ConditionTrue},
					},
				},
			}, &gardencorev1beta1.Gardener{Version: "1.12.8"}, Succeed()),
			Entry("healthy with non-matching version", &gardencorev1beta1.Seed{
				Status: gardencorev1beta1.SeedStatus{
					Gardener: &gardencorev1beta1.Gardener{Version: "1.12.8"},
					Conditions: []gardencorev1beta1.Condition{
						{Type: gardencorev1beta1.GardenletReady, Status: gardencorev1beta1.ConditionTrue},
						{Type: gardencorev1beta1.SeedSystemComponentsHealthy, Status: gardencorev1beta1.ConditionTrue},
					},
				},
			}, &gardencorev1beta1.Gardener{Version: "1.13.8"}, HaveOccurred()),
			Entry("unhealthy available condition (gardenlet ready) and matching version", &gardencorev1beta1.Seed{
				Status: gardencorev1beta1.SeedStatus{
					Gardener: &gardencorev1beta1.Gardener{Version: "1.12.8"},
					Conditions: []gardencorev1beta1.Condition{
						{Type: gardencorev1beta1.GardenletReady, Status: gardencorev1beta1.ConditionFalse},
						{Type: gardencorev1beta1.SeedSystemComponentsHealthy, Status: gardencorev1beta1.ConditionTrue},
					},
				},
			}, &gardencorev1beta1.Gardener{Version: "1.12.8"}, HaveOccurred()),
			Entry("unhealthy available condition (seed system components healthy) and matching version", &gardencorev1beta1.Seed{
				Status: gardencorev1beta1.SeedStatus{
					Gardener: &gardencorev1beta1.Gardener{Version: "1.12.8"},
					Conditions: []gardencorev1beta1.Condition{
						{Type: gardencorev1beta1.GardenletReady, Status: gardencorev1beta1.ConditionTrue},
						{Type: gardencorev1beta1.SeedSystemComponentsHealthy, Status: gardencorev1beta1.ConditionFalse},
					},
				},
			}, &gardencorev1beta1.Gardener{Version: "1.12.8"}, HaveOccurred()),
			Entry("unhealthy available condition (all conditions) and matching version", &gardencorev1beta1.Seed{
				Status: gardencorev1beta1.SeedStatus{
					Gardener: &gardencorev1beta1.Gardener{Version: "1.12.8"},
					Conditions: []gardencorev1beta1.Condition{
						{Type: gardencorev1beta1.GardenletReady, Status: gardencorev1beta1.ConditionFalse},
						{Type: gardencorev1beta1.SeedSystemComponentsHealthy, Status: gardencorev1beta1.ConditionFalse},
					},
				},
			}, &gardencorev1beta1.Gardener{Version: "1.12.8"}, HaveOccurred()),
		)
	})
})

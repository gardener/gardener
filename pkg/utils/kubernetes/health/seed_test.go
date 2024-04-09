// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

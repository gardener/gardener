// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package shoot_test

import (
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/controllermanager/controller/shoot"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("#FilterSeedForShootConditions", func() {
	var (
		oldSeed, newSeed *gardencorev1beta1.Seed
		gardenletReady   = []gardencorev1beta1.Condition{
			{
				Type:   gardencorev1beta1.SeedGardenletReady,
				Status: gardencorev1beta1.ConditionTrue,
			}}
		gardenletNotReady = []gardencorev1beta1.Condition{
			{
				Type:   gardencorev1beta1.SeedGardenletReady,
				Status: gardencorev1beta1.ConditionFalse,
			}}
	)

	BeforeEach(func() {
		oldSeed = &gardencorev1beta1.Seed{}
		newSeed = &gardencorev1beta1.Seed{}
	})

	It("should accept in case of cache resync update events", func() {
		newSeed.ResourceVersion = "1"
		oldSeed.ResourceVersion = "1"

		Expect(shoot.FilterSeedForShootConditions(newSeed, oldSeed, nil, false)).To(BeTrue())
	})
	It("should accept in case of deletion events", func() {
		Expect(shoot.FilterSeedForShootConditions(newSeed, nil, nil, true)).To(BeTrue())
	})
	It("should accept in case of create events", func() {
		Expect(shoot.FilterSeedForShootConditions(newSeed, nil, nil, false)).To(BeTrue())
	})
	It("should accept if conditions differ", func() {
		newSeed.ResourceVersion = "1"
		oldSeed.ResourceVersion = "2"
		newSeed.Status.Conditions = gardenletReady
		oldSeed.Status.Conditions = gardenletNotReady

		Expect(shoot.FilterSeedForShootConditions(newSeed, oldSeed, nil, false)).To(BeTrue())
	})
	It("should deny if conditions are the same", func() {
		newSeed.ResourceVersion = "1"
		oldSeed.ResourceVersion = "2"
		newSeed.Status.Conditions = gardenletReady
		oldSeed.Status.Conditions = gardenletReady

		Expect(shoot.FilterSeedForShootConditions(newSeed, oldSeed, nil, false)).To(BeFalse())
	})
})

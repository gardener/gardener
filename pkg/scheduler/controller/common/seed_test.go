// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"

	. "github.com/gardener/gardener/pkg/scheduler/controller/common"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
)

var _ = Describe("#VerifySeedReadiness", func() {
	var seed *gardencorev1beta1.Seed

	DescribeTable("condition is false",
		func(conditionType gardencorev1beta1.ConditionType, backup bool, expected gomegatypes.GomegaMatcher) {
			var seedBackup *gardencorev1beta1.SeedBackup
			if backup {
				seedBackup = &gardencorev1beta1.SeedBackup{}
			}
			seed = &gardencorev1beta1.Seed{
				Spec: gardencorev1beta1.SeedSpec{
					Backup: seedBackup,
				},
				Status: gardencorev1beta1.SeedStatus{
					Conditions: []gardencorev1beta1.Condition{
						{Type: gardencorev1beta1.SeedBootstrapped, Status: gardencorev1beta1.ConditionTrue},
						{Type: gardencorev1beta1.SeedGardenletReady, Status: gardencorev1beta1.ConditionTrue},
						{Type: gardencorev1beta1.SeedBackupBucketsReady, Status: gardencorev1beta1.ConditionTrue},
						{Type: gardencorev1beta1.SeedExtensionsReady, Status: gardencorev1beta1.ConditionTrue},
					},
				},
			}

			for i, cond := range seed.Status.Conditions {
				if cond.Type == conditionType {
					seed.Status.Conditions[i].Status = gardencorev1beta1.ConditionFalse
					break
				}
			}
			Expect(VerifySeedReadiness(seed)).To(expected)
		},
		Entry("SeedBootstrapped is false", gardencorev1beta1.SeedBootstrapped, true, BeFalse()),
		Entry("SeedGardenletReady is false", gardencorev1beta1.SeedGardenletReady, true, BeFalse()),
		Entry("SeedBackupBucketsReady is false", gardencorev1beta1.SeedBackupBucketsReady, true, BeFalse()),
		Entry("SeedBackupBucketsReady is false but no backup specified", gardencorev1beta1.SeedBackupBucketsReady, false, BeTrue()),
		Entry("SeedExtensionsReady is false", gardencorev1beta1.SeedExtensionsReady, true, BeTrue()),
	)

})

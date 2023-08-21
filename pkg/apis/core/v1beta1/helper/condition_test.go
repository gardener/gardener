// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package helper_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	testclock "k8s.io/utils/clock/testing"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("condition", func() {
	var (
		zeroTime     time.Time
		zeroMetaTime metav1.Time

		fakeClock = testclock.NewFakeClock(time.Now())
	)

	DescribeTable("#FailedCondition",
		func(thresholds map[gardencorev1beta1.ConditionType]time.Duration, lastOperation *gardencorev1beta1.LastOperation, now time.Time, condition gardencorev1beta1.Condition, reason, message string, expected types.GomegaMatcher) {
			fakeClock.SetTime(now)
			Expect(FailedCondition(fakeClock, lastOperation, thresholds, condition, reason, message)).To(expected)
		},
		Entry("true condition with threshold",
			map[gardencorev1beta1.ConditionType]time.Duration{
				gardencorev1beta1.ShootControlPlaneHealthy: time.Minute,
			},
			nil,
			zeroTime,
			gardencorev1beta1.Condition{
				Type:   gardencorev1beta1.ShootControlPlaneHealthy,
				Status: gardencorev1beta1.ConditionTrue,
			},
			"",
			"",
			beConditionWithStatus(gardencorev1beta1.ConditionProgressing)),
		Entry("true condition without condition threshold",
			map[gardencorev1beta1.ConditionType]time.Duration{},
			nil,
			zeroTime,
			gardencorev1beta1.Condition{
				Type:   gardencorev1beta1.ShootControlPlaneHealthy,
				Status: gardencorev1beta1.ConditionTrue,
			},
			"",
			"",
			beConditionWithStatus(gardencorev1beta1.ConditionFalse)),
		Entry("progressing condition within last operation update time threshold",
			map[gardencorev1beta1.ConditionType]time.Duration{
				gardencorev1beta1.ShootControlPlaneHealthy: time.Minute,
			},
			&gardencorev1beta1.LastOperation{
				State:          gardencorev1beta1.LastOperationStateSucceeded,
				LastUpdateTime: zeroMetaTime,
			},
			zeroTime,
			gardencorev1beta1.Condition{
				Type:   gardencorev1beta1.ShootControlPlaneHealthy,
				Status: gardencorev1beta1.ConditionProgressing,
			},
			"",
			"",
			beConditionWithStatus(gardencorev1beta1.ConditionProgressing)),
		Entry("progressing condition outside last operation update time threshold but within last transition time threshold",
			map[gardencorev1beta1.ConditionType]time.Duration{
				gardencorev1beta1.ShootControlPlaneHealthy: time.Minute,
			},
			&gardencorev1beta1.LastOperation{
				State:          gardencorev1beta1.LastOperationStateSucceeded,
				LastUpdateTime: zeroMetaTime,
			},
			zeroTime.Add(time.Minute+time.Second),
			gardencorev1beta1.Condition{
				Type:               gardencorev1beta1.ShootControlPlaneHealthy,
				Status:             gardencorev1beta1.ConditionProgressing,
				LastTransitionTime: metav1.Time{Time: zeroMetaTime.Add(time.Minute)},
			},
			"",
			"",
			beConditionWithStatus(gardencorev1beta1.ConditionProgressing)),
		Entry("progressing condition outside last operation update time threshold and last transition time threshold",
			map[gardencorev1beta1.ConditionType]time.Duration{
				gardencorev1beta1.ShootControlPlaneHealthy: time.Minute,
			},
			&gardencorev1beta1.LastOperation{
				State:          gardencorev1beta1.LastOperationStateSucceeded,
				LastUpdateTime: zeroMetaTime,
			},
			zeroTime.Add(time.Minute+time.Second),
			gardencorev1beta1.Condition{
				Type:   gardencorev1beta1.ShootControlPlaneHealthy,
				Status: gardencorev1beta1.ConditionProgressing,
			},
			"",
			"",
			beConditionWithStatus(gardencorev1beta1.ConditionFalse)),
		Entry("failed condition within last operation update time threshold",
			map[gardencorev1beta1.ConditionType]time.Duration{
				gardencorev1beta1.ShootControlPlaneHealthy: time.Minute,
			},
			&gardencorev1beta1.LastOperation{
				State:          gardencorev1beta1.LastOperationStateSucceeded,
				LastUpdateTime: zeroMetaTime,
			},
			zeroTime.Add(time.Minute-time.Second),
			gardencorev1beta1.Condition{
				Type:   gardencorev1beta1.ShootControlPlaneHealthy,
				Status: gardencorev1beta1.ConditionFalse,
			},
			"",
			"",
			beConditionWithStatus(gardencorev1beta1.ConditionProgressing)),
		Entry("failed condition outside of last operation update time threshold with same reason",
			map[gardencorev1beta1.ConditionType]time.Duration{
				gardencorev1beta1.ShootControlPlaneHealthy: time.Minute,
			},
			&gardencorev1beta1.LastOperation{
				State:          gardencorev1beta1.LastOperationStateSucceeded,
				LastUpdateTime: zeroMetaTime,
			},
			zeroTime.Add(time.Minute+time.Second),
			gardencorev1beta1.Condition{
				Type:   gardencorev1beta1.ShootControlPlaneHealthy,
				Status: gardencorev1beta1.ConditionFalse,
				Reason: "Reason",
			},
			"Reason",
			"",
			beConditionWithStatus(gardencorev1beta1.ConditionFalse)),
		Entry("failed condition outside of last operation update time threshold with a different reason",
			map[gardencorev1beta1.ConditionType]time.Duration{
				gardencorev1beta1.ShootControlPlaneHealthy: time.Minute,
			},
			&gardencorev1beta1.LastOperation{
				State:          gardencorev1beta1.LastOperationStateSucceeded,
				LastUpdateTime: zeroMetaTime,
			},
			zeroTime.Add(time.Minute+time.Second),
			gardencorev1beta1.Condition{
				Type:   gardencorev1beta1.ShootControlPlaneHealthy,
				Status: gardencorev1beta1.ConditionFalse,
				Reason: "foo",
			},
			"bar",
			"",
			beConditionWithStatus(gardencorev1beta1.ConditionProgressing)),
		Entry("failed condition outside of last operation update time threshold with a different message",
			map[gardencorev1beta1.ConditionType]time.Duration{
				gardencorev1beta1.ShootControlPlaneHealthy: time.Minute,
			},
			&gardencorev1beta1.LastOperation{
				State:          gardencorev1beta1.LastOperationStateSucceeded,
				LastUpdateTime: zeroMetaTime,
			},
			zeroTime.Add(time.Minute+time.Second),
			gardencorev1beta1.Condition{
				Type:    gardencorev1beta1.ShootControlPlaneHealthy,
				Status:  gardencorev1beta1.ConditionFalse,
				Message: "foo",
			},
			"",
			"bar",
			beConditionWithStatus(gardencorev1beta1.ConditionFalse)),
		Entry("failed condition without thresholds",
			map[gardencorev1beta1.ConditionType]time.Duration{},
			nil,
			zeroTime,
			gardencorev1beta1.Condition{
				Type:   gardencorev1beta1.ShootControlPlaneHealthy,
				Status: gardencorev1beta1.ConditionFalse,
			},
			"",
			"",
			beConditionWithStatus(gardencorev1beta1.ConditionFalse)),
	)
})

func beConditionWithStatus(status gardencorev1beta1.ConditionStatus) types.GomegaMatcher {
	return WithStatus(status)
}

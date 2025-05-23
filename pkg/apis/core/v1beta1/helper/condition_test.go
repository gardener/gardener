// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper_test

import (
	"errors"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	testclock "k8s.io/utils/clock/testing"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Condition", func() {
	var (
		zeroTime     time.Time
		zeroMetaTime metav1.Time

		fakeClock *testclock.FakeClock
	)

	BeforeEach(func() {
		fakeClock = testclock.NewFakeClock(time.Now())
	})

	Describe("#GetOrInitConditionWithClock", func() {
		It("should get the existing condition", func() {
			var (
				c          = gardencorev1beta1.Condition{Type: "foo"}
				conditions = []gardencorev1beta1.Condition{c}
			)

			Expect(GetOrInitConditionWithClock(fakeClock, conditions, "foo")).To(Equal(c))
		})

		It("should return a new, initialized condition", func() {
			Expect(GetOrInitConditionWithClock(fakeClock, nil, "foo")).To(Equal(InitConditionWithClock(fakeClock, "foo")))
		})
	})

	Describe("#GetCondition", func() {
		It("should return the found condition", func() {
			var (
				conditionType gardencorev1beta1.ConditionType = "test-1"
				condition                                     = gardencorev1beta1.Condition{
					Type: conditionType,
				}
				conditions = []gardencorev1beta1.Condition{condition}
			)

			cond := GetCondition(conditions, conditionType)

			Expect(cond).NotTo(BeNil())
			Expect(*cond).To(Equal(condition))
		})

		It("should return nil because the required condition could not be found", func() {
			var (
				conditionType gardencorev1beta1.ConditionType = "test-1"
				conditions    []gardencorev1beta1.Condition
			)

			cond := GetCondition(conditions, conditionType)

			Expect(cond).To(BeNil())
		})
	})

	DescribeTable("#FailedCondition",
		func(thresholds map[gardencorev1beta1.ConditionType]time.Duration, lastOperation *gardencorev1beta1.LastOperation, now time.Time, condition gardencorev1beta1.Condition, reason, message string, expected gomegatypes.GomegaMatcher) {
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

	var (
		testTime      = metav1.NewTime(time.Unix(10, 10))
		afterTestTime = func(t metav1.Time) bool { return t.After(testTime.Time) }
	)

	DescribeTable("#UpdatedConditionWithClock",
		func(condition gardencorev1beta1.Condition, status gardencorev1beta1.ConditionStatus, reason, message string, codes []gardencorev1beta1.ErrorCode, matcher gomegatypes.GomegaMatcher) {
			updated := UpdatedConditionWithClock(fakeClock, condition, status, reason, message, codes...)

			Expect(updated).To(matcher)
		},
		Entry("initialize empty timestamps",
			gardencorev1beta1.Condition{
				Type:    "type",
				Status:  gardencorev1beta1.ConditionTrue,
				Reason:  "reason",
				Message: "message",
			},
			gardencorev1beta1.ConditionTrue,
			"reason",
			"message",
			nil,
			MatchFields(IgnoreExtras, Fields{
				"Status":             Equal(gardencorev1beta1.ConditionTrue),
				"Reason":             Equal("reason"),
				"Message":            Equal("message"),
				"LastTransitionTime": Not(Equal(zeroTime)),
				"LastUpdateTime":     Not(Equal(zeroTime)),
			}),
		),
		Entry("no update",
			gardencorev1beta1.Condition{
				Type:               "type",
				Status:             gardencorev1beta1.ConditionTrue,
				Reason:             "reason",
				Message:            "message",
				LastTransitionTime: testTime,
				LastUpdateTime:     testTime,
			},
			gardencorev1beta1.ConditionTrue,
			"reason",
			"message",
			nil,
			MatchFields(IgnoreExtras, Fields{
				"Status":             Equal(gardencorev1beta1.ConditionTrue),
				"Reason":             Equal("reason"),
				"Message":            Equal("message"),
				"LastTransitionTime": Equal(testTime),
				"LastUpdateTime":     Equal(testTime),
			}),
		),
		Entry("update reason",
			gardencorev1beta1.Condition{
				Type:               "type",
				Status:             gardencorev1beta1.ConditionTrue,
				Reason:             "reason",
				Message:            "message",
				LastTransitionTime: testTime,
				LastUpdateTime:     testTime,
			},
			gardencorev1beta1.ConditionTrue,
			"OtherReason",
			"message",
			nil,
			MatchFields(IgnoreExtras, Fields{
				"Status":             Equal(gardencorev1beta1.ConditionTrue),
				"Reason":             Equal("OtherReason"),
				"Message":            Equal("message"),
				"LastTransitionTime": Equal(testTime),
				"LastUpdateTime":     Satisfy(afterTestTime),
			}),
		),
		Entry("update codes",
			gardencorev1beta1.Condition{
				Type:               "type",
				Status:             gardencorev1beta1.ConditionTrue,
				Reason:             "reason",
				Message:            "message",
				LastTransitionTime: testTime,
				LastUpdateTime:     testTime,
			},
			gardencorev1beta1.ConditionTrue,
			"reason",
			"message",
			[]gardencorev1beta1.ErrorCode{gardencorev1beta1.ErrorInfraQuotaExceeded},
			MatchFields(IgnoreExtras, Fields{
				"Status":             Equal(gardencorev1beta1.ConditionTrue),
				"Reason":             Equal("reason"),
				"Message":            Equal("message"),
				"Codes":              Equal([]gardencorev1beta1.ErrorCode{gardencorev1beta1.ErrorInfraQuotaExceeded}),
				"LastTransitionTime": Equal(testTime),
				"LastUpdateTime":     Satisfy(afterTestTime),
			}),
		),
		Entry("update status",
			gardencorev1beta1.Condition{
				Type:               "type",
				Status:             gardencorev1beta1.ConditionTrue,
				Reason:             "reason",
				Message:            "message",
				LastTransitionTime: testTime,
				LastUpdateTime:     testTime,
			},
			gardencorev1beta1.ConditionFalse,
			"reason",
			"message",
			nil,
			MatchFields(IgnoreExtras, Fields{
				"Status":             Equal(gardencorev1beta1.ConditionFalse),
				"Reason":             Equal("reason"),
				"Message":            Equal("message"),
				"LastTransitionTime": Satisfy(afterTestTime),
				"LastUpdateTime":     Equal(testTime),
			}),
		),
		Entry("clear codes",
			gardencorev1beta1.Condition{
				Type:               "type",
				Status:             gardencorev1beta1.ConditionTrue,
				Reason:             "reason",
				Message:            "message",
				LastTransitionTime: testTime,
				LastUpdateTime:     testTime,
				Codes:              []gardencorev1beta1.ErrorCode{gardencorev1beta1.ErrorInfraQuotaExceeded},
			},
			gardencorev1beta1.ConditionTrue,
			"reason",
			"message",
			nil,
			MatchFields(IgnoreExtras, Fields{
				"Status":             Equal(gardencorev1beta1.ConditionTrue),
				"Reason":             Equal("reason"),
				"Message":            Equal("message"),
				"LastTransitionTime": Equal(testTime),
				"LastUpdateTime":     Satisfy(afterTestTime),
				"Codes":              BeEmpty(),
			}),
		),
	)

	Describe("#BuildConditions", func() {
		var (
			conditionTypes = []gardencorev1beta1.ConditionType{"foo"}

			fooCondition = gardencorev1beta1.Condition{
				Type:               "foo",
				Status:             gardencorev1beta1.ConditionTrue,
				Reason:             "foo reason",
				Message:            "foo message",
				LastTransitionTime: testTime,
				LastUpdateTime:     testTime,
			}
			barCondition = gardencorev1beta1.Condition{
				Type:               "bar",
				Status:             gardencorev1beta1.ConditionTrue,
				Reason:             "bar reason",
				Message:            "bar message",
				LastTransitionTime: testTime,
				LastUpdateTime:     testTime,
			}
			conditions = []gardencorev1beta1.Condition{fooCondition}

			newConditions []gardencorev1beta1.Condition
		)

		BeforeEach(func() {
			newFooCondition := fooCondition.DeepCopy()
			newFooCondition.LastTransitionTime = metav1.NewTime(time.Unix(11, 11))
			newConditions = []gardencorev1beta1.Condition{*newFooCondition}
		})

		It("should replace the existing condition", func() {
			Expect(BuildConditions(conditions, newConditions, conditionTypes)).To(ConsistOf(newConditions))
		})

		It("should keep existing conditions of a different type", func() {
			conditions = append(conditions, barCondition)
			Expect(BuildConditions(conditions, newConditions, conditionTypes)).To(ConsistOf(append(newConditions, barCondition)))
		})
	})

	Describe("#MergeConditions", func() {
		It("should merge the conditions", func() {
			var (
				typeFoo gardencorev1beta1.ConditionType = "foo"
				typeBar gardencorev1beta1.ConditionType = "bar"
			)

			oldConditions := []gardencorev1beta1.Condition{
				{
					Type:   typeFoo,
					Reason: "hugo",
				},
			}

			result := MergeConditions(oldConditions, gardencorev1beta1.Condition{Type: typeFoo}, gardencorev1beta1.Condition{Type: typeBar})

			Expect(result).To(Equal([]gardencorev1beta1.Condition{{Type: typeFoo}, {Type: typeBar}}))
		})
	})

	DescribeTable("#RemoveConditions",
		func(conditions []gardencorev1beta1.Condition, conditionTypes []gardencorev1beta1.ConditionType, expectedResult []gardencorev1beta1.Condition) {
			Expect(RemoveConditions(conditions, conditionTypes...)).To(Equal(expectedResult))
		},
		Entry("remove foo", []gardencorev1beta1.Condition{{Type: "foo"}, {Type: "bar"}}, []gardencorev1beta1.ConditionType{"foo"},
			[]gardencorev1beta1.Condition{{Type: "bar"}}),
		Entry("remove bar", []gardencorev1beta1.Condition{{Type: "foo"}, {Type: "bar"}}, []gardencorev1beta1.ConditionType{"bar"},
			[]gardencorev1beta1.Condition{{Type: "foo"}}),
		Entry("don't remove anything", []gardencorev1beta1.Condition{{Type: "foo"}, {Type: "bar"}}, nil,
			[]gardencorev1beta1.Condition{{Type: "foo"}, {Type: "bar"}}),
		Entry("remove from an empty slice", nil, []gardencorev1beta1.ConditionType{"foo"}, nil),
	)

	DescribeTable("#RetainConditions",
		func(conditions []gardencorev1beta1.Condition, conditionTypes []gardencorev1beta1.ConditionType, expectedResult []gardencorev1beta1.Condition) {
			Expect(RetainConditions(conditions, conditionTypes...)).To(Equal(expectedResult))
		},
		Entry("remove foo", []gardencorev1beta1.Condition{{Type: "foo"}, {Type: "bar"}}, []gardencorev1beta1.ConditionType{"bar"},
			[]gardencorev1beta1.Condition{{Type: "bar"}}),
		Entry("remove bar", []gardencorev1beta1.Condition{{Type: "foo"}, {Type: "bar"}}, []gardencorev1beta1.ConditionType{"foo"},
			[]gardencorev1beta1.Condition{{Type: "foo"}}),
		Entry("remove anything", []gardencorev1beta1.Condition{{Type: "foo"}, {Type: "bar"}}, nil,
			nil),
		Entry("don't remove anything", []gardencorev1beta1.Condition{{Type: "foo"}, {Type: "bar"}}, []gardencorev1beta1.ConditionType{"foo", "bar"},
			[]gardencorev1beta1.Condition{{Type: "foo"}, {Type: "bar"}}),
		Entry("remove from an empty slice", nil, []gardencorev1beta1.ConditionType{"foo"}, nil),
	)

	Describe("#NewConditionOrError", func() {
		It("should return the condition", func() {
			condition := gardencorev1beta1.Condition{Type: "foo"}
			Expect(NewConditionOrError(fakeClock, gardencorev1beta1.Condition{Type: "foo"}, &condition, nil)).To(Equal(condition))
		})

		It("should update the condition to 'UNKNOWN' if new condition is 'nil'", func() {
			conditions := NewConditionOrError(fakeClock, gardencorev1beta1.Condition{Type: "foo"}, nil, nil)
			Expect(conditions.Status).To(Equal(gardencorev1beta1.ConditionStatus("Unknown")))
			Expect(conditions.Reason).To(Equal("ConditionCheckError"))
		})

		It("should update the condition to 'UNKNOWN' in case of an error", func() {
			conditions := NewConditionOrError(fakeClock, gardencorev1beta1.Condition{Type: "foo"}, &gardencorev1beta1.Condition{Type: "foo"}, errors.New(""))
			Expect(conditions.Status).To(Equal(gardencorev1beta1.ConditionStatus("Unknown")))
			Expect(conditions.Reason).To(Equal("ConditionCheckError"))
		})
	})
})

func beConditionWithStatus(status gardencorev1beta1.ConditionStatus) gomegatypes.GomegaMatcher {
	return WithStatus(status)
}

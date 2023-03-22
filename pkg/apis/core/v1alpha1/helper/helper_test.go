// Copyright 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	. "github.com/onsi/gomega/gstruct"
	"github.com/onsi/gomega/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	. "github.com/gardener/gardener/pkg/apis/core/v1alpha1/helper"
)

var _ = Describe("helper", func() {
	Describe("errors", func() {
		var (
			testTime      = metav1.NewTime(time.Unix(10, 10))
			zeroTime      metav1.Time
			afterTestTime = func(t metav1.Time) bool { return t.After(testTime.Time) }
		)

		DescribeTable("#UpdatedCondition",
			func(condition gardencorev1alpha1.Condition, status gardencorev1alpha1.ConditionStatus, reason, message string, codes []gardencorev1alpha1.ErrorCode, matcher types.GomegaMatcher) {
				updated := UpdatedCondition(condition, status, reason, message, codes...)

				Expect(updated).To(matcher)
			},
			Entry("initialize empty timestamps",
				gardencorev1alpha1.Condition{
					Type:    "type",
					Status:  gardencorev1alpha1.ConditionTrue,
					Reason:  "reason",
					Message: "message",
				},
				gardencorev1alpha1.ConditionTrue,
				"reason",
				"message",
				nil,
				MatchFields(IgnoreExtras, Fields{
					"Status":             Equal(gardencorev1alpha1.ConditionTrue),
					"Reason":             Equal("reason"),
					"Message":            Equal("message"),
					"LastTransitionTime": Not(Equal(zeroTime)),
					"LastUpdateTime":     Not(Equal(zeroTime)),
				}),
			),
			Entry("no update",
				gardencorev1alpha1.Condition{
					Type:               "type",
					Status:             gardencorev1alpha1.ConditionTrue,
					Reason:             "reason",
					Message:            "message",
					LastTransitionTime: testTime,
					LastUpdateTime:     testTime,
				},
				gardencorev1alpha1.ConditionTrue,
				"reason",
				"message",
				nil,
				MatchFields(IgnoreExtras, Fields{
					"Status":             Equal(gardencorev1alpha1.ConditionTrue),
					"Reason":             Equal("reason"),
					"Message":            Equal("message"),
					"LastTransitionTime": Equal(testTime),
					"LastUpdateTime":     Equal(testTime),
				}),
			),
			Entry("update reason",
				gardencorev1alpha1.Condition{
					Type:               "type",
					Status:             gardencorev1alpha1.ConditionTrue,
					Reason:             "reason",
					Message:            "message",
					LastTransitionTime: testTime,
					LastUpdateTime:     testTime,
				},
				gardencorev1alpha1.ConditionTrue,
				"OtherReason",
				"message",
				nil,
				MatchFields(IgnoreExtras, Fields{
					"Status":             Equal(gardencorev1alpha1.ConditionTrue),
					"Reason":             Equal("OtherReason"),
					"Message":            Equal("message"),
					"LastTransitionTime": Equal(testTime),
					"LastUpdateTime":     Satisfy(afterTestTime),
				}),
			),
			Entry("update codes",
				gardencorev1alpha1.Condition{
					Type:               "type",
					Status:             gardencorev1alpha1.ConditionTrue,
					Reason:             "reason",
					Message:            "message",
					LastTransitionTime: testTime,
					LastUpdateTime:     testTime,
				},
				gardencorev1alpha1.ConditionTrue,
				"reason",
				"message",
				[]gardencorev1alpha1.ErrorCode{gardencorev1alpha1.ErrorInfraQuotaExceeded},
				MatchFields(IgnoreExtras, Fields{
					"Status":             Equal(gardencorev1alpha1.ConditionTrue),
					"Reason":             Equal("reason"),
					"Message":            Equal("message"),
					"Codes":              Equal([]gardencorev1alpha1.ErrorCode{gardencorev1alpha1.ErrorInfraQuotaExceeded}),
					"LastTransitionTime": Equal(testTime),
					"LastUpdateTime":     Satisfy(afterTestTime),
				}),
			),
			Entry("update status",
				gardencorev1alpha1.Condition{
					Type:               "type",
					Status:             gardencorev1alpha1.ConditionTrue,
					Reason:             "reason",
					Message:            "message",
					LastTransitionTime: testTime,
					LastUpdateTime:     testTime,
				},
				gardencorev1alpha1.ConditionFalse,
				"reason",
				"message",
				nil,
				MatchFields(IgnoreExtras, Fields{
					"Status":             Equal(gardencorev1alpha1.ConditionFalse),
					"Reason":             Equal("reason"),
					"Message":            Equal("message"),
					"LastTransitionTime": Satisfy(afterTestTime),
					"LastUpdateTime":     Equal(testTime),
				}),
			),
			Entry("clear codes",
				gardencorev1alpha1.Condition{
					Type:               "type",
					Status:             gardencorev1alpha1.ConditionTrue,
					Reason:             "reason",
					Message:            "message",
					LastTransitionTime: testTime,
					LastUpdateTime:     testTime,
					Codes:              []gardencorev1alpha1.ErrorCode{gardencorev1alpha1.ErrorInfraQuotaExceeded},
				},
				gardencorev1alpha1.ConditionTrue,
				"reason",
				"message",
				nil,
				MatchFields(IgnoreExtras, Fields{
					"Status":             Equal(gardencorev1alpha1.ConditionTrue),
					"Reason":             Equal("reason"),
					"Message":            Equal("message"),
					"LastTransitionTime": Equal(testTime),
					"LastUpdateTime":     Satisfy(afterTestTime),
					"Codes":              BeEmpty(),
				}),
			),
		)

		Describe("#MergeConditions", func() {
			It("should merge the conditions", func() {
				var (
					typeFoo gardencorev1alpha1.ConditionType = "foo"
					typeBar gardencorev1alpha1.ConditionType = "bar"
				)

				oldConditions := []gardencorev1alpha1.Condition{
					{
						Type:   typeFoo,
						Reason: "hugo",
					},
				}

				result := MergeConditions(oldConditions, gardencorev1alpha1.Condition{Type: typeFoo}, gardencorev1alpha1.Condition{Type: typeBar})

				Expect(result).To(Equal([]gardencorev1alpha1.Condition{{Type: typeFoo}, {Type: typeBar}}))
			})
		})

		Describe("#GetCondition", func() {
			It("should return the found condition", func() {
				var (
					conditionType gardencorev1alpha1.ConditionType = "test-1"
					condition                                      = gardencorev1alpha1.Condition{
						Type: conditionType,
					}
					conditions = []gardencorev1alpha1.Condition{condition}
				)

				cond := GetCondition(conditions, conditionType)

				Expect(cond).NotTo(BeNil())
				Expect(*cond).To(Equal(condition))
			})

			It("should return nil because the required condition could not be found", func() {
				var (
					conditionType gardencorev1alpha1.ConditionType = "test-1"
					conditions                                     = []gardencorev1alpha1.Condition{}
				)

				cond := GetCondition(conditions, conditionType)

				Expect(cond).To(BeNil())
			})
		})

		Describe("#GetOrInitCondition", func() {
			It("should get the existing condition", func() {
				var (
					c          = gardencorev1alpha1.Condition{Type: "foo"}
					conditions = []gardencorev1alpha1.Condition{c}
				)

				Expect(GetOrInitCondition(conditions, "foo")).To(Equal(c))
			})

			It("should return a new, initialized condition", func() {
				tmp := Now
				Now = func() metav1.Time {
					return metav1.NewTime(time.Unix(0, 0))
				}
				defer func() { Now = tmp }()

				Expect(GetOrInitCondition(nil, "foo")).To(Equal(InitCondition("foo")))
			})
		})
	})
})

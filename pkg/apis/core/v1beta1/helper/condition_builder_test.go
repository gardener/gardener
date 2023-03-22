// Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/clock"
	testclock "k8s.io/utils/clock/testing"
	"k8s.io/utils/pointer"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
)

var _ = Describe("Builder", func() {
	const (
		conditionType = gardencorev1beta1.ConditionType("Test")
		// re-decalared so the underlying constant is not changed
		unknowStatus       = gardencorev1beta1.ConditionStatus("Unknown")
		fooStatus          = gardencorev1beta1.ConditionStatus("Foo")
		bazReason          = "Baz"
		fubarMessage       = "FuBar"
		unitializedMessage = `The condition has been initialized but its semantic check has not been performed yet.`
		unspecifiedMessage = `No message given.`
		unspecifiedReason  = "Unspecified"
		initializedReason  = "ConditionInitialized"
	)

	var (
		defaultTime  metav1.Time
		defaultClock clock.Clock
		codes        = []gardencorev1beta1.ErrorCode{
			gardencorev1beta1.ErrorInfraDependencies,
		}
	)

	BeforeEach(func() {
		defaultTime = metav1.NewTime(time.Unix(2, 2))
		defaultClock = testclock.NewFakeClock(defaultTime.Time)
	})

	Describe("#NewConditionBuilder", func() {
		It("should return error if condition type is empty", func() {
			bldr, err := NewConditionBuilder("")

			Expect(bldr).To(BeNil())
			Expect(err).To(HaveOccurred())
		})

		It("should return not empty builder on success", func() {
			bldr, err := NewConditionBuilder("Foo")

			Expect(bldr).ToNot(BeNil())
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("#Build", func() {
		var (
			result  gardencorev1beta1.Condition
			updated bool
			bldr    ConditionBuilder
		)

		JustBeforeEach(func() {
			bldr, _ = NewConditionBuilder(conditionType)
		})

		Context("empty condition", func() {
			JustBeforeEach(func() {
				result, updated = bldr.WithClock(defaultClock).Build()
			})

			It("should mark the result as updated", func() {
				Expect(updated).To(BeTrue())
			})

			It("should return correct result", func() {
				Expect(result).To(Equal(gardencorev1beta1.Condition{
					Type:               conditionType,
					Status:             unknowStatus,
					LastTransitionTime: defaultTime,
					LastUpdateTime:     defaultTime,
					Reason:             initializedReason,
					Message:            unitializedMessage,
				}))
			})
		})

		Context("#WithStatus", func() {
			JustBeforeEach(func() {
				result, updated = bldr.
					WithClock(defaultClock).
					WithStatus(fooStatus).
					Build()
			})

			It("should mark the result as updated", func() {
				Expect(updated).To(BeTrue())
			})

			It("should return correct result", func() {
				Expect(result).To(Equal(gardencorev1beta1.Condition{
					Type:               conditionType,
					Status:             fooStatus,
					LastTransitionTime: defaultTime,
					LastUpdateTime:     defaultTime,
					Reason:             initializedReason,
					Message:            unitializedMessage,
				}))
			})
		})

		Context("#WithReason", func() {
			DescribeTable("New condition", func(reason *string, expectedReason string) {
				if reason != nil {
					bldr.WithReason(*reason)
				}

				result, updated = bldr.
					WithClock(defaultClock).
					Build()

				Expect(updated).To(BeTrue())

				Expect(result).To(Equal(gardencorev1beta1.Condition{
					Type:               conditionType,
					Status:             unknowStatus,
					LastTransitionTime: defaultTime,
					LastUpdateTime:     defaultTime,
					Reason:             expectedReason,
					Message:            unitializedMessage,
				}))
			},
				Entry("reason is not set", nil, initializedReason),
				Entry("empty reason is set", pointer.String(""), unspecifiedReason),
				Entry("reason is set", pointer.String(bazReason), bazReason),
			)

			DescribeTable("With old condition", func(reason *string, previousReason, expectedReason string) {
				lastUpdateTime := metav1.NewTime(time.Unix(11, 0))

				if reason != nil {
					bldr.WithReason(*reason)
				}

				result, updated = bldr.
					WithClock(defaultClock).
					WithOldCondition(gardencorev1beta1.Condition{
						Type:               conditionType,
						Status:             fooStatus,
						LastTransitionTime: metav1.NewTime(time.Unix(10, 0)),
						LastUpdateTime:     lastUpdateTime,
						Reason:             previousReason,
						Message:            fubarMessage,
						Codes:              codes,
					}).
					WithCodes(codes...).
					Build()

				if reason != nil && *reason != previousReason || previousReason == "" {
					Expect(updated).To(BeTrue())
					lastUpdateTime = defaultTime
				}

				Expect(result).To(Equal(gardencorev1beta1.Condition{
					Type:               conditionType,
					Status:             fooStatus,
					LastTransitionTime: metav1.NewTime(time.Unix(10, 0)),
					LastUpdateTime:     lastUpdateTime,
					Reason:             expectedReason,
					Message:            fubarMessage,
					Codes:              codes,
				}))
			},
				Entry("reason is not set", nil, bazReason, bazReason),
				Entry("reason was previously empty", nil, "", initializedReason),
				Entry("empty reason is set", pointer.String(""), bazReason, unspecifiedReason),
				Entry("message is the same", pointer.String("ReasonA"), "ReasonA", "ReasonA"),
				Entry("message changed", pointer.String("ReasonA"), bazReason, "ReasonA"),
			)
		})

		Context("#WithMessage", func() {
			DescribeTable("New condition", func(message *string, expectedMessage string) {
				if message != nil {
					bldr.WithMessage(*message)
				}

				result, updated = bldr.
					WithClock(defaultClock).
					Build()

				Expect(updated).To(BeTrue())

				Expect(result).To(Equal(gardencorev1beta1.Condition{
					Type:               conditionType,
					Status:             unknowStatus,
					LastTransitionTime: defaultTime,
					LastUpdateTime:     defaultTime,
					Reason:             initializedReason,
					Message:            expectedMessage,
				}))
			},
				Entry("message is not set", nil, unitializedMessage),
				Entry("empty message is set", pointer.String(""), unspecifiedMessage),
				Entry("message is set", pointer.String(fubarMessage), fubarMessage),
			)

			DescribeTable("With old condition", func(message *string, previousMessage, expectedMessage string) {
				lastUpdateTime := metav1.NewTime(time.Unix(11, 0))

				if message != nil {
					bldr.WithMessage(*message)
				}

				result, updated = bldr.
					WithClock(defaultClock).
					WithOldCondition(gardencorev1beta1.Condition{
						Type:               conditionType,
						Status:             fooStatus,
						LastTransitionTime: metav1.NewTime(time.Unix(10, 0)),
						LastUpdateTime:     lastUpdateTime,
						Reason:             bazReason,
						Message:            previousMessage,
						Codes:              codes,
					}).
					WithCodes(codes...).
					Build()

				if message != nil && *message != previousMessage || previousMessage == "" {
					Expect(updated).To(BeTrue())
					lastUpdateTime = defaultTime
				}

				Expect(result).To(Equal(gardencorev1beta1.Condition{
					Type:               conditionType,
					Status:             fooStatus,
					LastTransitionTime: metav1.NewTime(time.Unix(10, 0)),
					LastUpdateTime:     lastUpdateTime,
					Reason:             bazReason,
					Message:            expectedMessage,
					Codes:              codes,
				}))
			},
				Entry("message is not set", nil, fubarMessage, fubarMessage),
				Entry("message was previously empty", nil, "", unitializedMessage),
				Entry("empty message is set", pointer.String(""), fubarMessage, unspecifiedMessage),
				Entry("message is the same", pointer.String("another message"), "another message", "another message"),
				Entry("message changed", pointer.String("another message"), fubarMessage, "another message"),
			)
		})

		Context("#WithCodes", func() {
			JustBeforeEach(func() {
				result, updated = bldr.
					WithClock(defaultClock).
					WithCodes(codes...).
					Build()
			})

			It("should mark the result as updated", func() {
				Expect(updated).To(BeTrue())
			})

			It("should return correct result", func() {
				Expect(result).To(Equal(gardencorev1beta1.Condition{
					Type:               conditionType,
					Status:             unknowStatus,
					LastTransitionTime: defaultTime,
					LastUpdateTime:     defaultTime,
					Reason:             initializedReason,
					Message:            unitializedMessage,
					Codes:              codes,
				}))
			})
		})

		Context("#WithOldCondition", func() {
			JustBeforeEach(func() {
				result, updated = bldr.
					WithClock(defaultClock).
					WithOldCondition(gardencorev1beta1.Condition{
						Type:               conditionType,
						Status:             fooStatus,
						LastTransitionTime: metav1.NewTime(time.Unix(10, 0)),
						LastUpdateTime:     metav1.NewTime(time.Unix(11, 0)),
						Reason:             bazReason,
						Message:            fubarMessage,
						Codes:              codes,
					}).
					WithCodes(codes...).
					Build()
			})

			It("should mark the result as not updated", func() {
				Expect(updated).To(BeFalse())
			})

			It("should return correct result", func() {
				Expect(result).To(Equal(gardencorev1beta1.Condition{
					Type:               conditionType,
					Status:             fooStatus,
					LastTransitionTime: metav1.NewTime(time.Unix(10, 0)),
					LastUpdateTime:     metav1.NewTime(time.Unix(11, 0)),
					Reason:             bazReason,
					Message:            fubarMessage,
					Codes:              codes,
				}))
			})
		})

		Context("Clear error codes", func() {
			JustBeforeEach(func() {
				result, updated = bldr.
					WithClock(defaultClock).
					WithOldCondition(gardencorev1beta1.Condition{
						Type:               conditionType,
						Status:             fooStatus,
						LastTransitionTime: metav1.NewTime(time.Unix(10, 0)),
						LastUpdateTime:     metav1.NewTime(time.Unix(11, 0)),
						Reason:             bazReason,
						Message:            fubarMessage,
						Codes:              codes,
					}).
					Build()
			})

			It("should mark the result as updated", func() {
				Expect(updated).To(BeTrue())
			})

			It("should return correct result", func() {
				Expect(result).To(Equal(gardencorev1beta1.Condition{
					Type:               conditionType,
					Status:             fooStatus,
					LastTransitionTime: metav1.NewTime(time.Unix(10, 0)),
					LastUpdateTime:     defaultTime,
					Reason:             bazReason,
					Message:            fubarMessage,
				}))
			})
		})

		Context("Full override", func() {
			JustBeforeEach(func() {
				result, updated = bldr.
					WithClock(defaultClock).
					WithStatus("SomeNewStatus").
					WithMessage("Some message").
					WithReason("SomeNewReason").
					WithCodes(codes...).
					WithOldCondition(gardencorev1beta1.Condition{
						Type:               conditionType,
						Status:             fooStatus,
						LastTransitionTime: metav1.NewTime(time.Unix(10, 0)),
						LastUpdateTime:     metav1.NewTime(time.Unix(11, 0)),
						Reason:             bazReason,
						Message:            fubarMessage,
						Codes:              []gardencorev1beta1.ErrorCode{gardencorev1beta1.ErrorInfraQuotaExceeded},
					}).
					Build()
			})

			It("should mark the result as updated", func() {
				Expect(updated).To(BeTrue())
			})

			It("should return correct result", func() {
				Expect(result).To(Equal(gardencorev1beta1.Condition{
					Type:               conditionType,
					Status:             gardencorev1beta1.ConditionStatus("SomeNewStatus"),
					LastTransitionTime: defaultTime,
					LastUpdateTime:     defaultTime,
					Reason:             "SomeNewReason",
					Message:            "Some message",
					Codes:              codes,
				}))
			})
		})

		Context("LastTransitionTime", func() {
			It("should update last transition time when status is updated", func() {
				result, _ = bldr.
					WithClock(defaultClock).
					WithOldCondition(gardencorev1beta1.Condition{
						Type:               conditionType,
						Status:             fooStatus,
						LastTransitionTime: metav1.NewTime(time.Unix(10, 0)),
						LastUpdateTime:     metav1.NewTime(time.Unix(11, 0)),
						Reason:             bazReason,
						Message:            fubarMessage,
						Codes:              []gardencorev1beta1.ErrorCode{gardencorev1beta1.ErrorInfraQuotaExceeded},
					}).
					WithStatus("SomeNewStatus").
					Build()

				Expect(result.LastTransitionTime).To(Equal(defaultTime))
			})

			It("should not update last transition time when status is not updated", func() {
				result, _ = bldr.
					WithClock(defaultClock).
					WithOldCondition(gardencorev1beta1.Condition{
						Type:               conditionType,
						Status:             fooStatus,
						LastTransitionTime: metav1.NewTime(time.Unix(10, 0)),
						LastUpdateTime:     metav1.NewTime(time.Unix(11, 0)),
						Reason:             bazReason,
						Message:            fubarMessage,
						Codes:              []gardencorev1beta1.ErrorCode{gardencorev1beta1.ErrorInfraQuotaExceeded},
					}).
					Build()

				Expect(result.LastTransitionTime).To(Equal(metav1.NewTime(time.Unix(10, 0))))
			})
		})

		Context("LastUpdateTime", func() {
			It("should update LastUpdateTime when codes are updated", func() {
				result, _ = bldr.
					WithClock(defaultClock).
					WithOldCondition(gardencorev1beta1.Condition{
						Type:               conditionType,
						Status:             fooStatus,
						LastTransitionTime: metav1.NewTime(time.Unix(10, 0)),
						LastUpdateTime:     metav1.NewTime(time.Unix(11, 0)),
						Reason:             bazReason,
						Message:            fubarMessage,
						Codes:              []gardencorev1beta1.ErrorCode{gardencorev1beta1.ErrorInfraQuotaExceeded},
					}).
					WithCodes(codes...).
					Build()

				Expect(result.LastUpdateTime).To(Equal(defaultTime))
			})

			It("should update LastUpdateTime when message is updated", func() {
				result, _ = bldr.
					WithClock(defaultClock).
					WithOldCondition(gardencorev1beta1.Condition{
						Type:               conditionType,
						Status:             fooStatus,
						LastTransitionTime: metav1.NewTime(time.Unix(10, 0)),
						LastUpdateTime:     metav1.NewTime(time.Unix(11, 0)),
						Reason:             bazReason,
						Message:            fubarMessage,
						Codes:              []gardencorev1beta1.ErrorCode{gardencorev1beta1.ErrorInfraQuotaExceeded},
					}).
					WithMessage("Some message").
					Build()

				Expect(result.LastUpdateTime).To(Equal(defaultTime))
			})

			It("should update LastUpdateTime when reason is updated", func() {
				result, _ = bldr.
					WithClock(defaultClock).
					WithOldCondition(gardencorev1beta1.Condition{
						Type:               conditionType,
						Status:             fooStatus,
						LastTransitionTime: metav1.NewTime(time.Unix(10, 0)),
						LastUpdateTime:     metav1.NewTime(time.Unix(11, 0)),
						Reason:             bazReason,
						Message:            fubarMessage,
						Codes:              []gardencorev1beta1.ErrorCode{gardencorev1beta1.ErrorInfraQuotaExceeded},
					}).
					WithReason("SomeNewReason").
					Build()

				Expect(result.LastUpdateTime).To(Equal(defaultTime))
			})

			It("should not update LastUpdateTime when codes, message and reason are not updated", func() {
				result, _ = bldr.
					WithClock(defaultClock).
					WithOldCondition(gardencorev1beta1.Condition{
						Type:               conditionType,
						Status:             fooStatus,
						LastTransitionTime: metav1.NewTime(time.Unix(10, 0)),
						LastUpdateTime:     metav1.NewTime(time.Unix(11, 0)),
						Reason:             bazReason,
						Message:            fubarMessage,
						Codes:              []gardencorev1beta1.ErrorCode{gardencorev1beta1.ErrorInfraQuotaExceeded},
					}).
					WithCodes([]gardencorev1beta1.ErrorCode{gardencorev1beta1.ErrorInfraQuotaExceeded}...).
					Build()

				Expect(result.LastUpdateTime).To(Equal(metav1.NewTime(time.Unix(11, 0))))
			})
		})
	})
})

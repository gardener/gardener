// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	. "github.com/gardener/gardener/pkg/apis/core/v1alpha1/helper"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/onsi/gomega/types"
)

var (
	trueVar  = true
	falseVar = false
)

var _ = Describe("helper", func() {
	Describe("errors", func() {
		var zeroTime metav1.Time

		DescribeTable("#UpdatedCondition",
			func(condition gardencorev1alpha1.Condition, status gardencorev1alpha1.ConditionStatus, reason, message string, matcher types.GomegaMatcher) {
				updated := UpdatedCondition(condition, status, reason, message)

				Expect(updated).To(matcher)
			},
			Entry("no update",
				gardencorev1alpha1.Condition{
					Status:  gardencorev1alpha1.ConditionTrue,
					Reason:  "reason",
					Message: "message",
				},
				gardencorev1alpha1.ConditionTrue,
				"reason",
				"message",
				MatchFields(IgnoreExtras, Fields{
					"Status":             Equal(gardencorev1alpha1.ConditionTrue),
					"Reason":             Equal("reason"),
					"Message":            Equal("message"),
					"LastTransitionTime": Equal(zeroTime),
					"LastUpdateTime":     Not(Equal(zeroTime)),
				}),
			),
			Entry("update reason",
				gardencorev1alpha1.Condition{
					Status:  gardencorev1alpha1.ConditionTrue,
					Reason:  "reason",
					Message: "message",
				},
				gardencorev1alpha1.ConditionTrue,
				"OtherReason",
				"message",
				MatchFields(IgnoreExtras, Fields{
					"Status":             Equal(gardencorev1alpha1.ConditionTrue),
					"Reason":             Equal("OtherReason"),
					"Message":            Equal("message"),
					"LastTransitionTime": Equal(zeroTime),
					"LastUpdateTime":     Not(Equal(zeroTime)),
				}),
			),
			Entry("update status",
				gardencorev1alpha1.Condition{
					Status:  gardencorev1alpha1.ConditionTrue,
					Reason:  "reason",
					Message: "message",
				},
				gardencorev1alpha1.ConditionFalse,
				"OtherReason",
				"message",
				MatchFields(IgnoreExtras, Fields{
					"Status":             Equal(gardencorev1alpha1.ConditionFalse),
					"Reason":             Equal("OtherReason"),
					"Message":            Equal("message"),
					"LastTransitionTime": Not(Equal(zeroTime)),
					"LastUpdateTime":     Not(Equal(zeroTime)),
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

				Expect(result).To(Equal([]gardencorev1alpha1.Condition{gardencorev1alpha1.Condition{Type: typeFoo}, gardencorev1alpha1.Condition{Type: typeBar}}))
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

		DescribeTable("#IsResourceSupported",
			func(resources []gardencorev1alpha1.ControllerResource, resourceKind, resourceType string, expectation bool) {
				Expect(IsResourceSupported(resources, resourceKind, resourceType)).To(Equal(expectation))
			},
			Entry("expect true",
				[]gardencorev1alpha1.ControllerResource{
					{
						Kind: "foo",
						Type: "bar",
					},
				},
				"foo",
				"bar",
				true,
			),
			Entry("expect true",
				[]gardencorev1alpha1.ControllerResource{
					{
						Kind: "foo",
						Type: "bar",
					},
				},
				"foo",
				"BAR",
				true,
			),
			Entry("expect false",
				[]gardencorev1alpha1.ControllerResource{
					{
						Kind: "foo",
						Type: "bar",
					},
				},
				"foo",
				"baz",
				false,
			),
		)

		DescribeTable("#IsControllerInstallationSuccessful",
			func(conditions []gardencorev1alpha1.Condition, expectation bool) {
				controllerInstallation := gardencorev1alpha1.ControllerInstallation{
					Status: gardencorev1alpha1.ControllerInstallationStatus{
						Conditions: conditions,
					},
				}
				Expect(IsControllerInstallationSuccessful(controllerInstallation)).To(Equal(expectation))
			},
			Entry("expect true",
				[]gardencorev1alpha1.Condition{
					{
						Type:   gardencorev1alpha1.ControllerInstallationInstalled,
						Status: gardencorev1alpha1.ConditionTrue,
					},
				},
				true,
			),
			Entry("expect false",
				[]gardencorev1alpha1.Condition{
					{
						Type:   gardencorev1alpha1.ControllerInstallationInstalled,
						Status: gardencorev1alpha1.ConditionFalse,
					},
				},
				false,
			),
			Entry("expect false",
				[]gardencorev1alpha1.Condition{},
				false,
			),
		)
	})
})

// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	. "github.com/gardener/gardener/pkg/operation/shoot"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("Shoot Utils", func() {
	Context("Status", func() {
		DescribeTable("#OrWorse",
			func(s1, s2, expected Status) {
				Expect(s1.OrWorse(s2)).To(Equal(expected))
				Expect(s2.OrWorse(s1)).To(Equal(expected), "not reflexive")
			},
			Entry("healthy - healthy", StatusHealthy, StatusHealthy, StatusHealthy),
			Entry("healthy - progressing", StatusHealthy, StatusProgressing, StatusProgressing),
			Entry("healthy - unhealthy", StatusHealthy, StatusUnhealthy, StatusUnhealthy),
			Entry("progressing - progressing", StatusProgressing, StatusProgressing, StatusProgressing),
			Entry("progressing - unhealthy", StatusProgressing, StatusUnhealthy, StatusUnhealthy),
			Entry("unhealthy - unhealthy", StatusUnhealthy, StatusUnhealthy, StatusUnhealthy),
		)

		DescribeTable("#ConditionStatusToStatus",
			func(status gardencorev1beta1.ConditionStatus, expected Status) {
				Expect(ConditionStatusToStatus(status)).To(Equal(expected))
			},
			Entry("ConditionTrue", gardencorev1beta1.ConditionTrue, StatusHealthy),
			Entry("ConditionProgressing", gardencorev1beta1.ConditionProgressing, StatusProgressing),
			Entry("ConditionUnknown", gardencorev1beta1.ConditionUnknown, StatusUnhealthy),
			Entry("ConditionFalse", gardencorev1beta1.ConditionFalse, StatusUnhealthy),
		)

		DescribeTable("#ComputeConditionStatus",
			func(conditions []gardencorev1beta1.Condition, expected Status) {
				Expect(ComputeConditionStatus(conditions...)).To(Equal(expected))
			},
			Entry("no conditions", nil, StatusHealthy),
			Entry("true condition", []gardencorev1beta1.Condition{
				{Status: gardencorev1beta1.ConditionTrue},
			}, StatusHealthy),
			Entry("progressing condition", []gardencorev1beta1.Condition{
				{Status: gardencorev1beta1.ConditionTrue},
				{Status: gardencorev1beta1.ConditionProgressing},
			}, StatusProgressing),
			Entry("unknown condition", []gardencorev1beta1.Condition{
				{Status: gardencorev1beta1.ConditionTrue},
				{Status: gardencorev1beta1.ConditionProgressing},
				{Status: gardencorev1beta1.ConditionUnknown},
			}, StatusUnhealthy),
			Entry("false condition", []gardencorev1beta1.Condition{
				{Status: gardencorev1beta1.ConditionTrue},
				{Status: gardencorev1beta1.ConditionProgressing},
				{Status: gardencorev1beta1.ConditionUnknown},
				{Status: gardencorev1beta1.ConditionFalse},
			}, StatusUnhealthy),
		)

		DescribeTable("#BoolToStatus",
			func(b bool, expected Status) {
				Expect(BoolToStatus(b)).To(Equal(expected))
			},
			Entry("true", true, StatusHealthy),
			Entry("false", false, StatusUnhealthy),
		)

		DescribeTable("#ComputeStatus",
			func(lastOperation *gardencorev1beta1.LastOperation, lastErrors []gardencorev1beta1.LastError, conditions []gardencorev1beta1.Condition, expected Status) {
				Expect(ComputeStatus(lastOperation, lastErrors, conditions...)).To(Equal(expected))
			},
			Entry("lastOperation is nil",
				nil, nil, nil, StatusHealthy),
			Entry("lastOperation.Type is LastOperationTypeCreate",
				&gardencorev1beta1.LastOperation{Type: gardencorev1beta1.LastOperationTypeCreate}, nil, nil, StatusHealthy),
			Entry("lastOperation.Type is LastOperationTypeDelete",
				&gardencorev1beta1.LastOperation{Type: gardencorev1beta1.LastOperationTypeDelete}, nil, nil, StatusHealthy),
			Entry("lastOperation.Type is LastOperationTypeCreate and lastError is set",
				&gardencorev1beta1.LastOperation{Type: gardencorev1beta1.LastOperationTypeCreate}, []gardencorev1beta1.LastError{{}}, nil, StatusUnhealthy),
			Entry("lastOperation.Type is LastOperationTypeDelete and lastError is set",
				&gardencorev1beta1.LastOperation{Type: gardencorev1beta1.LastOperationTypeDelete}, []gardencorev1beta1.LastError{{}}, nil, StatusUnhealthy),
			Entry("lastOperation.State is LastOperationStateProcessing with healthy conditions",
				&gardencorev1beta1.LastOperation{State: gardencorev1beta1.LastOperationStateProcessing}, nil, nil, StatusHealthy),
			Entry("lastOperation.State is LastOperationStateProcessing with unhealthy conditions",
				&gardencorev1beta1.LastOperation{State: gardencorev1beta1.LastOperationStateProcessing}, nil, []gardencorev1beta1.Condition{{Status: gardencorev1beta1.ConditionFalse}}, StatusUnhealthy),
			Entry("lastOperation.State is LastOperationStateProcessing with healthy conditions but lastError set",
				&gardencorev1beta1.LastOperation{State: gardencorev1beta1.LastOperationStateProcessing}, []gardencorev1beta1.LastError{{}}, nil, StatusUnhealthy),
			Entry("lastOperation.State is neither LastOperationStateProcessing nor LastOperationStateSucceeded with healthy conditions",
				&gardencorev1beta1.LastOperation{State: gardencorev1beta1.LastOperationStateError}, nil, nil, StatusUnhealthy),
			Entry("lastOperation.State is LastOperationStateSucceeded with healthy conditions",
				&gardencorev1beta1.LastOperation{State: gardencorev1beta1.LastOperationStateSucceeded}, nil, nil, StatusHealthy),
			Entry("lastOperation.State is LastOperationStateSucceeded with unhealthy conditions",
				&gardencorev1beta1.LastOperation{State: gardencorev1beta1.LastOperationStateSucceeded}, nil, []gardencorev1beta1.Condition{{Status: gardencorev1beta1.ConditionFalse}}, StatusUnhealthy),
			Entry("lastOperation.Type is LastOperationTypeCreate and lastOperation.State is LastOperationStateSucceeded with unhealthy conditions",
				&gardencorev1beta1.LastOperation{Type: gardencorev1beta1.LastOperationTypeCreate, State: gardencorev1beta1.LastOperationStateSucceeded}, nil, []gardencorev1beta1.Condition{{Status: gardencorev1beta1.ConditionFalse}}, StatusUnhealthy),
		)
	})
})

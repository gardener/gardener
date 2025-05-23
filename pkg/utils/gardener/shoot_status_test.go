// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardener_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/utils/gardener"
)

var _ = Describe("Shoot Utils", func() {
	Context("ShootStatus", func() {
		DescribeTable("#OrWorse",
			func(s1, s2, expected ShootStatus) {
				Expect(s1.OrWorse(s2)).To(Equal(expected))
				Expect(s2.OrWorse(s1)).To(Equal(expected), "not reflexive")
			},
			Entry("healthy - healthy", ShootStatusHealthy, ShootStatusHealthy, ShootStatusHealthy),
			Entry("healthy - progressing", ShootStatusHealthy, ShootStatusProgressing, ShootStatusProgressing),
			Entry("healthy - unknown", ShootStatusHealthy, ShootStatusUnknown, ShootStatusUnknown),
			Entry("healthy - unhealthy", ShootStatusHealthy, ShootStatusUnhealthy, ShootStatusUnhealthy),
			Entry("progressing - progressing", ShootStatusProgressing, ShootStatusProgressing, ShootStatusProgressing),
			Entry("progressing - unknown", ShootStatusProgressing, ShootStatusUnknown, ShootStatusUnknown),
			Entry("progressing - unhealthy", ShootStatusProgressing, ShootStatusUnhealthy, ShootStatusUnhealthy),
			Entry("unknown - unknown", ShootStatusUnknown, ShootStatusUnknown, ShootStatusUnknown),
			Entry("unknown - unhealthy", ShootStatusUnknown, ShootStatusUnhealthy, ShootStatusUnhealthy),
			Entry("unhealthy - unhealthy", ShootStatusUnhealthy, ShootStatusUnhealthy, ShootStatusUnhealthy),
		)

		DescribeTable("#ConditionStatusToShootStatus",
			func(status gardencorev1beta1.ConditionStatus, expected ShootStatus) {
				Expect(ConditionStatusToShootStatus(status)).To(Equal(expected))
			},
			Entry("ConditionTrue", gardencorev1beta1.ConditionTrue, ShootStatusHealthy),
			Entry("ConditionProgressing", gardencorev1beta1.ConditionProgressing, ShootStatusProgressing),
			Entry("ConditionUnknown", gardencorev1beta1.ConditionUnknown, ShootStatusUnknown),
			Entry("ConditionFalse", gardencorev1beta1.ConditionFalse, ShootStatusUnhealthy),
		)

		DescribeTable("#ComputeConditionStatus",
			func(conditions []gardencorev1beta1.Condition, expected ShootStatus) {
				Expect(ComputeConditionStatus(conditions...)).To(Equal(expected))
			},
			Entry("no conditions", nil, ShootStatusHealthy),
			Entry("true condition", []gardencorev1beta1.Condition{
				{Status: gardencorev1beta1.ConditionTrue},
			}, ShootStatusHealthy),
			Entry("progressing condition", []gardencorev1beta1.Condition{
				{Status: gardencorev1beta1.ConditionTrue},
				{Status: gardencorev1beta1.ConditionProgressing},
			}, ShootStatusProgressing),
			Entry("unknown condition", []gardencorev1beta1.Condition{
				{Status: gardencorev1beta1.ConditionTrue},
				{Status: gardencorev1beta1.ConditionProgressing},
				{Status: gardencorev1beta1.ConditionUnknown},
			}, ShootStatusUnknown),
			Entry("false condition", []gardencorev1beta1.Condition{
				{Status: gardencorev1beta1.ConditionTrue},
				{Status: gardencorev1beta1.ConditionProgressing},
				{Status: gardencorev1beta1.ConditionUnknown},
				{Status: gardencorev1beta1.ConditionFalse},
			}, ShootStatusUnhealthy),
		)

		DescribeTable("#BoolToShootStatus",
			func(b bool, expected ShootStatus) {
				Expect(BoolToShootStatus(b)).To(Equal(expected))
			},
			Entry("true", true, ShootStatusHealthy),
			Entry("false", false, ShootStatusUnhealthy),
		)

		DescribeTable("#ComputeShootStatus",
			func(lastOperation *gardencorev1beta1.LastOperation, lastErrors []gardencorev1beta1.LastError, conditions []gardencorev1beta1.Condition, expected ShootStatus) {
				Expect(ComputeShootStatus(lastOperation, lastErrors, conditions...)).To(Equal(expected))
			},
			Entry("lastOperation is nil",
				nil, nil, nil, ShootStatusHealthy),
			Entry("lastOperation.Type is LastOperationTypeCreate",
				&gardencorev1beta1.LastOperation{Type: gardencorev1beta1.LastOperationTypeCreate}, nil, nil, ShootStatusHealthy),
			Entry("lastOperation.Type is LastOperationTypeDelete",
				&gardencorev1beta1.LastOperation{Type: gardencorev1beta1.LastOperationTypeDelete}, nil, nil, ShootStatusHealthy),
			Entry("lastOperation.Type is LastOperationTypeCreate and lastError is set",
				&gardencorev1beta1.LastOperation{Type: gardencorev1beta1.LastOperationTypeCreate}, []gardencorev1beta1.LastError{{}}, nil, ShootStatusUnhealthy),
			Entry("lastOperation.Type is LastOperationTypeDelete and lastError is set",
				&gardencorev1beta1.LastOperation{Type: gardencorev1beta1.LastOperationTypeDelete}, []gardencorev1beta1.LastError{{}}, nil, ShootStatusUnhealthy),
			Entry("lastOperation.State is LastOperationStateProcessing with healthy conditions",
				&gardencorev1beta1.LastOperation{State: gardencorev1beta1.LastOperationStateProcessing}, nil, nil, ShootStatusHealthy),
			Entry("lastOperation.State is LastOperationStateProcessing with unhealthy conditions",
				&gardencorev1beta1.LastOperation{State: gardencorev1beta1.LastOperationStateProcessing}, nil, []gardencorev1beta1.Condition{{Status: gardencorev1beta1.ConditionFalse}}, ShootStatusUnhealthy),
			Entry("lastOperation.State is LastOperationStateProcessing with healthy conditions but lastError set",
				&gardencorev1beta1.LastOperation{State: gardencorev1beta1.LastOperationStateProcessing}, []gardencorev1beta1.LastError{{}}, nil, ShootStatusUnhealthy),
			Entry("lastOperation.State is neither LastOperationStateProcessing nor LastOperationStateSucceeded with healthy conditions",
				&gardencorev1beta1.LastOperation{State: gardencorev1beta1.LastOperationStateError}, nil, nil, ShootStatusUnhealthy),
			Entry("lastOperation.State is LastOperationStateSucceeded with healthy conditions",
				&gardencorev1beta1.LastOperation{State: gardencorev1beta1.LastOperationStateSucceeded}, nil, nil, ShootStatusHealthy),
			Entry("lastOperation.State is LastOperationStateSucceeded with unknown conditions",
				&gardencorev1beta1.LastOperation{State: gardencorev1beta1.LastOperationStateSucceeded}, nil, []gardencorev1beta1.Condition{{Status: gardencorev1beta1.ConditionUnknown}}, ShootStatusUnknown),
			Entry("lastOperation.State is LastOperationStateSucceeded with unhealthy conditions",
				&gardencorev1beta1.LastOperation{State: gardencorev1beta1.LastOperationStateSucceeded}, nil, []gardencorev1beta1.Condition{{Status: gardencorev1beta1.ConditionFalse}}, ShootStatusUnhealthy),
			Entry("lastOperation.Type is LastOperationTypeCreate and lastOperation.State is LastOperationStateSucceeded with unhealthy conditions",
				&gardencorev1beta1.LastOperation{Type: gardencorev1beta1.LastOperationTypeCreate, State: gardencorev1beta1.LastOperationStateSucceeded}, nil, []gardencorev1beta1.Condition{{Status: gardencorev1beta1.ConditionFalse}}, ShootStatusUnhealthy),
		)
	})
})

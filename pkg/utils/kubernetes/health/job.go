// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package health

import (
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
)

// CheckJob checks whether the given Job is healthy.
// A Job is considered healthy if its `JobFailed` condition is missing or has status `False`.
func CheckJob(job *batchv1.Job) error {
	condition := getJobCondition(job.Status.Conditions, batchv1.JobFailed)
	if condition == nil {
		return nil
	}
	return checkConditionState(string(condition.Type), string(corev1.ConditionFalse), string(condition.Status), condition.Reason, condition.Message)
}

func getJobCondition(conditions []batchv1.JobCondition, conditionType batchv1.JobConditionType) *batchv1.JobCondition {
	for _, condition := range conditions {
		if condition.Type == conditionType {
			return &condition
		}
	}
	return nil
}

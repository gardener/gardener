// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package health

import (
	pvcautoscalingv1alpha1 "github.com/gardener/pvc-autoscaler/api/autoscaling/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CheckPersistentVolumeClaimAutoscaler checks whether the given PersistentVolumeClaimAutoscaler is healthy.
func CheckPersistentVolumeClaimAutoscaler(pvca *pvcautoscalingv1alpha1.PersistentVolumeClaimAutoscaler) error {
	for _, condition := range pvca.Status.Conditions {
		switch condition.Type {
		case string(pvcautoscalingv1alpha1.ConditionTypeResizing):
			if err := checkConditionState(condition.Type, string(metav1.ConditionTrue), string(condition.Status), condition.Reason, condition.Message); err != nil {
				return err
			}
		}
	}

	return nil
}

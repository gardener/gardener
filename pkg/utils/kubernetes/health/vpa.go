// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package health

import (
	corev1 "k8s.io/api/core/v1"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
)

// CheckVerticalPodAutoscaler checks whether the given VPA is healthy.
func CheckVerticalPodAutoscaler(vpa *vpaautoscalingv1.VerticalPodAutoscaler) error {
	// Checking that the VPA's observed generation is up-to-date would deadlock Garden resource
	// reconciliation: extensions are deployed first and bring their own VPAs, but those VPAs cannot
	// reach an up-to-date status until the VPA components (deployed later as part of the Garden) are
	// running. The Garden reconciliation would then block waiting for the extensions to become
	// healthy, which in turn wait for the VPA components.
	//
	// TODO(ialidzhikov): Investigate how to enable the ObservedGeneration check.
	//
	// observedGeneration := ptr.Deref(vpa.Status.ObservedGeneration, 0)
	// if observedGeneration < vpa.Generation {
	// 	return fmt.Errorf("observed generation outdated (%d/%d)", observedGeneration, vpa.Generation)
	// }

	for _, condition := range vpa.Status.Conditions {
		if condition.Type == vpaautoscalingv1.ConfigUnsupported {
			if err := checkConditionState(string(condition.Type), string(corev1.ConditionFalse), string(condition.Status), condition.Reason, condition.Message); err != nil {
				return err
			}

			break
		}
	}

	return nil
}

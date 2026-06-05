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
	// TODO(ialidzhikov): Uncomment the section below, once https://github.com/gardener/gardener/issues/14734 is resolved and `k8s.io/autoscaler/vertical-pod-autoscaler` is updated to v1.7.0.
	//
	// if vpa.Status.ObservedGeneration < vpa.Generation {
	// 	return fmt.Errorf("observed generation outdated (%d/%d)", vpa.Status.ObservedGeneration, vpa.Generation)
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

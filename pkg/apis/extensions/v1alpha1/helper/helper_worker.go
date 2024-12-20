// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

// ClusterAutoscalerRequired returns whether the given worker pool configuration indicates that a cluster-autoscaler
// is needed.
func ClusterAutoscalerRequired(pools []extensionsv1alpha1.WorkerPool) bool {
	for _, pool := range pools {
		if pool.Maximum > pool.Minimum {
			return true
		}
	}
	return false
}

// GetMachineDeploymentClusterAutoscalerAnnotations returns a map of annotations with values intended to be used as cluster-autoscaler options for the worker group
func GetMachineDeploymentClusterAutoscalerAnnotations(caOptions *extensionsv1alpha1.ClusterAutoscalerOptions) map[string]string {
	var annotations map[string]string
	if caOptions != nil {
		annotations = map[string]string{}
		if caOptions.ScaleDownUtilizationThreshold != nil {
			annotations[extensionsv1alpha1.ScaleDownUtilizationThresholdAnnotation] = *caOptions.ScaleDownUtilizationThreshold
		}
		if caOptions.ScaleDownGpuUtilizationThreshold != nil {
			annotations[extensionsv1alpha1.ScaleDownGpuUtilizationThresholdAnnotation] = *caOptions.ScaleDownGpuUtilizationThreshold
		}
		if caOptions.ScaleDownUnneededTime != nil {
			annotations[extensionsv1alpha1.ScaleDownUnneededTimeAnnotation] = caOptions.ScaleDownUnneededTime.Duration.String()
		}
		if caOptions.ScaleDownUnreadyTime != nil {
			annotations[extensionsv1alpha1.ScaleDownUnreadyTimeAnnotation] = caOptions.ScaleDownUnreadyTime.Duration.String()
		}
		if caOptions.MaxNodeProvisionTime != nil {
			annotations[extensionsv1alpha1.MaxNodeProvisionTimeAnnotation] = caOptions.MaxNodeProvisionTime.Duration.String()
		}
	}

	return annotations
}

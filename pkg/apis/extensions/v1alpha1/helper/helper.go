// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper

import extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"

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

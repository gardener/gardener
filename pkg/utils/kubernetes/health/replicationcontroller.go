// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package health

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
)

// CheckReplicationController checks whether the given ReplicationController is healthy.
// A ReplicationController is considered healthy if the controller observed its current revision and
// if the number of ready replicas is equal to the number of replicas.
func CheckReplicationController(rc *corev1.ReplicationController) error {
	if rc.Status.ObservedGeneration < rc.Generation {
		return fmt.Errorf("observed generation outdated (%d/%d)", rc.Status.ObservedGeneration, rc.Generation)
	}

	replicas := rc.Spec.Replicas
	if replicas != nil && rc.Status.ReadyReplicas < *replicas {
		return fmt.Errorf("ReplicationController does not have minimum availability")
	}

	return nil
}

// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package health

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
)

// CheckReplicaSet checks whether the given ReplicaSet is healthy.
// A ReplicaSet is considered healthy if the controller observed its current revision and
// if the number of ready replicas is equal to the number of replicas.
func CheckReplicaSet(rs *appsv1.ReplicaSet) error {
	if rs.Status.ObservedGeneration < rs.Generation {
		return fmt.Errorf("observed generation outdated (%d/%d)", rs.Status.ObservedGeneration, rs.Generation)
	}

	replicas := rs.Spec.Replicas
	if replicas != nil && rs.Status.ReadyReplicas < *replicas {
		return fmt.Errorf("ReplicaSet does not have minimum availability")
	}

	return nil
}

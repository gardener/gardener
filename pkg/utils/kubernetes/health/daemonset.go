// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package health

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func daemonSetMaxUnavailable(daemonSet *appsv1.DaemonSet) int {
	if daemonSet.Status.DesiredNumberScheduled == 0 || daemonSet.Spec.UpdateStrategy.Type != appsv1.RollingUpdateDaemonSetStrategyType {
		return 0
	}

	rollingUpdate := daemonSet.Spec.UpdateStrategy.RollingUpdate
	if rollingUpdate == nil {
		return 0
	}

	maxUnavailable, err := intstr.GetScaledValueFromIntOrPercent(rollingUpdate.MaxUnavailable, int(daemonSet.Status.DesiredNumberScheduled), false)
	if err != nil {
		return 0
	}

	return maxUnavailable
}

// CheckDaemonSet checks whether the given DaemonSet is healthy.
// A DaemonSet is considered healthy if its controller observed its current revision and if
// its desired number of scheduled pods is equal to its updated number of scheduled pods.
func CheckDaemonSet(daemonSet *appsv1.DaemonSet) error {
	if daemonSet.Status.ObservedGeneration < daemonSet.Generation {
		return fmt.Errorf("observed generation outdated (%d/%d)", daemonSet.Status.ObservedGeneration, daemonSet.Generation)
	}

	if daemonSet.Status.CurrentNumberScheduled < daemonSet.Status.DesiredNumberScheduled {
		return fmt.Errorf("not enough scheduled pods (%d/%d)", daemonSet.Status.CurrentNumberScheduled, daemonSet.Status.DesiredNumberScheduled)
	}

	if daemonSet.Status.NumberMisscheduled > 0 {
		return fmt.Errorf("misscheduled pods found (%d)", daemonSet.Status.NumberMisscheduled)
	}

	// Check if DaemonSet rollout is ongoing.
	if daemonSet.Status.UpdatedNumberScheduled < daemonSet.Status.DesiredNumberScheduled {
		if maxUnavailable := daemonSetMaxUnavailable(daemonSet); int(daemonSet.Status.NumberUnavailable) > maxUnavailable {
			return fmt.Errorf("too many unavailable pods found (%d/%d, only max. %d unavailable pods allowed)", daemonSet.Status.NumberUnavailable, daemonSet.Status.CurrentNumberScheduled, maxUnavailable)
		}
	} else {
		if daemonSet.Status.NumberUnavailable > 0 {
			return fmt.Errorf("too many unavailable pods found (%d/%d)", daemonSet.Status.NumberUnavailable, daemonSet.Status.CurrentNumberScheduled)
		}
	}

	return nil
}

// IsDaemonSetProgressing returns false if the DaemonSet has been fully rolled out. Otherwise, it returns true along
// with a reason, why the DaemonSet is not considered to be fully rolled out.
func IsDaemonSetProgressing(daemonSet *appsv1.DaemonSet) (bool, string) {
	if daemonSet.Status.ObservedGeneration < daemonSet.Generation {
		return true, fmt.Sprintf("observed generation outdated (%d/%d)", daemonSet.Status.ObservedGeneration, daemonSet.Generation)
	}

	desiredReplicas := daemonSet.Status.DesiredNumberScheduled
	updatedReplicas := daemonSet.Status.UpdatedNumberScheduled
	if updatedReplicas < desiredReplicas {
		return true, fmt.Sprintf("%d of %d replica(s) have been updated", updatedReplicas, desiredReplicas)
	}

	return false, "DaemonSet is fully rolled out"
}

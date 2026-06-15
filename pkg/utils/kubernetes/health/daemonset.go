// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package health

import (
	"context"
	"fmt"
	"slices"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/api/indexer"
)

// daemonSetMaxUnavailable returns the maximum number of unavailable pods allowed for a DaemonSet.
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

// CheckDaemonSetWithPreservedNodes re-evaluates a failing DaemonSet health check by subtracting
// unavailable pods on preserved unhealthy nodes.
// Returns true if the failure is suppressed (attributable entirely to preserved nodes), false otherwise.
// If NumberAvailable == 0, the failure is never suppressed regardless of preserved nodes.
func CheckDaemonSetWithPreservedNodes(ctx context.Context, c client.Client, ds *appsv1.DaemonSet, preservedNodeNames sets.Set[string]) (bool, error) {
	// Fast path: already healthy.
	if err := CheckDaemonSet(ds); err == nil {
		return false, nil
	}
	// Zero availability: real outage, always surface.
	if ds.Status.NumberAvailable == 0 {
		return false, nil
	}
	if preservedNodeNames.Len() == 0 {
		return false, nil
	}
	// Count not-ready pods on preserved unhealthy nodes using the PodNodeName field index.
	selector, err := metav1.LabelSelectorAsSelector(ds.Spec.Selector)
	if err != nil {
		return false, err
	}
	var preservedUnavailable int
	for nodeName := range preservedNodeNames {
		podList := &corev1.PodList{}
		if err := c.List(ctx, podList,
			client.InNamespace(ds.Namespace),
			client.MatchingFields{indexer.PodNodeName: nodeName},
			client.MatchingLabelsSelector{Selector: selector},
		); err != nil {
			return false, err
		}
		for _, pod := range podList.Items {
			isReady := slices.ContainsFunc(pod.Status.Conditions, func(c corev1.PodCondition) bool {
				return c.Type == corev1.PodReady && c.Status == corev1.ConditionTrue
			})
			if !isReady {
				preservedUnavailable++
			}
		}
	}
	nonPreservedUnavailable := int(ds.Status.NumberUnavailable) - preservedUnavailable
	maxUnavailable := daemonSetMaxUnavailable(ds)
	if ds.Status.UpdatedNumberScheduled < ds.Status.DesiredNumberScheduled {
		// During rollout: tolerance is maxUnavailable.
		if nonPreservedUnavailable > maxUnavailable {
			return false, nil
		}
	} else {
		// Fully rolled out: tolerance is 0.
		if nonPreservedUnavailable > 0 {
			return false, nil
		}
	}
	return true, nil
}

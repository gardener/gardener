// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

// Copied from https://github.com/kubernetes/kubernetes/blob/a93f803f8e400f1d42dc812bc51932ff3b31798a/pkg/api/pod/util.go#L181-L211

package health

import (
	"fmt"
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

// IsPodReady returns true if a pod is ready; false otherwise.
func IsPodReady(pod *corev1.Pod) bool {
	return IsPodReadyConditionTrue(pod.Status)
}

// IsPodReadyConditionTrue returns true if a pod is ready; false otherwise.
func IsPodReadyConditionTrue(status corev1.PodStatus) bool {
	condition := GetPodReadyCondition(status)
	return condition != nil && condition.Status == corev1.ConditionTrue
}

// GetPodReadyCondition extracts the pod ready condition from the given status and returns that.
// Returns nil if the condition is not present.
func GetPodReadyCondition(status corev1.PodStatus) *corev1.PodCondition {
	_, condition := GetPodCondition(&status, corev1.PodReady)
	return condition
}

// GetPodCondition extracts the provided condition from the given status and returns that.
// Returns nil and -1 if the condition is not present, and the index of the located condition.
func GetPodCondition(status *corev1.PodStatus, conditionType corev1.PodConditionType) (int, *corev1.PodCondition) {
	if status == nil {
		return -1, nil
	}

	for i := range status.Conditions {
		if status.Conditions[i].Type == conditionType {
			return i, &status.Conditions[i]
		}
	}
	return -1, nil
}

var healthyPodPhases = []corev1.PodPhase{corev1.PodRunning, corev1.PodSucceeded}

// CheckPod checks whether the given Pod is healthy.
// A Pod is considered healthy if its `.status.phase` is `Running` or `Succeeded`.
func CheckPod(pod *corev1.Pod) error {
	for _, healthyPhase := range healthyPodPhases {
		if pod.Status.Phase == healthyPhase {
			return nil
		}
	}

	return fmt.Errorf("pod is in invalid phase %q (expected one of %q)", pod.Status.Phase, healthyPodPhases)
}

// IsPodStale returns true when the pod reason indicates staleness.
func IsPodStale(reason string) bool {
	return strings.Contains(reason, "Evicted") ||
		strings.HasPrefix(reason, "OutOf") ||
		strings.Contains(reason, "NodeAffinity") ||
		strings.Contains(reason, "NodeLost")
}

// IsPodCompleted returns true when the pod ready condition indicates completeness.
func IsPodCompleted(conditions []corev1.PodCondition) bool {
	return slices.ContainsFunc(conditions, func(condition corev1.PodCondition) bool {
		return condition.Type == corev1.PodReady && condition.Status == corev1.ConditionFalse && condition.Reason == "PodCompleted"
	})
}

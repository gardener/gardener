/*
Copyright 2015 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Modifications Copyright 2024 SAP SE or an SAP affiliate company and Gardener contributors

// This file contains several helper functions copied from the DaemonSet controller for determining which Nodes should
// run daemon pods.
// All code in this file is copied instead of vendored because it is located in unexported parts of the Kubernetes
// codebase.
// The logic for determining which Nodes should run daemon pods hasn't changed for quite some time. Hence, we assume
// that it is consistent across Kubernetes versions.

package helper

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	v1helper "k8s.io/component-helpers/scheduling/corev1"
	"k8s.io/component-helpers/scheduling/corev1/nodeaffinity"
)

// NodeShouldRunDaemonPod checks a set of preconditions against a (node,daemonset) and returns a
// summary. Returned booleans are:
//   - shouldRun:
//     Returns true when a daemonset should run on the node if a daemonset pod is not already
//     running on that node.
//   - shouldContinueRunning:
//     Returns true when a daemonset should continue running on a node if a daemonset pod is already
//     running on that node.
//
// Copied from https://github.com/kubernetes/kubernetes/blob/v1.32.0/pkg/controller/daemon/daemon_controller.go#L1275-L1306
func NodeShouldRunDaemonPod(node *corev1.Node, ds *appsv1.DaemonSet) (bool, bool) {
	pod := NewPod(ds, node.Name)

	// If the daemon set specifies a node name, check that it matches with node.Name.
	if ds.Spec.Template.Spec.NodeName != "" && ds.Spec.Template.Spec.NodeName != node.Name {
		return false, false
	}

	taints := node.Spec.Taints
	fitsNodeName, fitsNodeAffinity, fitsTaints := predicates(pod, node, taints)
	if !fitsNodeName || !fitsNodeAffinity {
		return false, false
	}

	if !fitsTaints {
		// Scheduled daemon pods should continue running if they tolerate NoExecute taint.
		_, hasUntoleratedTaint := v1helper.FindMatchingUntoleratedTaint(taints, pod.Spec.Tolerations, func(t *corev1.Taint) bool {
			return t.Effect == corev1.TaintEffectNoExecute
		})
		return false, !hasUntoleratedTaint
	}

	return true, true
}

// predicates checks if a DaemonSet's pod can run on a node.
// Copied from https://github.com/kubernetes/kubernetes/blob/v1.32.0/pkg/controller/daemon/daemon_controller.go#L1308-L1318
func predicates(pod *corev1.Pod, node *corev1.Node, taints []corev1.Taint) (fitsNodeName, fitsNodeAffinity, fitsTaints bool) {
	fitsNodeName = len(pod.Spec.NodeName) == 0 || pod.Spec.NodeName == node.Name
	// Ignore parsing errors for backwards compatibility.
	fitsNodeAffinity, _ = nodeaffinity.GetRequiredNodeAffinity(pod).Match(node)
	_, hasUntoleratedTaint := v1helper.FindMatchingUntoleratedTaint(taints, pod.Spec.Tolerations, func(t *corev1.Taint) bool {
		return t.Effect == corev1.TaintEffectNoExecute || t.Effect == corev1.TaintEffectNoSchedule
	})
	fitsTaints = !hasUntoleratedTaint
	return
}

// NewPod creates a new pod
// Copied from https://github.com/kubernetes/kubernetes/blob/v1.32.0/pkg/controller/daemon/daemon_controller.go#L1320-L1330
func NewPod(ds *appsv1.DaemonSet, nodeName string) *corev1.Pod {
	newPod := &corev1.Pod{Spec: ds.Spec.Template.Spec, ObjectMeta: ds.Spec.Template.ObjectMeta}
	newPod.Namespace = ds.Namespace
	newPod.Spec.NodeName = nodeName

	// Added default tolerations for DaemonSet pods.
	AddOrUpdateDaemonPodTolerations(&newPod.Spec)

	return newPod
}

// AddOrUpdateDaemonPodTolerations apply necessary tolerations to DaemonSet Pods, e.g. node.kubernetes.io/not-ready:NoExecute.
// Copied from https://github.com/kubernetes/kubernetes/blob/v1.32.0/pkg/controller/daemon/util/daemonset_util.go#L47-L102
func AddOrUpdateDaemonPodTolerations(spec *corev1.PodSpec) {
	// DaemonSet pods shouldn't be deleted by NodeController in case of node problems.
	// Add infinite toleration for taint notReady:NoExecute here
	// to survive taint-based eviction enforced by NodeController
	// when node turns not ready.
	AddOrUpdateTolerationInPodSpec(spec, &corev1.Toleration{
		Key:      corev1.TaintNodeNotReady,
		Operator: corev1.TolerationOpExists,
		Effect:   corev1.TaintEffectNoExecute,
	})

	// DaemonSet pods shouldn't be deleted by NodeController in case of node problems.
	// Add infinite toleration for taint unreachable:NoExecute here
	// to survive taint-based eviction enforced by NodeController
	// when node turns unreachable.
	AddOrUpdateTolerationInPodSpec(spec, &corev1.Toleration{
		Key:      corev1.TaintNodeUnreachable,
		Operator: corev1.TolerationOpExists,
		Effect:   corev1.TaintEffectNoExecute,
	})

	// According to TaintNodesByCondition feature, all DaemonSet pods should tolerate
	// MemoryPressure, DiskPressure, PIDPressure, Unschedulable and NetworkUnavailable taints.
	AddOrUpdateTolerationInPodSpec(spec, &corev1.Toleration{
		Key:      corev1.TaintNodeDiskPressure,
		Operator: corev1.TolerationOpExists,
		Effect:   corev1.TaintEffectNoSchedule,
	})

	AddOrUpdateTolerationInPodSpec(spec, &corev1.Toleration{
		Key:      corev1.TaintNodeMemoryPressure,
		Operator: corev1.TolerationOpExists,
		Effect:   corev1.TaintEffectNoSchedule,
	})

	AddOrUpdateTolerationInPodSpec(spec, &corev1.Toleration{
		Key:      corev1.TaintNodePIDPressure,
		Operator: corev1.TolerationOpExists,
		Effect:   corev1.TaintEffectNoSchedule,
	})

	AddOrUpdateTolerationInPodSpec(spec, &corev1.Toleration{
		Key:      corev1.TaintNodeUnschedulable,
		Operator: corev1.TolerationOpExists,
		Effect:   corev1.TaintEffectNoSchedule,
	})

	if spec.HostNetwork {
		AddOrUpdateTolerationInPodSpec(spec, &corev1.Toleration{
			Key:      corev1.TaintNodeNetworkUnavailable,
			Operator: corev1.TolerationOpExists,
			Effect:   corev1.TaintEffectNoSchedule,
		})
	}
}

// AddOrUpdateTolerationInPodSpec tries to add a toleration to the toleration list in PodSpec.
// Returns true if something was updated, false otherwise.
// Copied from https://github.com/kubernetes/kubernetes/blob/v1.32.0/pkg/apis/core/v1/helper/helpers.go#L261-L287
func AddOrUpdateTolerationInPodSpec(spec *corev1.PodSpec, toleration *corev1.Toleration) bool {
	podTolerations := spec.Tolerations

	var newTolerations []corev1.Toleration
	updated := false
	for i := range podTolerations {
		if toleration.MatchToleration(&podTolerations[i]) {
			if Semantic.DeepEqual(toleration, podTolerations[i]) {
				return false
			}
			newTolerations = append(newTolerations, *toleration)
			updated = true
			continue
		}

		newTolerations = append(newTolerations, podTolerations[i])
	}

	if !updated {
		newTolerations = append(newTolerations, *toleration)
	}

	spec.Tolerations = newTolerations
	return true
}

// Semantic can do semantic deep equality checks for core objects.
// Example: apiequality.Semantic.DeepEqual(aPod, aPodWithNonNilButEmptyMaps) == true
// Copied from https://github.com/kubernetes/kubernetes/blob/v1.32.0/pkg/apis/core/helper/helpers.go#L92-L114
var Semantic = conversion.EqualitiesOrDie(
	func(a, b resource.Quantity) bool {
		// Ignore formatting, only care that numeric value stayed the same.
		// TODO: if we decide it's important, it should be safe to start comparing the format.
		//
		// Uninitialized quantities are equivalent to 0 quantities.
		return a.Cmp(b) == 0
	},
	func(a, b metav1.MicroTime) bool {
		return a.UTC() == b.UTC()
	},
	func(a, b metav1.Time) bool {
		return a.UTC() == b.UTC()
	},
	func(a, b labels.Selector) bool {
		return a.String() == b.String()
	},
	func(a, b fields.Selector) bool {
		return a.String() == b.String()
	},
)

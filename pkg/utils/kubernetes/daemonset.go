// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubernetes

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

// PodManagedByDaemonSet returns 'true' if the given pod is managed by a DaemonSet, determined by the existing owner references.
func PodManagedByDaemonSet(pod *corev1.Pod) bool {
	for _, ownerRef := range pod.OwnerReferences {
		if ownerRef.APIVersion == appsv1.SchemeGroupVersion.String() &&
			ownerRef.Kind == "DaemonSet" &&
			ptr.Deref(ownerRef.Controller, false) {
			return true
		}
	}

	return false
}

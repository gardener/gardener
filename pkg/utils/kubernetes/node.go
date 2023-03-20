// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kubernetes

import (
	"strings"

	corev1 "k8s.io/api/core/v1"
	kubeletapis "k8s.io/kubelet/pkg/apis"
)

// IsNodeLabelAllowedForKubelet determines whether kubelet is allowed by the NodeRestriction admission plugin to set a
// label on its own Node object with the given key.
// See https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/#noderestriction.
func IsNodeLabelAllowedForKubelet(key string) bool {
	namespace := getLabelNamespace(key)

	// kubelets are forbidden to set node-restriction labels
	if namespace == corev1.LabelNamespaceNodeRestriction || strings.HasSuffix(namespace, "."+corev1.LabelNamespaceNodeRestriction) {
		return false
	}

	// kubelets are forbidden to set unknown kubernetes.io and k8s.io labels
	if isKubernetesLabelNamespace(namespace) && !kubeletapis.IsKubeletLabel(key) {
		return false
	}

	return true
}

// same logic as in kube-apiserver and kubelet code
func getLabelNamespace(key string) string {
	if parts := strings.SplitN(key, "/", 2); len(parts) == 2 {
		return parts[0]
	}
	return ""
}

func isKubernetesLabelNamespace(namespace string) bool {
	if namespace == "kubernetes.io" || strings.HasSuffix(namespace, ".kubernetes.io") {
		return true
	}
	if namespace == "k8s.io" || strings.HasSuffix(namespace, ".k8s.io") {
		return true
	}
	return false
}

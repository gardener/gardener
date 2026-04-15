// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubernetes

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeletapis "k8s.io/kubelet/pkg/apis"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// HasMoreThanOneNode returns true if the cluster has more than one node. It uses a metadata-only list with limit 2 to
// minimize the amount of data transferred.
func HasMoreThanOneNode(ctx context.Context, reader client.Reader) (bool, error) {
	nodeList := &metav1.PartialObjectMetadataList{}
	nodeList.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("NodeList"))
	if err := reader.List(ctx, nodeList, client.Limit(2)); err != nil {
		return false, fmt.Errorf("failed to list nodes: %w", err)
	}
	return len(nodeList.Items) > 1, nil
}

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

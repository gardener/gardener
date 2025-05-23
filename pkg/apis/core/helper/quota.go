// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"errors"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// QuotaScope returns the scope of a quota scope reference.
func QuotaScope(scopeRef corev1.ObjectReference) (string, error) {
	if gvk := schema.FromAPIVersionAndKind(scopeRef.APIVersion, scopeRef.Kind); gvk.Group == "core.gardener.cloud" && gvk.Kind == "Project" {
		return "project", nil
	}
	if scopeRef.APIVersion == "v1" && scopeRef.Kind == "Secret" {
		return "credentials", nil
	}
	if gvk := schema.FromAPIVersionAndKind(scopeRef.APIVersion, scopeRef.Kind); gvk.Group == "security.gardener.cloud" && gvk.Kind == "WorkloadIdentity" {
		return "credentials", nil
	}
	return "", errors.New("unknown quota scope")
}

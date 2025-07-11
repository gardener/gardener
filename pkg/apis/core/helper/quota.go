// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"errors"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	// ProjectGVK is the GroupVersionKind for Gardener Project resources.
	ProjectGVK = schema.GroupVersionKind{Group: "core.gardener.cloud", Version: "v1beta1", Kind: "Project"}
	// SecretGVK is the GroupVersionKind for Kubernetes Secret resources.
	SecretGVK = schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Secret"}
	// WorkloadIdentityGVK is the GroupVersionKind for Gardener WorkloadIdentity resources.
	WorkloadIdentityGVK = schema.GroupVersionKind{Group: "security.gardener.cloud", Version: "v1alpha1", Kind: "WorkloadIdentity"}
)

// QuotaScope returns the scope of a quota scope reference.
func QuotaScope(scopeRef corev1.ObjectReference) (string, error) {
	switch schema.FromAPIVersionAndKind(scopeRef.APIVersion, scopeRef.Kind) {
	case ProjectGVK:
		return "project", nil
	case SecretGVK:
		return "credentials", nil
	case WorkloadIdentityGVK:
		return "credentials", nil
	default:
		return "", errors.New("unknown quota scope")
	}
}

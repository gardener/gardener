// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
)

// ResourceReferencesEqual returns true when none of the Secret/ConfigMap/WorkloadIdentity resource references have changed.
func ResourceReferencesEqual(oldResources, newResources []gardencorev1.NamedResourceReference) bool {
	oldNames := namesForResourceReferences(oldResources)
	newNames := namesForResourceReferences(newResources)

	return oldNames.Equal(newNames)
}

func namesForResourceReferences(resources []gardencorev1.NamedResourceReference) sets.Set[string] {
	names := sets.New[string]()
	for _, resource := range resources {
		if resource.ResourceRef.APIVersion == corev1.SchemeGroupVersion.String() && sets.New("Secret", "ConfigMap").Has(resource.ResourceRef.Kind) ||
			resource.ResourceRef.APIVersion == securityv1alpha1.SchemeGroupVersion.String() && resource.ResourceRef.Kind == "WorkloadIdentity" {
			names.Insert(resource.ResourceRef.Kind + "/" + resource.ResourceRef.Name)
		}
	}
	return names
}

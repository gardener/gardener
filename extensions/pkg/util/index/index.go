// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package index

import (
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

// SecretRefNamespaceField is the field name for the index function that extracts the corresponding field from SecretBinding.
const SecretRefNamespaceField string = "secretRef.namespace"

// SecretRefNamespaceIndexerFunc extracts the secretRef.namespace field of a SecretBinding.
func SecretRefNamespaceIndexerFunc(rawObj client.Object) []string {
	secretBinding, ok := rawObj.(*gardencorev1beta1.SecretBinding)
	if !ok {
		return []string{}
	}
	return []string{secretBinding.SecretRef.Namespace}
}

// SecretBindingNameField is the field name for the index function that extracts the corresponding field from Shoot.
const SecretBindingNameField string = "spec.secretBindingName"

// SecretBindingNameIndexerFunc extracts the spec.secretBindingName field of a Shoot.
func SecretBindingNameIndexerFunc(rawObj client.Object) []string {
	shoot, ok := rawObj.(*gardencorev1beta1.Shoot)
	if !ok {
		return []string{}
	}
	if shoot.Spec.SecretBindingName == nil {
		return []string{}
	}
	return []string{*shoot.Spec.SecretBindingName}
}

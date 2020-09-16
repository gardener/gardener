// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package index

import (
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"

	"k8s.io/apimachinery/pkg/runtime"
)

// SecretRefNamespaceField is the field name for the index function that extracts the corresponding field from SecretBinding.
const SecretRefNamespaceField string = "secretRef.namespace"

// SecretRefNamespaceIndexerFunc extracts the secretRef.namespace field of a SecretBinding.
func SecretRefNamespaceIndexerFunc(rawObj runtime.Object) []string {
	secretBinding, ok := rawObj.(*gardencorev1beta1.SecretBinding)
	if !ok {
		return []string{}
	}
	return []string{secretBinding.SecretRef.Namespace}
}

// SecretBindingNameField is the field name for the index function that extracts the corresponding field from Shoot.
const SecretBindingNameField string = "spec.secretBindingName"

// SecretBindingNameIndexerFunc extracts the spec.secretBindingName field of a Shoot.
func SecretBindingNameIndexerFunc(rawObj runtime.Object) []string {
	shoot, ok := rawObj.(*gardencorev1beta1.Shoot)
	if !ok {
		return []string{}
	}
	return []string{shoot.Spec.SecretBindingName}
}

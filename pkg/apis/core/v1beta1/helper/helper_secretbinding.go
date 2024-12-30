// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"strings"

	"k8s.io/apimachinery/pkg/util/sets"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

// SecretBindingHasType checks if the given SecretBinding has the given provider type.
func SecretBindingHasType(secretBinding *gardencorev1beta1.SecretBinding, providerType string) bool {
	if secretBinding.Provider == nil {
		return false
	}

	types := GetSecretBindingTypes(secretBinding)
	if len(types) == 0 {
		return false
	}

	return sets.New(types...).Has(providerType)
}

// AddTypeToSecretBinding adds the given provider type to the SecretBinding.
func AddTypeToSecretBinding(secretBinding *gardencorev1beta1.SecretBinding, providerType string) {
	if secretBinding.Provider == nil {
		secretBinding.Provider = &gardencorev1beta1.SecretBindingProvider{
			Type: providerType,
		}
		return
	}

	types := GetSecretBindingTypes(secretBinding)
	if !sets.New(types...).Has(providerType) {
		types = append(types, providerType)
	}
	secretBinding.Provider.Type = strings.Join(types, ",")
}

// GetSecretBindingTypes returns the SecretBinding provider types.
func GetSecretBindingTypes(secretBinding *gardencorev1beta1.SecretBinding) []string {
	return strings.Split(secretBinding.Provider.Type, ",")
}

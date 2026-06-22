// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
)

// GetSecretsForOCIRepository returns the names of all secrets referenced in the given OCIRepository.
func GetSecretsForOCIRepository(ociRepository *gardencorev1.OCIRepository) []string {
	var secretNames []string
	if ociRepository == nil {
		return secretNames
	}

	if ociRepository.CABundleSecretRef != nil {
		secretNames = append(secretNames, ociRepository.CABundleSecretRef.Name)
	}

	if ociRepository.PullSecretRef != nil {
		secretNames = append(secretNames, ociRepository.PullSecretRef.Name)
	}

	return secretNames
}

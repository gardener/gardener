// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	corev1 "k8s.io/api/core/v1"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

// SyncBackupSecretRefAndCredentialsRef syncs the seed backup credentials when possible.
// TODO(vpnachev): Remove this function after v1.121.0 has been released.
func SyncBackupSecretRefAndCredentialsRef(backup *gardencorev1beta1.SeedBackup) {
	if backup == nil {
		return
	}

	emptySecretRef := corev1.SecretReference{}

	// secretRef is set and credentialsRef is not, sync both fields.
	if backup.SecretRef != emptySecretRef && backup.CredentialsRef == nil {
		backup.CredentialsRef = &corev1.ObjectReference{
			APIVersion: "v1",
			Kind:       "Secret",
			Namespace:  backup.SecretRef.Namespace,
			Name:       backup.SecretRef.Name,
		}

		return
	}

	// secretRef is unset and credentialsRef refer a secret, sync both fields.
	if backup.SecretRef == emptySecretRef && backup.CredentialsRef != nil &&
		backup.CredentialsRef.APIVersion == "v1" && backup.CredentialsRef.Kind == "Secret" {
		backup.SecretRef = corev1.SecretReference{
			Namespace: backup.CredentialsRef.Namespace,
			Name:      backup.CredentialsRef.Name,
		}

		return
	}

	// in all other cases we can do nothing:
	// - both fields are unset -> we have nothing to sync
	// - both fields are set -> let the validation check if they are correct
	// - credentialsRef refer to WorkloadIdentity -> secretRef should stay unset
}

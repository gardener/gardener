// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	corev1 "k8s.io/api/core/v1"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

// SyncSeedDNSProviderCredentials syncs the seed DNS credentials when possible.
// TODO(vpnachev): Remove this function after v1.138.0 has been released.
func SyncSeedDNSProviderCredentials(dns *gardencorev1beta1.SeedDNSProvider) {
	if dns == nil {
		return
	}

	// secretRef is set and credentialsRef is not, sync both fields.
	if !isSecretRefEmpty(dns.SecretRef) && isCredentialsRefEmpty(dns.CredentialsRef) {
		dns.CredentialsRef = corev1.ObjectReference{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Secret",
			Namespace:  dns.SecretRef.Namespace,
			Name:       dns.SecretRef.Name,
		}

		return
	}

	// secretRef is unset and credentialsRef refer a secret, sync both fields.
	if isSecretRefEmpty(dns.SecretRef) &&
		!isCredentialsRefEmpty(dns.CredentialsRef) &&
		dns.CredentialsRef.APIVersion == corev1.SchemeGroupVersion.String() &&
		dns.CredentialsRef.Kind == "Secret" {
		dns.SecretRef = corev1.SecretReference{
			Namespace: dns.CredentialsRef.Namespace,
			Name:      dns.CredentialsRef.Name,
		}

		return
	}

	// in all other cases we can do nothing:
	// - both fields are unset -> we have nothing to sync
	// - both fields are set -> let the validation check if they are correct
	// - credentialsRef refer to WorkloadIdentity -> secretRef should stay unset
}

func isSecretRefEmpty(secretRef corev1.SecretReference) bool {
	return secretRef.Name == "" && secretRef.Namespace == ""
}

func isCredentialsRefEmpty(credentialsRef corev1.ObjectReference) bool {
	return credentialsRef.APIVersion == "" && credentialsRef.Kind == "" &&
		credentialsRef.Name == "" && credentialsRef.Namespace == ""
}

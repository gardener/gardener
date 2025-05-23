// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controlplane

import (
	corev1 "k8s.io/api/core/v1"

	"github.com/gardener/gardener/pkg/utils"
)

// MergeSecretMaps merges the 2 given secret maps.
func MergeSecretMaps(a, b map[string]*corev1.Secret) map[string]*corev1.Secret {
	x := make(map[string]*corev1.Secret, len(a))
	for _, m := range []map[string]*corev1.Secret{a, b} {
		for k, v := range m {
			x[k] = v
		}
	}
	return x
}

// ComputeChecksums computes and returns SAH256 checksums for the given secrets and configmaps.
func ComputeChecksums(secrets map[string]*corev1.Secret, cms map[string]*corev1.ConfigMap) map[string]string {
	checksums := make(map[string]string, len(secrets)+len(cms))
	for name, secret := range secrets {
		checksums[name] = utils.ComputeChecksum(secret.Data)
	}
	for name, cm := range cms {
		checksums[name] = utils.ComputeChecksum(cm.Data)
	}
	return checksums
}

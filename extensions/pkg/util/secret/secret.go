// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package secret

import (
	"context"

	"github.com/gardener/gardener/extensions/pkg/util/index"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// IsSecretInUseByShoot checks whether the given secret is in use by Shoot with the given provider type.
func IsSecretInUseByShoot(ctx context.Context, c client.Client, secret *corev1.Secret, providerType string) (bool, error) {
	// TODO: controller-runtime cached client does not support non-exact field matches.
	// Once this limitation is removed, we can add client.MatchingFields by secretRef.name and secretRef.namespace.
	secretBindings := &gardencorev1beta1.SecretBindingList{}
	if err := c.List(ctx, secretBindings,
		client.MatchingFields{index.SecretRefNamespaceField: secret.Namespace}); err != nil {
		return false, err
	}

	for _, secretBinding := range secretBindings.Items {
		// Filter out the the SecretBindings that do not reference the given secret
		if secretBinding.SecretRef.Name != secret.Name {
			continue
		}

		shoots := &gardencorev1beta1.ShootList{}
		if err := c.List(ctx, shoots,
			client.InNamespace(secretBinding.Namespace),
			client.MatchingFields{index.SecretBindingNameField: secretBinding.Name}); err != nil {
			return false, err
		}

		for _, shoot := range shoots.Items {
			if shoot.Spec.Provider.Type == providerType {
				return true, nil
			}
		}
	}

	return false, nil
}

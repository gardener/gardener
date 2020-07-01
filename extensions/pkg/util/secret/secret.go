// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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

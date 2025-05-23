// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package access

import (
	"context"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

// CreateShootClientFromStaticTokenKubeconfig retrieves the static token kubeconfig secret and creates a shoot client.
func CreateShootClientFromStaticTokenKubeconfig(ctx context.Context, gardenClient kubernetes.Interface, shoot *gardencorev1beta1.Shoot) (kubernetes.Interface, error) {
	return kubernetes.NewClientFromSecret(ctx, gardenClient.Client(), shoot.Namespace, gardenerutils.ComputeShootProjectResourceName(shoot.Name, "kubeconfig"),
		kubernetes.WithDisabledCachedClient(),
	)
}

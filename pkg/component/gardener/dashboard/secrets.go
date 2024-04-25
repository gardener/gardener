// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package dashboard

import (
	"context"

	corev1 "k8s.io/api/core/v1"

	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

func (g *gardenerDashboard) newVirtualGardenAccessSecret() *gardenerutils.AccessSecret {
	return gardenerutils.NewShootAccessSecret(deploymentName, g.namespace)
}

func (g *gardenerDashboard) reconcileSessionSecret(ctx context.Context) (*corev1.Secret, error) {
	// TODO(rfranzke): Auto-rotate this secret after https://github.com/gardener/dashboard/issues/1790 is implemented.
	return g.secretsManager.Generate(ctx, &secretsutils.BasicAuthSecretConfig{
		Name:           "gardener-dashboard-session-secret",
		Format:         secretsutils.BasicAuthFormatNormal,
		Username:       "admin",
		PasswordLength: 32,
	}, secretsmanager.Rotate(secretsmanager.InPlace))
}

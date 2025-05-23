// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package dashboard

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"

	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

func (g *gardenerDashboard) newVirtualGardenAccessSecret() *gardenerutils.AccessSecret {
	return gardenerutils.NewShootAccessSecret(deploymentName, g.namespace)
}

func (g *gardenerDashboard) reconcileSessionSecret(ctx context.Context) (*corev1.Secret, error) {
	return g.secretsManager.Generate(ctx, &secretsutils.BasicAuthSecretConfig{
		Name:           "gardener-dashboard-session-secret",
		Format:         secretsutils.BasicAuthFormatNormal,
		Username:       "admin",
		PasswordLength: 32,
	}, secretsmanager.Rotate(secretsmanager.KeepOld), secretsmanager.Validity(30*24*time.Hour), secretsmanager.IgnoreOldSecretsAfter(24*time.Hour))
}

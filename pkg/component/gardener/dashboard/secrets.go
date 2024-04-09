// Copyright 2024 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

func (g *gardenerDashboard) reconcileSecretSession(ctx context.Context) (*corev1.Secret, error) {
	// TODO(rfranzke): Auto-rotate this secret after https://github.com/gardener/dashboard/issues/1790 is implemented.
	return g.secretsManager.Generate(ctx, &secretsutils.BasicAuthSecretConfig{
		Name:           "gardener-dashboard-session-secret",
		Format:         secretsutils.BasicAuthFormatNormal,
		Username:       "admin",
		PasswordLength: 32,
	}, secretsmanager.Rotate(secretsmanager.InPlace))
}

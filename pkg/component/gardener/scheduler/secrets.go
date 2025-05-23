// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package scheduler

import (
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

func (g *gardenerScheduler) newVirtualGardenAccessSecret() *gardenerutils.AccessSecret {
	return gardenerutils.NewShootAccessSecret(DeploymentName, g.namespace)
}

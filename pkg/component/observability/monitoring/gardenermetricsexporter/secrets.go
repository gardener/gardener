// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package gardenermetricsexporter

import (
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

func (g *gardenerMetricsExporter) newVirtualGardenAccessSecret() *gardenerutils.AccessSecret {
	return gardenerutils.NewShootAccessSecret(deploymentName, g.namespace)
}

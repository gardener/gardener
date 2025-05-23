// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package test

import (
	"context"

	"github.com/gardener/gardener/charts"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
)

type gardenlet struct {
	kubernetes.ChartApplier
	values map[string]any
}

// NewGardenletChartApplier can be used to deploy the Gardenlet chart.
func NewGardenletChartApplier(applier kubernetes.ChartApplier, values map[string]any) component.Deployer {
	return &gardenlet{
		ChartApplier: applier,
		values:       values,
	}
}

func (c *gardenlet) Deploy(ctx context.Context) error {
	return c.ApplyFromEmbeddedFS(ctx, charts.ChartGardenlet, charts.ChartPathGardenlet, v1beta1constants.GardenNamespace, "gardenlet", kubernetes.Values(c.values))
}

func (c *gardenlet) Destroy(ctx context.Context) error {
	return c.DeleteFromEmbeddedFS(ctx, charts.ChartGardenlet, charts.ChartPathGardenlet, v1beta1constants.GardenNamespace, "gardenlet", kubernetes.Values(c.values))
}

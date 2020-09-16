// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"

	"github.com/gardener/gardener/pkg/chartrenderer"
	"github.com/gardener/gardener/pkg/utils/chart"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	"github.com/gardener/gardener/pkg/utils/managedresources"

	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// RenderChartAndCreateManagedResource renders a chart and creates a ManagedResource for the gardener-resource-manager
// out of the results.
func RenderChartAndCreateManagedResource(ctx context.Context, namespace string, name string, client client.Client, chartRenderer chartrenderer.Interface, chart chart.Interface, values map[string]interface{}, imageVector imagevector.ImageVector, chartNamespace string, version string, withNoCleanupLabel bool, forceOverwriteAnnotations bool) error {
	chartName, data, err := chart.Render(chartRenderer, chartNamespace, imageVector, version, version, values)
	if err != nil {
		return errors.Wrapf(err, "could not render chart")
	}

	// Create or update managed resource referencing the previously created secret
	var injectedLabels map[string]string
	if withNoCleanupLabel {
		injectedLabels = map[string]string{ShootNoCleanupLabel: "true"}
	}

	return managedresources.CreateManagedResource(ctx, client, namespace, name, "", chartName, data, false, injectedLabels, forceOverwriteAnnotations)
}

// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package controller

import (
	"context"

	"github.com/pkg/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/extensions/pkg/util"
	"github.com/gardener/gardener/pkg/chartrenderer"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	managedresources "github.com/gardener/gardener/pkg/utils/managedresources"
)

// RenderChartAndCreateManagedResource renders a chart and creates a ManagedResource for the gardener-resource-manager
// out of the results.
func RenderChartAndCreateManagedResource(ctx context.Context, namespace string, name string, client client.Client, chartRenderer chartrenderer.Interface, chart util.Chart, values map[string]interface{}, imageVector imagevector.ImageVector, chartNamespace string, version string, withNoCleanupLabel bool, forceOverwriteAnnotations bool) error {
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

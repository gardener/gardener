// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"fmt"

	"github.com/gardener/gardener/landscaper/pkg/controlplane/controller/chart"
	"github.com/gardener/gardener/landscaper/pkg/controlplane/controller/values"
)

const suffixApplicationChart = "application"

// DeployApplicationChart deploys the application chart into the Garden cluster
func (o *operation) DeployApplicationChart(ctx context.Context) error {
	var cgClusterIP *string

	if o.imports.VirtualGarden != nil && o.imports.VirtualGarden.Enabled {
		cgClusterIP = o.imports.VirtualGarden.ClusterIP
	}

	valuesHelper := values.NewApplicationChartValuesHelper(
		o.getGardenClient().Client(),
		o.imports.VirtualGarden != nil && o.imports.VirtualGarden.Enabled,
		cgClusterIP,
		*o.imports.GardenerAPIServer.ComponentConfiguration.CA.Crt,
		o.imports.GardenerAdmissionController.ComponentConfiguration.CA.Crt,
		o.imports.InternalDomain,
		o.imports.DefaultDomains,
		*o.imports.OpenVPNDiffieHellmanKey,
		o.imports.Alerting,
		o.admissionControllerConfig)

	values, err := valuesHelper.GetApplicationChartValues()
	if err != nil {
		return fmt.Errorf("failed to generate the values for the control plane application chart: %w", err)
	}

	applier := chart.NewChartApplier(o.getGardenClient().ChartApplier(), values, o.chartPath, suffixApplicationChart)
	if err := applier.Deploy(ctx); err != nil {
		return fmt.Errorf("failed deploying control plane application chart to the Garden cluster: %w", err)
	}

	return nil
}

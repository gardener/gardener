// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package charttest

import (
	"context"
	"path/filepath"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
)

type gardenlet struct {
	kubernetes.ChartApplier
	chartPath string
	values    map[string]interface{}
}

// NewGardenletChartApplier can be used to deploy the Gardenlet chart.
func NewGardenletChartApplier(
	applier kubernetes.ChartApplier,
	values map[string]interface{},
	chartsRootPath string,
) component.Deployer {
	return &gardenlet{
		ChartApplier: applier,
		chartPath:    filepath.Join(chartsRootPath, "gardener", "gardenlet"),
		values:       values,
	}
}

func (c *gardenlet) Deploy(ctx context.Context) error {
	return c.Apply(ctx, c.chartPath, v1beta1constants.GardenNamespace, "gardenlet", kubernetes.Values(c.values))
}

func (c *gardenlet) Destroy(ctx context.Context) error {
	return c.Delete(ctx, c.chartPath, v1beta1constants.GardenNamespace, "gardenlet", kubernetes.Values(c.values))
}

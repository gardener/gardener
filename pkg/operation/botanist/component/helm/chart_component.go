// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package helm

import (
	"context"

	"k8s.io/apimachinery/pkg/api/meta"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
)

type Values interface{}

func NewChartComponent(chartApplier kubernetes.ChartApplier, chartPath string, namespace, name string, values Values) component.DeployWaiter {
	return &ChartComponent{
		ChartApplier: chartApplier,
		ChartPath:    chartPath,
		Namespace:    namespace,
		Name:         name,
		Values:       values,
	}
}

type ChartComponent struct {
	kubernetes.ChartApplier
	ChartPath       string
	Namespace, Name string
	Values          Values
}

func (c ChartComponent) Deploy(ctx context.Context) error {
	return c.Apply(ctx, c.ChartPath, c.Namespace, c.Name, kubernetes.Values(c.Values))
}

func (c ChartComponent) Destroy(ctx context.Context) error {
	return c.Delete(ctx, c.ChartPath, c.Namespace, c.Name, kubernetes.Values(c.Values), kubernetes.TolerateErrorFunc(meta.IsNoMatchError))
}

func (c ChartComponent) Wait(ctx context.Context) error {
	return nil
}

func (c ChartComponent) WaitCleanup(ctx context.Context) error {
	return nil
}

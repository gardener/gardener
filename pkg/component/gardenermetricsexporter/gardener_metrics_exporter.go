// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package gardenermetricsexporter

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/component"
)

// Interface contains functions for a gardener-metrics-exporter deployer.
type Interface interface {
	component.DeployWaiter
}

// Values is a set of configuration values for the gardener-metrics-exporter component.
type Values struct {
	// Image is the container image used for gardener-metrics-exporter.
	Image string
}

// New creates a new instance of DeployWaiter for gardener-metrics-exporter.
func New(
	client client.Client,
	namespace string,
	values Values,
) Interface {
	return &gardenerMetricsExporter{
		client:    client,
		namespace: namespace,
		values:    values,
	}
}

type gardenerMetricsExporter struct {
	client    client.Client
	namespace string
	values    Values
}

func (g *gardenerMetricsExporter) Deploy(ctx context.Context) error      { return nil }
func (g *gardenerMetricsExporter) Destroy(ctx context.Context) error     { return nil }
func (g *gardenerMetricsExporter) Wait(ctx context.Context) error        { return nil }
func (g *gardenerMetricsExporter) WaitCleanup(ctx context.Context) error { return nil }

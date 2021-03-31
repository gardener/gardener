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

package istio

import (
	"context"
	"path/filepath"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/botanist/component"

	crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type crds struct {
	kubernetes.ChartApplier
	chartPath string
	client    crclient.Client
}

// NewIstioCRD can be used to deploy istio CRDs.
// Destroy does nothing.
func NewIstioCRD(
	applier kubernetes.ChartApplier,
	chartsRootPath string,
	client crclient.Client,
) component.DeployWaiter {
	return &crds{
		ChartApplier: applier,
		chartPath:    filepath.Join(chartsRootPath, "istio", "istio-crds"),
		client:       client,
	}
}

func (c *crds) Deploy(ctx context.Context) error {
	return c.Apply(ctx, c.chartPath, "", "istio")
}

func (c *crds) Destroy(ctx context.Context) error {
	// istio cannot be safely removed
	return nil
}

func (c *crds) Wait(ctx context.Context) error {
	return nil
}

func (c *crds) WaitCleanup(ctx context.Context) error {
	return nil
}

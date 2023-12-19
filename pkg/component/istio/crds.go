// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package istio

import (
	"context"
	"embed"
	"path/filepath"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
)

var (
	//go:embed charts/istio/istio-crds
	chartCRDs     embed.FS
	chartPathCRDs = filepath.Join("charts", "istio", "istio-crds")
)

type crds struct {
	kubernetes.ChartApplier
}

// NewCRD can be used to deploy istio CRDs.
func NewCRD(
	applier kubernetes.ChartApplier,
) component.DeployWaiter {
	return &crds{
		ChartApplier: applier,
	}
}

func (c *crds) Deploy(ctx context.Context) error {
	return c.ApplyFromEmbeddedFS(ctx, chartCRDs, chartPathCRDs, "", "istio")
}

func (c *crds) Destroy(ctx context.Context) error {
	return c.DeleteFromEmbeddedFS(ctx, chartCRDs, chartPathCRDs, "", "istio")
}

func (c *crds) Wait(_ context.Context) error {
	return nil
}

func (c *crds) WaitCleanup(_ context.Context) error {
	return nil
}

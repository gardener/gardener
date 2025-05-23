// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"

	"github.com/gardener/gardener/imagevector"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/nodeexporter"
	imagevectorutils "github.com/gardener/gardener/pkg/utils/imagevector"
)

// DefaultNodeExporter returns a deployer for the NodeExporter.
func (b *Botanist) DefaultNodeExporter() (component.DeployWaiter, error) {
	image, err := imagevector.Containers().FindImage(imagevector.ContainerImageNameNodeExporter, imagevectorutils.RuntimeVersion(b.ShootVersion()), imagevectorutils.TargetVersion(b.ShootVersion()))
	if err != nil {
		return nil, err
	}

	values := nodeexporter.Values{
		Image:      image.String(),
		VPAEnabled: b.Shoot.WantsVerticalPodAutoscaler,
	}

	return nodeexporter.New(
		b.SeedClientSet.Client(),
		b.Shoot.ControlPlaneNamespace,
		values,
	), nil
}

// ReconcileNodeExporter deploys or destroys the node-exporter component depending on whether shoot monitoring is enabled or not.
func (b *Botanist) ReconcileNodeExporter(ctx context.Context) error {
	if !b.IsShootMonitoringEnabled() {
		return b.Shoot.Components.SystemComponents.NodeExporter.Destroy(ctx)
	}

	return b.Shoot.Components.SystemComponents.NodeExporter.Deploy(ctx)
}

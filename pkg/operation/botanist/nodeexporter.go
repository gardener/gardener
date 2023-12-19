// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"

	"github.com/gardener/gardener/imagevector"
	"github.com/gardener/gardener/pkg/component/nodeexporter"
	imagevectorutils "github.com/gardener/gardener/pkg/utils/imagevector"
)

// DefaultNodeExporter returns a deployer for the NodeExporter.
func (b *Botanist) DefaultNodeExporter() (nodeexporter.Interface, error) {
	image, err := imagevector.ImageVector().FindImage(imagevector.ImageNameNodeExporter, imagevectorutils.RuntimeVersion(b.ShootVersion()), imagevectorutils.TargetVersion(b.ShootVersion()))
	if err != nil {
		return nil, err
	}

	values := nodeexporter.Values{
		Image:       image.String(),
		VPAEnabled:  b.Shoot.WantsVerticalPodAutoscaler,
		PSPDisabled: b.Shoot.PSPDisabled,
	}

	return nodeexporter.New(
		b.SeedClientSet.Client(),
		b.Shoot.SeedNamespace,
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

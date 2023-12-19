// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"

	"github.com/gardener/gardener/imagevector"
	"github.com/gardener/gardener/pkg/component/blackboxexporter"
	imagevectorutils "github.com/gardener/gardener/pkg/utils/imagevector"
)

// DefaultBlackboxExporter returns a deployer for the blackbox-exporter.
func (b *Botanist) DefaultBlackboxExporter() (blackboxexporter.Interface, error) {
	image, err := imagevector.ImageVector().FindImage(imagevector.ImageNameBlackboxExporter, imagevectorutils.RuntimeVersion(b.ShootVersion()), imagevectorutils.TargetVersion(b.ShootVersion()))
	if err != nil {
		return nil, err
	}

	return blackboxexporter.New(
		b.SeedClientSet.Client(),
		b.Shoot.SeedNamespace,
		blackboxexporter.Values{
			Image:             image.String(),
			VPAEnabled:        b.Shoot.WantsVerticalPodAutoscaler,
			KubernetesVersion: b.Shoot.KubernetesVersion,
		},
	), nil
}

// ReconcileBlackboxExporter deploys or destroys the blackbox-exporter component depending on whether shoot monitoring is enabled or not.
func (b *Botanist) ReconcileBlackboxExporter(ctx context.Context) error {
	if b.Operation.IsShootMonitoringEnabled() {
		return b.Shoot.Components.SystemComponents.BlackboxExporter.Deploy(ctx)
	}

	return b.Shoot.Components.SystemComponents.BlackboxExporter.Destroy(ctx)
}

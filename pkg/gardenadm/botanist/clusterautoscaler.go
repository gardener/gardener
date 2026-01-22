// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"fmt"

	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/autoscaling/clusterautoscaler"
)

// DeployClusterAutoscaler deploys the cluster-autoscaler for self-hosted shoots.
func (b *GardenadmBotanist) DeployClusterAutoscaler(ctx context.Context) error {
	if err := component.OpWait(clusterautoscaler.NewBootstrapper(b.SeedClientSet.Client(), b.Shoot.ControlPlaneNamespace)).Deploy(ctx); err != nil {
		return fmt.Errorf("failed deploying cluster-autoscaler bootstrapper: %w", err)
	}

	if err := b.Shoot.Components.Extensions.Worker.WaitUntilWorkerStatusMachineDeploymentsUpdated(ctx); err != nil {
		return fmt.Errorf("failed waiting for worker status machine deployments: %w", err)
	}

	return b.Botanist.DeployClusterAutoscaler(ctx)
}

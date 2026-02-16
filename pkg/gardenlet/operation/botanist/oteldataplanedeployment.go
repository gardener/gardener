// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"

	"github.com/gardener/gardener/imagevector"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/observability/opentelemetry/dataplanedeployment"
	"github.com/gardener/gardener/pkg/features"
	imagevectorutils "github.com/gardener/gardener/pkg/utils/imagevector"
)

// DefaultOtelDataplaneDeployment returns a deployer for the OTEL Collector dataplane deployment.
func (b *Botanist) DefaultOtelDataplaneDeployment() (component.DeployWaiter, error) {
	image, err := imagevector.Containers().FindImage(
		imagevector.ContainerImageNameOpentelemetryCollector,
		imagevectorutils.RuntimeVersion(b.ShootVersion()),
		imagevectorutils.TargetVersion(b.ShootVersion()),
	)
	if err != nil {
		return nil, err
	}

	config := dataplanedeployment.Config{
		Image:    image.String(),
		Replicas: 1,
	}

	return dataplanedeployment.New(
		b.SeedClientSet.Client(),
		b.Shoot.ControlPlaneNamespace,
		config,
	), nil
}

// ReconcileOtelDataplaneDeployment deploys or destroys the OTEL dataplane collector deployment component
// depending on whether the feature gate is enabled and shoot monitoring is enabled.
func (b *Botanist) ReconcileOtelDataplaneDeployment(ctx context.Context) error {
	if !features.DefaultFeatureGate.Enabled(features.OpenTelemetryDataplaneCollector) ||
		!b.IsShootMonitoringEnabled() {
		return b.Shoot.Components.SystemComponents.OtelDataplaneDeployment.Destroy(ctx)
	}

	return b.Shoot.Components.SystemComponents.OtelDataplaneDeployment.Deploy(ctx)
}

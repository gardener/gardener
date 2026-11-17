// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"

	gardenlethelper "github.com/gardener/gardener/pkg/api/config/gardenlet/v1alpha1/helper"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/perses"
	"github.com/gardener/gardener/pkg/component/shared"
	"github.com/gardener/gardener/pkg/features"
)

// DefaultPerses returns a deployer for Perses.
func (b *Botanist) DefaultPerses() (perses.Interface, error) {
	values := perses.Values{
		ClusterType:         component.ClusterTypeShoot,
		Replicas:            b.Shoot.GetReplicas(1),
		PriorityClassName:   v1beta1constants.PriorityClassNameShootControlPlane100,
		VPAEnabled:          b.Shoot.WantsVerticalPodAutoscaler,
		VictoriaLogsEnabled: b.Shoot.IsShootControlPlaneLoggingEnabled(b.Config) && gardenlethelper.IsVictoriaLogsEnabled(b.Config),
	}

	if !b.Shoot.IsSelfHosted() {
		values.ExternalExposure = &perses.ExposureValues{
			AuthSecretName:               v1beta1constants.SecretNameObservabilityIngressUsers,
			AuthSecretManaged:            true,
			Host:                         b.ComputePersesHost(),
			IstioIngressGatewayLabels:    b.WildcardIstioLabels(),
			IstioIngressGatewayNamespace: b.WildcardIstioNamespace(),
			SecretsManager:               b.SecretsManager,
			SigningCA:                    v1beta1constants.SecretNameCACluster,
		}
		if b.ControlPlaneWildcardCert != nil {
			values.ExternalExposure.WildcardCertSecretName = new(b.ControlPlaneWildcardCert.GetName())
		}
	}

	return shared.NewPerses(b.SeedClientSet.Client(), b.Shoot.ControlPlaneNamespace, values)
}

// DeployPerses deploys Perses for the shoot in the seed cluster. It is destroyed if the PersesShoot feature gate is
// disabled or if the shoot does not need observability components.
func (b *Botanist) DeployPerses(ctx context.Context) error {
	if !b.wantsPerses() {
		return b.Shoot.Components.ControlPlane.Perses.Destroy(ctx)
	}

	return b.Shoot.Components.ControlPlane.Perses.Deploy(ctx)
}

// WaitForPerses waits until Perses for the shoot in the seed cluster is reconciled.
func (b *Botanist) WaitForPerses(ctx context.Context) error {
	if !b.wantsPerses() {
		return b.Shoot.Components.ControlPlane.Perses.WaitCleanup(ctx)
	}

	return b.Shoot.Components.ControlPlane.Perses.Wait(ctx)
}

func (b *Botanist) wantsPerses() bool {
	return features.DefaultFeatureGate.Enabled(features.PersesShoot) && b.WantsObservabilityComponents()
}

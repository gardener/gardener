// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"

	"k8s.io/utils/ptr"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/observability/plutono"
	"github.com/gardener/gardener/pkg/component/shared"
)

// DefaultPlutono returns a deployer for Plutono.
func (b *Botanist) DefaultPlutono() (plutono.Interface, error) {
	return shared.NewPlutono(
		b.SeedClientSet.Client(),
		b.Shoot.ControlPlaneNamespace,
		b.SecretsManager,
		component.ClusterTypeShoot,
		b.Shoot.GetReplicas(1),
		"",
		b.ComputePlutonoHost(),
		v1beta1constants.PriorityClassNameShootControlPlane100,
		b.ShootUsesDNS(),
		b.Shoot.IsWorkerless,
		false,
		b.Shoot.VPNHighAvailabilityEnabled,
		b.Shoot.WantsVerticalPodAutoscaler,
		nil,
	)
}

// DeployPlutono deploys the plutono in the Seed cluster.
func (b *Botanist) DeployPlutono(ctx context.Context) error {
	// disable plutono if no observability components are needed
	if !b.WantsObservabilityComponents() {
		return b.Shoot.Components.ControlPlane.Plutono.Destroy(ctx)
	}

	if b.ControlPlaneWildcardCert != nil {
		b.Shoot.Components.ControlPlane.Plutono.SetWildcardCertName(ptr.To(b.ControlPlaneWildcardCert.GetName()))
	}

	return b.Shoot.Components.ControlPlane.Plutono.Deploy(ctx)
}

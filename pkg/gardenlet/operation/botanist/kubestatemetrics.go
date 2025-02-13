// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"

	"github.com/gardener/gardener/imagevector"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/kubestatemetrics"
	imagevectorutils "github.com/gardener/gardener/pkg/utils/imagevector"
)

// DefaultKubeStateMetrics returns a deployer for the kube-state-metrics.
func (b *Botanist) DefaultKubeStateMetrics() (component.DeployWaiter, error) {
	image, err := imagevector.Containers().FindImage(imagevector.ContainerImageNameKubeStateMetrics, imagevectorutils.RuntimeVersion(b.SeedVersion()), imagevectorutils.TargetVersion(b.ShootVersion()))
	if err != nil {
		return nil, err
	}

	return kubestatemetrics.New(
		b.SeedClientSet.Client(),
		b.Shoot.ControlPlaneNamespace,
		b.SecretsManager,
		kubestatemetrics.Values{
			ClusterType:       component.ClusterTypeShoot,
			KubernetesVersion: b.Shoot.KubernetesVersion,
			Image:             image.String(),
			PriorityClassName: v1beta1constants.PriorityClassNameShootControlPlane100,
			Replicas:          b.Shoot.GetReplicas(1),
		},
	), nil
}

// DeployKubeStateMetrics deploys or destroys the kube-state-metrics to the shoot namespace in the seed.
func (b *Botanist) DeployKubeStateMetrics(ctx context.Context) error {
	if !b.IsShootMonitoringEnabled() {
		return b.Shoot.Components.ControlPlane.KubeStateMetrics.Destroy(ctx)
	}

	return b.Shoot.Components.ControlPlane.KubeStateMetrics.Deploy(ctx)
}

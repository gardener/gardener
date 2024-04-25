// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/blackboxexporter"
	shootblackboxexporter "github.com/gardener/gardener/pkg/component/observability/monitoring/blackboxexporter/shoot"
	sharedcomponent "github.com/gardener/gardener/pkg/component/shared"
)

// DefaultBlackboxExporter returns a deployer for the blackbox-exporter.
func (b *Botanist) DefaultBlackboxExporter() (blackboxexporter.Interface, error) {
	return sharedcomponent.NewBlackboxExporter(
		b.SeedClientSet.Client(),
		b.SecretsManager,
		b.Shoot.SeedNamespace,
		blackboxexporter.Values{
			ClusterType:       component.ClusterTypeShoot,
			VPAEnabled:        b.Shoot.WantsVerticalPodAutoscaler,
			KubernetesVersion: b.Shoot.KubernetesVersion,
			PodLabels: map[string]string{
				v1beta1constants.LabelNetworkPolicyShootFromSeed:    v1beta1constants.LabelNetworkPolicyAllowed,
				v1beta1constants.LabelNetworkPolicyToDNS:            v1beta1constants.LabelNetworkPolicyAllowed,
				v1beta1constants.LabelNetworkPolicyToPublicNetworks: v1beta1constants.LabelNetworkPolicyAllowed,
				v1beta1constants.LabelNetworkPolicyShootToAPIServer: v1beta1constants.LabelNetworkPolicyAllowed,
			},
			PriorityClassName: "system-cluster-critical",
			Config:            shootblackboxexporter.Config(),
		},
	)
}

// ReconcileBlackboxExporter deploys or destroys the blackbox-exporter component depending on whether shoot monitoring is enabled or not.
func (b *Botanist) ReconcileBlackboxExporter(ctx context.Context) error {
	if b.Operation.IsShootMonitoringEnabled() {
		return b.Shoot.Components.SystemComponents.BlackboxExporter.Deploy(ctx)
	}

	return b.Shoot.Components.SystemComponents.BlackboxExporter.Destroy(ctx)
}

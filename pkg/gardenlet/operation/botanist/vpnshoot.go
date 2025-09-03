// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"fmt"

	"github.com/gardener/gardener/imagevector"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	vpnseedserver "github.com/gardener/gardener/pkg/component/networking/vpn/seedserver"
	vpnshoot "github.com/gardener/gardener/pkg/component/networking/vpn/shoot"
	"github.com/gardener/gardener/pkg/features"
	imagevectorutils "github.com/gardener/gardener/pkg/utils/imagevector"
)

// DefaultVPNShoot returns a deployer for the VPNShoot
func (b *Botanist) DefaultVPNShoot() (vpnshoot.Interface, error) {
	imageNameVPNShootClient := imagevector.ContainerImageNameVpnClient
	image, err := imagevector.Containers().FindImage(imageNameVPNShootClient, imagevectorutils.RuntimeVersion(b.ShootVersion()), imagevectorutils.TargetVersion(b.ShootVersion()))
	if err != nil {
		return nil, err
	}

	var (
		openvpnPort int32 = vpnseedserver.GatewayPort
		headerKey   string
	)

	if features.DefaultFeatureGate.Enabled(features.UseUnifiedHTTPProxyPort) {
		openvpnPort = vpnseedserver.HTTPProxyGatewayPort
		headerKey = "X-Gardener-Destination"
	}
	values := vpnshoot.Values{
		Image:             image.String(),
		VPAEnabled:        b.Shoot.WantsVerticalPodAutoscaler,
		VPAUpdateDisabled: b.Shoot.VPNVPAUpdateDisabled,
		ReversedVPN: vpnshoot.ReversedVPNValues{
			Header:      "outbound|1194||" + vpnseedserver.ServiceName + "." + b.Shoot.ControlPlaneNamespace + ".svc.cluster.local",
			HeaderKey:   headerKey,
			Endpoint:    b.outOfClusterAPIServerFQDN(),
			OpenVPNPort: openvpnPort,
			IPFamilies:  b.Shoot.GetInfo().Spec.Networking.IPFamilies,
		},
		HighAvailabilityEnabled:              b.Shoot.VPNHighAvailabilityEnabled,
		HighAvailabilityNumberOfSeedServers:  b.Shoot.VPNHighAvailabilityNumberOfSeedServers,
		HighAvailabilityNumberOfShootClients: b.Shoot.VPNHighAvailabilityNumberOfShootClients,
		SeedPodNetwork:                       b.Seed.GetInfo().Spec.Networks.Pods,
	}

	return vpnshoot.New(
		b.SeedClientSet.Client(),
		b.Shoot.ControlPlaneNamespace,
		b.SecretsManager,
		values,
	), nil
}

// DeployVPNShoot deploys the vpn-shoot.
func (b *Botanist) DeployVPNShoot(ctx context.Context) error {
	b.Shoot.Components.SystemComponents.VPNShoot.SetPodNetworkCIDRs(b.Shoot.Networks.Pods)
	b.Shoot.Components.SystemComponents.VPNShoot.SetServiceNetworkCIDRs(b.Shoot.Networks.Services)
	b.Shoot.Components.SystemComponents.VPNShoot.SetNodeNetworkCIDRs(b.Shoot.Networks.Nodes)

	if err := b.Shoot.Components.SystemComponents.VPNShoot.Deploy(ctx); err != nil {
		return err
	}

	if features.DefaultFeatureGate.Enabled(features.UseUnifiedHTTPProxyPort) {
		return b.Shoot.UpdateInfoStatus(ctx, b.GardenClient, true, false, func(shoot *gardencorev1beta1.Shoot) error {
			condition := v1beta1helper.GetOrInitConditionWithClock(b.Clock, shoot.Status.Constraints, gardencorev1beta1.ShootUsesUnifiedHTTPProxyPort)
			condition = v1beta1helper.UpdatedConditionWithClock(b.Clock, condition, gardencorev1beta1.ConditionTrue, "ShootUsesUnifiedHTTPProxyPort", fmt.Sprintf("Shoot uses http-proxy port %d for VPN", vpnseedserver.HTTPProxyGatewayPort))
			shoot.Status.Constraints = v1beta1helper.MergeConditions(shoot.Status.Constraints, condition)
			return nil
		})
	}

	return nil
}

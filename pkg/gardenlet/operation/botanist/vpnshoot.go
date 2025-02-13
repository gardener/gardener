// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"github.com/gardener/gardener/imagevector"
	"github.com/gardener/gardener/pkg/component"
	vpnseedserver "github.com/gardener/gardener/pkg/component/networking/vpn/seedserver"
	vpnshoot "github.com/gardener/gardener/pkg/component/networking/vpn/shoot"
	imagevectorutils "github.com/gardener/gardener/pkg/utils/imagevector"
)

// DefaultVPNShoot returns a deployer for the VPNShoot
func (b *Botanist) DefaultVPNShoot() (component.DeployWaiter, error) {
	imageNameVPNShootClient := imagevector.ContainerImageNameVpnClient
	if !b.Shoot.UsesNewVPN {
		imageNameVPNShootClient = imagevector.ContainerImageNameVpnShootClient
	}
	image, err := imagevector.Containers().FindImage(imageNameVPNShootClient, imagevectorutils.RuntimeVersion(b.ShootVersion()), imagevectorutils.TargetVersion(b.ShootVersion()))
	if err != nil {
		return nil, err
	}

	values := vpnshoot.Values{
		Image:             image.String(),
		VPAEnabled:        b.Shoot.WantsVerticalPodAutoscaler,
		VPAUpdateDisabled: b.Shoot.VPNVPAUpdateDisabled,
		ReversedVPN: vpnshoot.ReversedVPNValues{
			Header:      "outbound|1194||" + vpnseedserver.ServiceName + "." + b.Shoot.ControlPlaneNamespace + ".svc.cluster.local",
			Endpoint:    b.outOfClusterAPIServerFQDN(),
			OpenVPNPort: 8132,
			IPFamilies:  b.Shoot.GetInfo().Spec.Networking.IPFamilies,
		},
		HighAvailabilityEnabled:              b.Shoot.VPNHighAvailabilityEnabled,
		HighAvailabilityNumberOfSeedServers:  b.Shoot.VPNHighAvailabilityNumberOfSeedServers,
		HighAvailabilityNumberOfShootClients: b.Shoot.VPNHighAvailabilityNumberOfShootClients,
		DisableNewVPN:                        !b.Shoot.UsesNewVPN,
		KubernetesVersion:                    b.Shoot.KubernetesVersion,
		SeedPodNetwork:                       b.Seed.GetInfo().Spec.Networks.Pods,
	}

	return vpnshoot.New(
		b.SeedClientSet.Client(),
		b.Shoot.ControlPlaneNamespace,
		b.SecretsManager,
		values,
	), nil
}

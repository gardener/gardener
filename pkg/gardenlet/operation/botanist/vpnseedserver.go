// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"

	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/imagevector"
	vpnseedserver "github.com/gardener/gardener/pkg/component/networking/vpn/seedserver"
	imagevectorutils "github.com/gardener/gardener/pkg/utils/imagevector"
)

// DefaultVPNSeedServer returns a deployer for the vpn-seed-server.
func (b *Botanist) DefaultVPNSeedServer() (vpnseedserver.Interface, error) {
	imageAPIServerProxy, err := imagevector.Containers().FindImage(imagevector.ContainerImageNameApiserverProxy, imagevectorutils.RuntimeVersion(b.SeedVersion()), imagevectorutils.TargetVersion(b.ShootVersion()))
	if err != nil {
		return nil, err
	}

	imageNameVPNSeedServer := imagevector.ContainerImageNameVpnServer
	imageVPNSeedServer, err := imagevector.Containers().FindImage(imageNameVPNSeedServer, imagevectorutils.RuntimeVersion(b.SeedVersion()), imagevectorutils.TargetVersion(b.ShootVersion()))
	if err != nil {
		return nil, err
	}

	values := vpnseedserver.Values{
		ImageAPIServerProxy: imageAPIServerProxy.String(),
		ImageVPNSeedServer:  imageVPNSeedServer.String(),
		Network: vpnseedserver.NetworkValues{
			IPFamilies: b.Shoot.GetInfo().Spec.Networking.IPFamilies,
			// Pod/service/node network CIDRs are set on deployment to handle dynamic network CIDRs
		},
		Replicas:                             b.Shoot.GetReplicas(1),
		HighAvailabilityEnabled:              b.Shoot.VPNHighAvailabilityEnabled,
		HighAvailabilityNumberOfSeedServers:  b.Shoot.VPNHighAvailabilityNumberOfSeedServers,
		HighAvailabilityNumberOfShootClients: b.Shoot.VPNHighAvailabilityNumberOfShootClients,
		VPAUpdateDisabled:                    b.Shoot.VPNVPAUpdateDisabled,
	}

	if b.ShootUsesDNS() {
		values.KubeAPIServerHost = ptr.To(b.outOfClusterAPIServerFQDN())
	}

	if b.Shoot.VPNHighAvailabilityEnabled {
		values.Replicas = b.Shoot.GetReplicas(int32(b.Shoot.VPNHighAvailabilityNumberOfSeedServers)) // #nosec G115 -- `b.Shoot.VPNHighAvailabilityNumberOfSeedServers` cannot be configured by users, it is set to `2` in the code.
	}

	return vpnseedserver.New(
		b.SeedClientSet.Client(),
		b.Shoot.ControlPlaneNamespace,
		b.SecretsManager,
		func() string { return b.IstioNamespace() },
		values,
	), nil
}

// DeployVPNServer deploys the vpn-seed-server.
func (b *Botanist) DeployVPNServer(ctx context.Context) error {
	b.Shoot.Components.ControlPlane.VPNSeedServer.SetNodeNetworkCIDRs(b.Shoot.Networks.Nodes)
	b.Shoot.Components.ControlPlane.VPNSeedServer.SetServiceNetworkCIDRs(b.Shoot.Networks.Services)
	b.Shoot.Components.ControlPlane.VPNSeedServer.SetPodNetworkCIDRs(b.Shoot.Networks.Pods)
	b.Shoot.Components.ControlPlane.VPNSeedServer.SetSeedNamespaceObjectUID(b.SeedNamespaceObject.UID)

	return b.Shoot.Components.ControlPlane.VPNSeedServer.Deploy(ctx)
}

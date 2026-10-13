// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"

	"github.com/gardener/gardener/imagevector"
	extensionsv1alpha1helper "github.com/gardener/gardener/pkg/api/extensions/v1alpha1/helper"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsdnsrecord "github.com/gardener/gardener/pkg/component/extensions/dnsrecord"
	vpnseedserver "github.com/gardener/gardener/pkg/component/networking/vpn/seedserver"
	vpnshoot "github.com/gardener/gardener/pkg/component/networking/vpn/shoot"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	imagevectorutils "github.com/gardener/gardener/pkg/utils/imagevector"
)

const dnsRecordLiveMigrationVPNName = "live-migration-vpn"

// LiveMigrationTemporaryVPNDNSName returns the hostname for the temporary VPN DNS record created during live
// control plane migration.
func LiveMigrationTemporaryVPNDNSName(internalClusterDomain string) string {
	return "vpn-tmp." + internalClusterDomain
}

// DefaultLiveMigrationVPNDNSRecord returns a DNSRecord deployer for vpn-tmp.<internalDomain>. It is built
// analogously to DefaultInternalDNSRecord but targets the temporary VPN subdomain.
func (b *Botanist) DefaultLiveMigrationVPNDNSRecord() extensionsdnsrecord.Interface {
	values := &extensionsdnsrecord.Values{
		Name:              b.Shoot.GetInfo().Name + "-" + dnsRecordLiveMigrationVPNName,
		SecretName:        DNSRecordSecretPrefix + "-" + b.Shoot.GetInfo().Name + "-" + dnsRecordLiveMigrationVPNName,
		Namespace:         b.Shoot.ControlPlaneNamespace,
		TTL:               b.dnsRecordTTLSeconds(),
		AnnotateOperation: true,
		IPStack:           gardenerutils.GetIPStackForShoot(b.Shoot.GetInfo()),
		Labels: map[string]string{
			v1beta1constants.LabelRole:  "live-migration-vpn",
			v1beta1constants.GardenRole: v1beta1constants.GardenRoleControlPlane,
		},
	}

	var credentialsDeployer extensionsdnsrecord.CredentialsDeployFunc
	if b.NeedsInternalDNS() {
		values.Type = b.Garden.InternalDomain.Provider
		if b.Garden.InternalDomain.Zone != "" {
			values.Zone = &b.Garden.InternalDomain.Zone
		}
		credentialsDeployer = extensionsdnsrecord.CredentialsDeployerFromCredentials(b.Garden.InternalDomain.Credentials, b.Shoot.GetInfo())
		values.DNSName = LiveMigrationTemporaryVPNDNSName(*b.Shoot.InternalClusterDomain)
	}

	return extensionsdnsrecord.New(
		b.Logger,
		b.SeedClientSet.Client(),
		values,
		extensionsdnsrecord.DefaultInterval,
		extensionsdnsrecord.DefaultSevereThreshold,
		extensionsdnsrecord.DefaultTimeout,
		credentialsDeployer,
	)
}

// DeployLiveMigrationVPNDNSRecord sets the record type and value from the destination seed's Istio LB address
// (b.APIServerAddress) and deploys the DNS record.
func (b *Botanist) DeployLiveMigrationVPNDNSRecord(ctx context.Context) error {
	b.Shoot.Components.Extensions.LiveMigrationVPNDNSRecord.SetRecordType(extensionsv1alpha1helper.GetDNSRecordType(b.APIServerAddress))
	b.Shoot.Components.Extensions.LiveMigrationVPNDNSRecord.SetValues([]string{b.APIServerAddress})
	return b.Shoot.Components.Extensions.LiveMigrationVPNDNSRecord.Deploy(ctx)
}

// DestroyLiveMigrationVPNDNSRecord destroys the vpn-tmp DNS record and waits for cleanup.
func (b *Botanist) DestroyLiveMigrationVPNDNSRecord(ctx context.Context) error {
	if err := b.Shoot.Components.Extensions.LiveMigrationVPNDNSRecord.Destroy(ctx); err != nil {
		return err
	}
	return b.Shoot.Components.Extensions.LiveMigrationVPNDNSRecord.WaitCleanup(ctx)
}

// DefaultTemporaryVPNShoot returns the vpn-shoot-tmp component deployed on the destination seed during live
// migration. It tunnels to the destination via the dedicated vpn-tmp.<internalDomain> endpoint, giving the new
// kube-apiserver a path into the shoot network before DNS is cut over.
func (b *Botanist) DefaultTemporaryVPNShoot() (vpnshoot.Interface, error) {
	endpoint := LiveMigrationTemporaryVPNDNSName(*b.Shoot.InternalClusterDomain)

	image, err := imagevector.Containers().FindImage(imagevector.ContainerImageNameVpnClient, imagevectorutils.RuntimeVersion(b.ShootVersion()), imagevectorutils.TargetVersion(b.ShootVersion()))
	if err != nil {
		return nil, err
	}

	return vpnshoot.New(
		b.SeedClientSet.Client(),
		b.Shoot.ControlPlaneNamespace,
		b.SecretsManager,
		vpnshoot.Values{
			Image: image.String(),
			ReversedVPN: vpnshoot.ReversedVPNValues{
				Header:     "outbound|1194||" + vpnseedserver.ServiceName + "." + b.Shoot.ControlPlaneNamespace + ".svc.cluster.local",
				Endpoint:   endpoint + ".",
				IPFamilies: b.Shoot.GetInfo().Spec.Networking.IPFamilies,
			},
			HighAvailabilityEnabled:              b.Shoot.VPNHighAvailabilityEnabled,
			HighAvailabilityNumberOfSeedServers:  b.Shoot.VPNHighAvailabilityNumberOfSeedServers,
			HighAvailabilityNumberOfShootClients: b.Shoot.VPNHighAvailabilityNumberOfShootClients,
			// Offset the temporary clients' bonding indices to N..2N-1 so they don't collide with the permanent
			// vpn-shoot's 0..N-1 on the shared destination vpn-seed-server. Certificate identity stays on the raw
			// ordinal (reusing vpn-shoot-client-0..N-1), which is legal under the server's duplicate-cn.
			ClientIndexOffset: b.Shoot.VPNHighAvailabilityNumberOfShootClients,
			SeedPodNetwork:    b.Seed.GetInfo().Spec.Networks.Pods,
			NameSuffix:        "tmp",
		},
	), nil
}

// DeployTemporaryVPNShoot sets network CIDRs and deploys the vpn-shoot-tmp ManagedResource.
func (b *Botanist) DeployTemporaryVPNShoot(ctx context.Context) error {
	b.Shoot.Components.SystemComponents.TemporaryVPNShoot.SetPodNetworkCIDRs(b.Shoot.Networks.Pods)
	b.Shoot.Components.SystemComponents.TemporaryVPNShoot.SetServiceNetworkCIDRs(b.Shoot.Networks.Services)
	b.Shoot.Components.SystemComponents.TemporaryVPNShoot.SetNodeNetworkCIDRs(b.Shoot.Networks.Nodes)
	return b.Shoot.Components.SystemComponents.TemporaryVPNShoot.Deploy(ctx)
}

// DestroyTemporaryVPNShoot deletes the vpn-shoot-tmp ManagedResource.
func (b *Botanist) DestroyTemporaryVPNShoot(ctx context.Context) error {
	return b.Shoot.Components.SystemComponents.TemporaryVPNShoot.Destroy(ctx)
}

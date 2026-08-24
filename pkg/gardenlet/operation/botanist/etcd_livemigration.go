// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"fmt"

	druidcorev1alpha1 "github.com/gardener/etcd-druid/api/core/v1alpha1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1helper "github.com/gardener/gardener/pkg/api/core/v1beta1/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/component/etcd/etcd"
	etcdconstants "github.com/gardener/gardener/pkg/component/etcd/etcd/constants"
	"github.com/gardener/gardener/pkg/component/etcd/peerexposure"
)

// NewPeerExposure is the constructor for the peer exposure component. Exposed for testing.
var NewPeerExposure = func(c client.Client, namespace string, values peerexposure.Values) component.DeployWaiter {
	return peerexposure.New(c, namespace, values)
}

// LiveMigrationEtcdPeerHost returns the SNI host under which the peer endpoint of the etcd member with the given role
// and ordinal on the given seed is reachable.
func LiveMigrationEtcdPeerHost(seedName, shootNamespace, ingressDomain, role string, ordinal int) string {
	return fmt.Sprintf("%s-%s.%s", liveMigrationEtcdMemberName(seedName, role, ordinal), shootNamespace, ingressDomain)
}

// LiveMigrationEtcdClientHost returns the SNI host under which the etcd client endpoint of the given seed is reachable.
func LiveMigrationEtcdClientHost(seedName, shootNamespace, ingressDomain, role string) string {
	return fmt.Sprintf("%s-%s-%s-client.%s", seedName, shootNamespace, etcd.Name(role), ingressDomain)
}

// liveMigrationEtcdMemberName returns the etcd member name for the given seed, role and ordinal. It matches
// etcd-druid's member naming (`<memberNamePrefix>-<etcd-name>-<ordinal>`), where the member name prefix is the seed.
func liveMigrationEtcdMemberName(seedName, role string, ordinal int) string {
	return fmt.Sprintf("%s-%s-%d", seedName, etcd.Name(role), ordinal)
}

// ComputeMemberPeerURLs returns the cross-seed peer URLs for the etcd members of the given seed and role.
// Each member is reachable via its SNI hostname on the seed's Istio ingress gateway.
func ComputeMemberPeerURLs(seedName, shootNamespace, ingressDomain, role string, replicas int32) []druidcorev1alpha1.MemberPeerURLs {
	memberPeerURLs := make([]druidcorev1alpha1.MemberPeerURLs, 0, replicas)
	for ordinal := 0; ordinal < int(replicas); ordinal++ {
		memberPeerURLs = append(memberPeerURLs, druidcorev1alpha1.MemberPeerURLs{
			MemberName: liveMigrationEtcdMemberName(seedName, role, ordinal),
			URLs:       []string{fmt.Sprintf("https://%s:%d", LiveMigrationEtcdPeerHost(seedName, shootNamespace, ingressDomain, role, ordinal), etcdconstants.PortEtcdPeer+int32(ordinal))},
		})
	}
	return memberPeerURLs
}

// DeployEtcdPeerExposure exposes the peer endpoints of this shoot's etcd members across seeds via the seed's shared Istio ingress gateway.
func (b *Botanist) DeployEtcdPeerExposure(ctx context.Context) error {
	var (
		role           = v1beta1constants.ETCDRoleMain
		seedName       = b.Seed.GetInfo().Name
		shootNamespace = b.Shoot.ControlPlaneNamespace
		ingressDomain  = b.Seed.IngressDomain()
		replicas       = getEtcdReplicas(b.Shoot.GetInfo())
	)

	members := make([]peerexposure.PeerMember, 0, replicas)
	for ordinal := 0; ordinal < int(replicas); ordinal++ {
		members = append(members, peerexposure.PeerMember{
			SNIHost:      LiveMigrationEtcdPeerHost(seedName, shootNamespace, ingressDomain, role, ordinal),
			PodFQDN:      etcdPodFQDN(role, ordinal, b.Shoot.ControlPlaneNamespace),
			ExternalPort: uint32(etcdconstants.PortEtcdPeerExternal) + uint32(ordinal), // #nosec G115 -- Port constants are positive values well within uint32 range.
		})
	}

	peerExposure := NewPeerExposure(b.SeedClientSet.Client(), b.Shoot.ControlPlaneNamespace, peerexposure.Values{
		Role:                         role,
		Members:                      members,
		ClientHost:                   LiveMigrationEtcdClientHost(seedName, shootNamespace, ingressDomain, role),
		IstioIngressGatewayNamespace: b.DefaultIstioNamespace(),
		IstioIngressGatewayLabels:    b.DefaultIstioLabels(),
	})

	if err := peerExposure.Deploy(ctx); err != nil {
		return fmt.Errorf("failed to deploy etcd peer exposure for role %q: %w", role, err)
	}

	return nil
}

// DestroyEtcdPeerExposure removes the cross-seed peer exposure of this shoot's etcd members.
func (b *Botanist) DestroyEtcdPeerExposure(ctx context.Context) error {
	var (
		role           = v1beta1constants.ETCDRoleMain
		seedName       = b.Seed.GetInfo().Name
		shootNamespace = b.Shoot.ControlPlaneNamespace
		ingressDomain  = b.Seed.IngressDomain()
		replicas       = getEtcdReplicas(b.Shoot.GetInfo())
	)

	members := make([]peerexposure.PeerMember, 0, replicas)
	for ordinal := 0; ordinal < int(replicas); ordinal++ {
		members = append(members, peerexposure.PeerMember{
			SNIHost:      LiveMigrationEtcdPeerHost(seedName, shootNamespace, ingressDomain, role, ordinal),
			PodFQDN:      etcdPodFQDN(role, ordinal, b.Shoot.ControlPlaneNamespace),
			ExternalPort: uint32(etcdconstants.PortEtcdPeerExternal) + uint32(ordinal), // #nosec G115 -- Port constants are positive values well within uint32 range.
		})
	}

	peerExposure := NewPeerExposure(b.SeedClientSet.Client(), b.Shoot.ControlPlaneNamespace, peerexposure.Values{
		Role:                         role,
		Members:                      members,
		IstioIngressGatewayNamespace: b.DefaultIstioNamespace(),
		IstioIngressGatewayLabels:    b.DefaultIstioLabels(),
	})

	if err := peerExposure.Destroy(ctx); err != nil {
		return fmt.Errorf("failed to destroy etcd peer exposure for role %q: %w", role, err)
	}

	return nil
}

// etcdPodFQDN returns the in-cluster DNS subdomain for an etcd StatefulSet pod through the headless peer Service.
func etcdPodFQDN(role string, ordinal int, namespace string) string {
	etcdName := etcd.Name(role)
	return fmt.Sprintf("%s-%d.%s-peer.%s.svc.cluster.local", etcdName, ordinal, etcdName, namespace)
}

// crossSeedPeerHostnames returns the combined Istio ingress peer hostnames for both the source and destination
// seeds (source first, then destination). The consistent ordering ensures the same config hash on both sides,
// so the secretsManager returns the already-restored source peer cert without regenerating it.
func crossSeedPeerHostnames(sourceSeedName, sourceIngressDomain, destSeedName, destIngressDomain, shootNamespace, role string, replicas int32) []string {
	hosts := make([]string, 0, 2*int(replicas))
	for ordinal := 0; ordinal < int(replicas); ordinal++ {
		hosts = append(hosts, LiveMigrationEtcdPeerHost(sourceSeedName, shootNamespace, sourceIngressDomain, role, ordinal))
	}
	for ordinal := 0; ordinal < int(replicas); ordinal++ {
		hosts = append(hosts, LiveMigrationEtcdPeerHost(destSeedName, shootNamespace, destIngressDomain, role, ordinal))
	}
	return hosts
}

// setLiveMigrationEtcdValues configures the cross-seed etcd values for a live control plane migration:
//   - The source sets AdditionalAdvertisePeerURLs to its OWN cross-seed peer URLs so other cluster members
//     can reach it via the Istio ingress gateway.
//   - The destination sets BootstrapWithExistingCluster with the source's cross-seed peer URLs (the same
//     URLs the source advertised) so etcd-druid joins the existing source cluster.
//   - SkipClientSANVerification is set on both sides because cross-seed peer TLS certificates carry the
//     local service SAN, not the Istio-exposed hostname.
func (b *Botanist) setLiveMigrationEtcdValues(ctx context.Context, values *etcd.Values, role string) error {
	if b.Shoot.IsSelfHosted() {
		return nil
	}
	// Only etcd-main is live migrated.
	if role != v1beta1constants.ETCDRoleMain {
		return nil
	}

	var (
		shoot             = b.Shoot.GetInfo()
		liveMigrationRole = v1beta1helper.GetLiveMigrationRole(shoot, b.Seed.GetInfo().Name)
		replicas          = getEtcdReplicas(shoot)
		shootNamespace    = b.Shoot.ControlPlaneNamespace
	)

	if liveMigrationRole == v1beta1helper.LiveMigrationRoleNone {
		return nil
	}

	values.SkipClientSANVerification = true

	switch liveMigrationRole {
	case v1beta1helper.LiveMigrationRoleSource:
		var (
			localSeedName      = b.Seed.GetInfo().Name
			localIngressDomain = b.Seed.IngressDomain()
			destSeedName       = ptr.Deref(shoot.Spec.SeedName, "")
			destSeed           = &gardencorev1beta1.Seed{}
			destIngressDomain  = ""
		)

		values.AdditionalAdvertisePeerURLs = ComputeMemberPeerURLs(localSeedName, shootNamespace, localIngressDomain, role, replicas)
		values.ExtraClientServiceDNSNames = []string{LiveMigrationEtcdClientHost(localSeedName, shootNamespace, localIngressDomain, role)}

		if err := b.GardenAPIReader.Get(ctx, client.ObjectKey{Name: destSeedName}, destSeed); err != nil {
			return fmt.Errorf("failed to get destination seed %q: %w", destSeedName, err)
		}

		if destSeed.Spec.Ingress != nil {
			destIngressDomain = destSeed.Spec.Ingress.Domain
		}
		values.ExtraPeerServiceDNSNames = crossSeedPeerHostnames(localSeedName, localIngressDomain, destSeedName, destIngressDomain, shootNamespace, role, replicas)

	case v1beta1helper.LiveMigrationRoleDestination:
		var (
			sourceSeedName      = ptr.Deref(shoot.Status.SeedName, "")
			sourceSeed          = &gardencorev1beta1.Seed{}
			sourceIngressDomain = ""
			localSeedName       = b.Seed.GetInfo().Name
			localIngressDomain  = b.Seed.IngressDomain()
		)

		if err := b.GardenAPIReader.Get(ctx, client.ObjectKey{Name: sourceSeedName}, sourceSeed); err != nil {
			return fmt.Errorf("failed to get source seed %q: %w", sourceSeedName, err)
		}

		if sourceSeed.Spec.Ingress != nil {
			sourceIngressDomain = sourceSeed.Spec.Ingress.Domain
		}

		sourcePeerURLs := ComputeMemberPeerURLs(sourceSeedName, shootNamespace, sourceIngressDomain, role, replicas)
		members := make([]druidcorev1alpha1.BootstrapExistingMember, 0, len(sourcePeerURLs))
		for _, m := range sourcePeerURLs {
			members = append(members, druidcorev1alpha1.BootstrapExistingMember{Name: m.MemberName, PeerURLs: m.URLs})
		}

		values.BootstrapWithExistingCluster = &druidcorev1alpha1.BootstrapWithExistingCluster{
			Members: members,
			ClientEndpoints: []string{
				fmt.Sprintf("https://%s:%d",
					LiveMigrationEtcdClientHost(sourceSeedName, shootNamespace, sourceIngressDomain, role),
					etcdconstants.PortEtcdClient),
			},
		}
		values.ExtraPeerServiceDNSNames = crossSeedPeerHostnames(sourceSeedName, sourceIngressDomain, localSeedName, localIngressDomain, shootNamespace, role, replicas)
		values.AdditionalAdvertisePeerURLs = ComputeMemberPeerURLs(localSeedName, shootNamespace, localIngressDomain, role, replicas)
	}

	return nil
}

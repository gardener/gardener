// SPDX-FileCopyrightText: Contributors to the Gardener project
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
	"github.com/gardener/gardener/pkg/component/etcd/memberexposure"
	"github.com/gardener/gardener/pkg/utils"
)

const (
	// liveMigrationHostLabelHashLength is the number of hex characters of the SHA256 hash used in the left-most DNS label
	// of the cross-seed etcd hostnames.
	liveMigrationHostLabelHashLength = 6
)

// NewMemberExposure is the constructor for the member exposure component. Exposed for testing.
var NewMemberExposure = func(c client.Client, namespace string, values memberexposure.Values) component.DeployWaiter {
	return memberexposure.New(c, namespace, values)
}

// CrossSeedPeerHostnames is the crossSeedPeerHostnames function. Exposed for testing.
var CrossSeedPeerHostnames = crossSeedPeerHostnames

// CrossSeedClientMemberHosts is the crossSeedClientMemberHosts function. Exposed for testing.
var CrossSeedClientMemberHosts = crossSeedClientMemberHosts

// SetLiveMigrationEtcdValues is the setLiveMigrationEtcdValues method. Exposed for testing.
var SetLiveMigrationEtcdValues = func(b *Botanist, ctx context.Context, values *etcd.Values, role string) error {
	return b.setLiveMigrationEtcdValues(ctx, values, role)
}

// LiveMigrationEtcdPeerHost returns the SNI host under which the peer endpoint of the etcd member with the given role
// and ordinal on the given seed is reachable from other seeds. It is derived from the seed's ingress domain so that it
// resolves to the seed's shared Istio ingress gateway load balancer.
// The shoot namespace is also included in the hash to ensure that concurrent migrations of different shoots from the same seed
// produce distinct SNI hostnames without affecting the etcd member name.
func LiveMigrationEtcdPeerHost(seedName, shootNamespace, ingressDomain, role string, ordinal int) string {
	hash := utils.ComputeSHA256Hex(fmt.Appendf(nil, "%s/%s/%s/%d", seedName, shootNamespace, role, ordinal))[:liveMigrationHostLabelHashLength]
	return fmt.Sprintf("etcd-%s-peer-%d-%s.%s", role, ordinal, hash, ingressDomain)
}

// LiveMigrationEtcdClientHost returns the SNI host under which the etcd client endpoint of the given seed is reachable.
func LiveMigrationEtcdClientHost(seedName, shootNamespace, ingressDomain, role string) string {
	hash := utils.ComputeSHA256Hex(fmt.Appendf(nil, "%s/%s/%s", seedName, shootNamespace, role))[:liveMigrationHostLabelHashLength]
	return fmt.Sprintf("etcd-%s-client-%s.%s", role, hash, ingressDomain)
}

// LiveMigrationEtcdClientMemberHost returns the per-member SNI host under which the etcd client endpoint of the given
// member on the given seed is advertised in the etcd cluster membership.
func LiveMigrationEtcdClientMemberHost(seedName, shootNamespace, ingressDomain, role string, ordinal int) string {
	hash := utils.ComputeSHA256Hex(fmt.Appendf(nil, "%s/%s/%s/%d", seedName, shootNamespace, role, ordinal))[:liveMigrationHostLabelHashLength]
	return fmt.Sprintf("etcd-%s-client-%d-%s.%s", role, ordinal, hash, ingressDomain)
}

// liveMigrationEtcdMemberName returns the etcd member name for the given seed, role and ordinal. It matches
// etcd-druid's member naming (`<memberNamePrefix>-<etcd-name>-<ordinal>`), where the member name prefix is the seed.
func liveMigrationEtcdMemberName(seedName, role string, ordinal int) string {
	return fmt.Sprintf("%s-%s-%d", seedName, etcd.Name(role), ordinal)
}

// ComputeMemberPeerURLs returns the cross-seed peer URLs for the etcd members of the given seed and role.
// Each member is reachable via its SNI hostname on the seed's Istio ingress gateway.
func ComputeMemberPeerURLs(seedName, shootNamespace, ingressDomain, role string, replicas int32) []druidcorev1alpha1.MemberURLs {
	memberPeerURLs := make([]druidcorev1alpha1.MemberURLs, 0, replicas)
	for i := 0; i < int(replicas); i++ {
		memberPeerURLs = append(memberPeerURLs, druidcorev1alpha1.MemberURLs{
			Name: liveMigrationEtcdMemberName(seedName, role, i),
			URLs: []string{fmt.Sprintf("https://%s:%d", LiveMigrationEtcdPeerHost(seedName, shootNamespace, ingressDomain, role, i), etcdconstants.PortEtcdPeerExternal+int32(i))}, // #nosec G115 -- Port constants are positive values well within int32 range.
		})
	}
	return memberPeerURLs
}

// ComputeMemberClientURLs returns the cross-seed client URLs for the etcd members of the given seed and role.
// Each member advertises its SNI client hostname on the seed's Istio ingress gateway.
func ComputeMemberClientURLs(seedName, shootNamespace, ingressDomain, role string, replicas int32) []druidcorev1alpha1.MemberURLs {
	memberClientURLs := make([]druidcorev1alpha1.MemberURLs, 0, replicas)
	for i := 0; i < int(replicas); i++ {
		memberClientURLs = append(memberClientURLs, druidcorev1alpha1.MemberURLs{
			Name: liveMigrationEtcdMemberName(seedName, role, i),
			URLs: []string{fmt.Sprintf("https://%s:%d", LiveMigrationEtcdClientMemberHost(seedName, shootNamespace, ingressDomain, role, i), memberexposure.EtcdClientPort)},
		})
	}
	return memberClientURLs
}

// DefaultEtcdMemberExposure returns a new instance of the component that exposes this shoot's etcd member endpoints
// for cross-seed live migration via the seed's shared Istio ingress gateway.
func (b *Botanist) DefaultEtcdMemberExposure() component.DeployWaiter {
	var (
		clientHost     string
		role           = v1beta1constants.ETCDRoleMain
		seedName       = b.Seed.GetInfo().Name
		shootNamespace = b.Shoot.ControlPlaneNamespace
		ingressDomain  = b.Seed.IngressDomain()
		replicas       = getEtcdReplicas(b.Shoot.GetInfo())
	)

	members := make([]memberexposure.Member, 0, replicas)
	for i := 0; i < int(replicas); i++ {
		member := memberexposure.Member{
			PeerSNIHost:   LiveMigrationEtcdPeerHost(seedName, shootNamespace, ingressDomain, role, i),
			PodFQDN:       etcdPodFQDN(role, i, shootNamespace),
			ExternalPort:  uint32(etcdconstants.PortEtcdPeerExternal) + uint32(i), // #nosec G115 -- Port constants are positive values well within uint32 range.
			ClientSNIHost: LiveMigrationEtcdClientMemberHost(seedName, shootNamespace, ingressDomain, role, i),
		}
		members = append(members, member)
	}

	if v1beta1helper.GetLiveMigrationRole(b.Shoot.GetInfo(), seedName) == v1beta1helper.LiveMigrationRoleSource {
		clientHost = LiveMigrationEtcdClientHost(seedName, shootNamespace, ingressDomain, role)
	}

	return NewMemberExposure(b.SeedClientSet.Client(), shootNamespace, memberexposure.Values{
		Role:                         role,
		Members:                      members,
		ClientHost:                   clientHost,
		IstioIngressGatewayNamespace: b.DefaultIstioNamespace(),
		IstioIngressGatewayLabels:    b.DefaultIstioLabels(),
	})
}

// DeployEtcdMemberExposure exposes the member endpoints of this shoot's etcd members across seeds via the seed's shared Istio ingress gateway.
func (b *Botanist) DeployEtcdMemberExposure(ctx context.Context) error {
	if b.Shoot.Components.ControlPlane.EtcdMemberExposure == nil {
		return nil
	}
	return b.Shoot.Components.ControlPlane.EtcdMemberExposure.Deploy(ctx)
}

// DestroyEtcdMemberExposure removes the cross-seed member exposure of this shoot's etcd members.
func (b *Botanist) DestroyEtcdMemberExposure(ctx context.Context) error {
	if b.Shoot.Components.ControlPlane.EtcdMemberExposure == nil {
		return nil
	}
	return b.Shoot.Components.ControlPlane.EtcdMemberExposure.Destroy(ctx)
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
	for i := 0; i < int(replicas); i++ {
		hosts = append(hosts, LiveMigrationEtcdPeerHost(sourceSeedName, shootNamespace, sourceIngressDomain, role, i))
	}
	for i := 0; i < int(replicas); i++ {
		hosts = append(hosts, LiveMigrationEtcdPeerHost(destSeedName, shootNamespace, destIngressDomain, role, i))
	}
	return hosts
}

// crossSeedClientMemberHosts returns the combined SNI client hostnames for both the source and destination
// seeds (source first, then destination). The consistent ordering ensures the same config hash on both sides,
// so the secretsManager returns the already-restored source client cert without regenerating it.
func crossSeedClientMemberHosts(sourceSeedName, sourceIngressDomain, destSeedName, destIngressDomain, shootNamespace, role string, replicas int32) []string {
	hosts := make([]string, 0, 2*int(replicas))
	for i := 0; i < int(replicas); i++ {
		hosts = append(hosts, LiveMigrationEtcdClientMemberHost(sourceSeedName, shootNamespace, sourceIngressDomain, role, i))
	}
	for i := 0; i < int(replicas); i++ {
		hosts = append(hosts, LiveMigrationEtcdClientMemberHost(destSeedName, shootNamespace, destIngressDomain, role, i))
	}
	return hosts
}

// setLiveMigrationEtcdValues configures the cross-seed etcd values for a live control plane migration:
//   - Both source and destination set AdditionalAdvertisedPeerURLs and AdditionalAdvertisedClientURLs (with
//     OverrideDefaultURL=true) to their OWN cross-seed URLs so members on the other cluster can reach them via
//     the Istio ingress gateway.
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

	switch liveMigrationRole {
	case v1beta1helper.LiveMigrationRoleSource:
		var (
			localSeedName      = b.Seed.GetInfo().Name
			localIngressDomain = b.Seed.IngressDomain()
			destSeedName       = ptr.Deref(shoot.Spec.SeedName, "")
			destSeed           = &gardencorev1beta1.Seed{}
			destIngressDomain  string
		)

		if err := b.GardenAPIReader.Get(ctx, client.ObjectKey{Name: destSeedName}, destSeed); err != nil {
			return fmt.Errorf("failed to get destination seed %q: %w", destSeedName, err)
		}

		if destSeed.Spec.Ingress == nil {
			return fmt.Errorf("destination seed %q has no ingress domain configured, which is required for live migration", destSeedName)
		}
		destIngressDomain = destSeed.Spec.Ingress.Domain

		values.LiveMigration = &etcd.LiveMigrationValues{
			SkipClientSANVerification: true,
			AdditionalAdvertisedURLs: &druidcorev1alpha1.AdditionalAdvertiseURLsSpec{
				PeerURLs: &druidcorev1alpha1.AdditionalURLsSpec{
					OverrideDefaultURL: new(true),
					Members:            ComputeMemberPeerURLs(localSeedName, shootNamespace, localIngressDomain, role, replicas),
				},
				ClientURLs: &druidcorev1alpha1.AdditionalURLsSpec{
					OverrideDefaultURL: new(true),
					Members:            ComputeMemberClientURLs(localSeedName, shootNamespace, localIngressDomain, role, replicas),
				},
			},
			ExtraClientServiceDNSNames: append(
				crossSeedClientMemberHosts(localSeedName, localIngressDomain, destSeedName, destIngressDomain, shootNamespace, role, replicas),
				LiveMigrationEtcdClientHost(localSeedName, shootNamespace, localIngressDomain, role),
			),
			ExtraPeerServiceDNSNames: crossSeedPeerHostnames(localSeedName, localIngressDomain, destSeedName, destIngressDomain, shootNamespace, role, replicas),
		}

	case v1beta1helper.LiveMigrationRoleDestination:
		var (
			sourceSeedName      = ptr.Deref(shoot.Status.SeedName, "")
			sourceSeed          = &gardencorev1beta1.Seed{}
			sourceIngressDomain string
			localSeedName       = b.Seed.GetInfo().Name
			localIngressDomain  = b.Seed.IngressDomain()
		)

		if err := b.GardenAPIReader.Get(ctx, client.ObjectKey{Name: sourceSeedName}, sourceSeed); err != nil {
			return fmt.Errorf("failed to get source seed %q: %w", sourceSeedName, err)
		}
		if sourceSeed.Spec.Ingress == nil {
			return fmt.Errorf("source seed %q has no ingress domain configured, which is required for live migration", sourceSeedName)
		}
		sourceIngressDomain = sourceSeed.Spec.Ingress.Domain

		sourcePeerURLs := ComputeMemberPeerURLs(sourceSeedName, shootNamespace, sourceIngressDomain, role, replicas)
		members := make([]druidcorev1alpha1.BootstrapExistingMember, 0, len(sourcePeerURLs))
		for _, m := range sourcePeerURLs {
			members = append(members, druidcorev1alpha1.BootstrapExistingMember{Name: m.Name, PeerURLs: m.URLs})
		}

		values.LiveMigration = &etcd.LiveMigrationValues{
			SkipClientSANVerification: true,
			BootstrapWithExistingCluster: &druidcorev1alpha1.BootstrapWithExistingCluster{
				Members:         members,
				ClientEndpoints: []string{fmt.Sprintf("https://%s:%d", LiveMigrationEtcdClientHost(sourceSeedName, shootNamespace, sourceIngressDomain, role), memberexposure.EtcdClientPort)},
			},
			ExtraPeerServiceDNSNames: crossSeedPeerHostnames(sourceSeedName, sourceIngressDomain, localSeedName, localIngressDomain, shootNamespace, role, replicas),
			AdditionalAdvertisedURLs: &druidcorev1alpha1.AdditionalAdvertiseURLsSpec{
				PeerURLs: &druidcorev1alpha1.AdditionalURLsSpec{
					OverrideDefaultURL: new(true),
					Members:            ComputeMemberPeerURLs(localSeedName, shootNamespace, localIngressDomain, role, replicas),
				},
				ClientURLs: &druidcorev1alpha1.AdditionalURLsSpec{
					OverrideDefaultURL: new(true),
					Members:            ComputeMemberClientURLs(localSeedName, shootNamespace, localIngressDomain, role, replicas),
				},
			},
			ExtraClientServiceDNSNames: crossSeedClientMemberHosts(sourceSeedName, sourceIngressDomain, localSeedName, localIngressDomain, shootNamespace, role, replicas),
		}
	}

	return nil
}

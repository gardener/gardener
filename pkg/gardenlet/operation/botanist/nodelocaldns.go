// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"fmt"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/imagevector"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/component/networking/nodelocaldns"
	imagevectorutils "github.com/gardener/gardener/pkg/utils/imagevector"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	versionutils "github.com/gardener/gardener/pkg/utils/version"
)

// DefaultNodeLocalDNS returns a deployer for the node-local-dns.
func (b *Botanist) DefaultNodeLocalDNS() (nodelocaldns.Interface, error) {
	image, err := imagevector.Containers().FindImage(imagevector.ContainerImageNameNodeLocalDns, imagevectorutils.RuntimeVersion(b.ShootVersion()), imagevectorutils.TargetVersion(b.ShootVersion()))
	if err != nil {
		return nil, err
	}

	imageAlpine, err := imagevector.Containers().FindImage(imagevector.ContainerImageNameAlpineIptables, imagevectorutils.RuntimeVersion(b.ShootVersion()), imagevectorutils.TargetVersion(b.ShootVersion()))
	if err != nil {
		return nil, err
	}

	return nodelocaldns.New(
		b.SeedClientSet.Client(),
		b.Shoot.ControlPlaneNamespace,
		nodelocaldns.Values{
			Image:       image.String(),
			AlpineImage: imageAlpine.String(),
			VPAEnabled:  b.Shoot.WantsVerticalPodAutoscaler,
			Config:      v1beta1helper.GetNodeLocalDNS(b.Shoot.GetInfo().Spec.SystemComponents),
		},
	), nil
}

// ReconcileNodeLocalDNS deploys or destroys the node-local-dns component depending on whether it is enabled for the Shoot.
func (b *Botanist) ReconcileNodeLocalDNS(ctx context.Context) error {
	// The node-local-dns interface cannot bind the kube-dns cluster IP since the interface
	// used for IPVS load-balancing already uses this address.
	clusterDNS := []string{"__PILLAR__CLUSTER__DNS__"}
	var dnsServers []string
	var coreDNS []string
	for _, ip := range b.Shoot.Networks.CoreDNS {
		coreDNS = append(coreDNS, ip.String())
	}
	if b.Shoot.IPVSEnabled() {
		clusterDNS = coreDNS
	} else {
		dnsServers = coreDNS
	}

	b.Shoot.Components.SystemComponents.NodeLocalDNS.SetClusterDNS(clusterDNS)
	b.Shoot.Components.SystemComponents.NodeLocalDNS.SetDNSServers(dnsServers)
	b.Shoot.Components.SystemComponents.NodeLocalDNS.SetIPFamilies(b.Shoot.GetInfo().Spec.Networking.IPFamilies)
	b.Shoot.Components.SystemComponents.NodeLocalDNS.SetShootClientSet(b.ShootClientSet)
	b.Shoot.Components.SystemComponents.NodeLocalDNS.SetSeedClientSet(b.SeedClientSet)
	b.Shoot.Components.SystemComponents.NodeLocalDNS.SetLogger(b.Logger)
	if b.Shoot.NodeLocalDNSEnabled {
		return b.Shoot.Components.SystemComponents.NodeLocalDNS.Deploy(ctx)
	}

	atLeastOnePoolLowerKubernetes134 := false
	// Check if at least one worker pool has a Kubernetes version lower than 1.34.
	// If so, we cannot remove NodeLocalDNS components yet, as they are still required
	// for nodes running that version.
	for _, worker := range b.Shoot.GetInfo().Spec.Provider.Workers {
		kubernetesVersion, err := v1beta1helper.CalculateEffectiveKubernetesVersion(b.Shoot.KubernetesVersion, worker.Kubernetes)
		if err != nil {
			return fmt.Errorf("failed to calculate effective Kubernetes version for worker %q: %w", worker.Name, err)
		}

		if versionutils.ConstraintK8sLess134.Check(kubernetesVersion) {
			atLeastOnePoolLowerKubernetes134 = true
			break
		}
	}
	kubeProxyConfig := b.Shoot.GetInfo().Spec.Kubernetes.KubeProxy
	if stillDesired, err := b.isNodeLocalDNSStillDesired(ctx); err != nil {
		return err
	} else if stillDesired && (atLeastOnePoolLowerKubernetes134 || v1beta1helper.IsKubeProxyIPVSMode(kubeProxyConfig)) {
		// Leave NodeLocalDNS components in the cluster until all nodes have been rolled
		return nil
	}

	return b.Shoot.Components.SystemComponents.NodeLocalDNS.Destroy(ctx)
}

// isNodeLocalDNSStillDesired indicates whether any node still requires node-local-dns components.
func (b *Botanist) isNodeLocalDNSStillDesired(ctx context.Context) (bool, error) {
	return kubernetesutils.ResourcesExist(ctx, b.ShootClientSet.Client(), &corev1.NodeList{}, b.ShootClientSet.Client().Scheme(), client.MatchingLabels{
		v1beta1constants.LabelNodeLocalDNS: strconv.FormatBool(true),
	})
}

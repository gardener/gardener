// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"fmt"
	"strconv"

	"github.com/Masterminds/semver/v3"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/imagevector"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
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

	imageCoreDNSConfigAdapater, err := imagevector.Containers().FindImage(imagevector.ContainerImageNameCorednsConfigAdapter)
	if err != nil {
		return nil, err
	}

	return nodelocaldns.New(
		b.SeedClientSet.Client(),
		b.Shoot.ControlPlaneNamespace,
		nodelocaldns.Values{
			Image:                      image.String(),
			AlpineImage:                imageAlpine.String(),
			CoreDNSConfigAdapaterImage: imageCoreDNSConfigAdapater.String(),
			VPAEnabled:                 b.Shoot.WantsVerticalPodAutoscaler,
			Config:                     v1beta1helper.GetNodeLocalDNS(b.Shoot.GetInfo().Spec.SystemComponents),
			Workers:                    b.Shoot.GetInfo().Spec.Provider.Workers,
			KubeProxyConfig:            b.Shoot.GetInfo().Spec.Kubernetes.KubeProxy,
			Log:                        b.Logger,
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
	if b.Shoot.NodeLocalDNSEnabled {
		return b.Shoot.Components.SystemComponents.NodeLocalDNS.Deploy(ctx)
	}

	atLeastOnePoolLowerKubernetes134, err := v1beta1helper.IsOneWorkerPoolLowerKubernetes134(b.Shoot.KubernetesVersion, b.Shoot.GetInfo().Spec.Provider.Workers)
	if err != nil {
		return err
	}

	kubeProxyConfig := b.Shoot.GetInfo().Spec.Kubernetes.KubeProxy
	if stillDesired, err := b.isNodeLocalDNSStillDesired(ctx); err != nil {
		return err
	} else if stillDesired && (atLeastOnePoolLowerKubernetes134 || v1beta1helper.IsKubeProxyIPVSMode(kubeProxyConfig)) {
		// Leave NodeLocalDNS components in the cluster until all nodes have been rolled
		if !v1beta1helper.IsKubeProxyIPVSMode(kubeProxyConfig) {
			if err := nodelocaldns.MarkNodesForCleanup(ctx, b.ShootClientSet.Client(), b.Shoot.GetInfo().Spec.Provider.Workers); err != nil {
				return fmt.Errorf("failed to mark nodes for cleanup: %w", err)
			}

			imageAlpine, err := imagevector.Containers().FindImage(imagevector.ContainerImageNameAlpineIptables, imagevectorutils.RuntimeVersion(b.ShootVersion()), imagevectorutils.TargetVersion(b.ShootVersion()))
			if err != nil {
				return err
			}

			cleanupRequired, err := hasKubernetesVersionsBelowAndAbove134(b.Shoot.KubernetesVersion, b.Shoot.GetInfo().Spec.Provider.Workers)
			if err != nil {
				return err
			}

			if cleanupRequired {
				if err = nodelocaldns.RunCleanup(ctx, b.ShootClientSet.Client(), imageAlpine.String(), b.Logger); err != nil {
					return fmt.Errorf("failed to run node-local-dns cleanup: %w", err)
				}
			}
		}
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

// hasKubernetesVersionsBelowAndAbove134 checks if there are worker pools with Kubernetes versions below 1.34 and others with versions 1.34 or above.
func hasKubernetesVersionsBelowAndAbove134(controlPlaneVersion *semver.Version, workers []gardencorev1beta1.Worker) (bool, error) {
	hasLower134 := false
	hasGreaterOrEqual134 := false
	controlPlaneIsLower134 := versionutils.ConstraintK8sLess134.Check(controlPlaneVersion)
	for _, worker := range workers {
		if worker.Kubernetes != nil && worker.Kubernetes.Version != nil {
			kubernetesVersion, err := semver.NewVersion(*worker.Kubernetes.Version)
			if err != nil {
				return hasLower134 && hasGreaterOrEqual134, err
			}

			if versionutils.ConstraintK8sLess134.Check(kubernetesVersion) {
				hasLower134 = true
			} else {
				hasGreaterOrEqual134 = true
			}
		} else {
			if controlPlaneIsLower134 {
				hasLower134 = true
			} else {
				hasGreaterOrEqual134 = true
			}
		}
	}
	return hasLower134 && hasGreaterOrEqual134, nil
}

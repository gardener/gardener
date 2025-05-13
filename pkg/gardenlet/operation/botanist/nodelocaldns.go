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
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	nodelocaldns "github.com/gardener/gardener/pkg/component/networking/nodelocaldns"
	imagevectorutils "github.com/gardener/gardener/pkg/utils/imagevector"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// DefaultNodeLocalDNS returns a deployer for the node-local-dns.
func (b *Botanist) DefaultNodeLocalDNS() (nodelocaldns.Interface, error) {
	image, err := imagevector.Containers().FindImage(imagevector.ContainerImageNameNodeLocalDns, imagevectorutils.RuntimeVersion(b.ShootVersion()), imagevectorutils.TargetVersion(b.ShootVersion()))
	if err != nil {
		return nil, err
	}

	return nodelocaldns.New(
		b.SeedClientSet.Client(),
		b.Shoot.ControlPlaneNamespace,
		nodelocaldns.Values{
			Image:      image.String(),
			VPAEnabled: b.Shoot.WantsVerticalPodAutoscaler,
			Config:     v1beta1helper.GetNodeLocalDNS(b.Shoot.GetInfo().Spec.SystemComponents),
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
	workerPools, err := b.computeWorkerPoolsForNodeLocalDNS(ctx)
	if err != nil {
		return err
	}

	b.Shoot.Components.SystemComponents.NodeLocalDNS.SetClusterDNS(clusterDNS)
	b.Shoot.Components.SystemComponents.NodeLocalDNS.SetDNSServers(dnsServers)
	b.Shoot.Components.SystemComponents.NodeLocalDNS.SetIPFamilies(b.Shoot.GetInfo().Spec.Networking.IPFamilies)
	b.Shoot.Components.SystemComponents.NodeLocalDNS.SetWorkerPools(workerPools)
	b.Shoot.Components.SystemComponents.NodeLocalDNS.SetShootClientSet(b.ShootClientSet)
	b.Shoot.Components.SystemComponents.NodeLocalDNS.SetLogger(b.Logger)
	if b.Shoot.NodeLocalDNSEnabled {
		return b.Shoot.Components.SystemComponents.NodeLocalDNS.Deploy(ctx)
	}

	stillRequired := false
	var parsedVersion *semver.Version
	for _, workerPool := range workerPools {
		if workerPool.KubernetesVersion != nil {
			parsedVersion, err = semver.NewVersion(workerPool.KubernetesVersion.String())
			if err != nil {
				return fmt.Errorf("failed to parse Kubernetes version %q: %w", workerPool.KubernetesVersion.String(), err)
			}
		} else {
			parsedVersion, err = semver.NewVersion(b.Shoot.KubernetesVersion.String())
			if err != nil {
				return fmt.Errorf("failed to parse Kubernetes version %q: %w", b.Shoot.KubernetesVersion.String(), err)
			}
		}

		if parsedVersion.LessThan(semver.MustParse("1.34.0")) {
			stillRequired = true
		}

	}
	if stillDesired, err := b.isNodeLocalDNSStillDesired(ctx); err != nil {
		return err
	} else if stillDesired && stillRequired {
		// Leave NodeLocalDNS components in the cluster until all nodes have been rolled
		return nil
	}

	b.Logger.Info("NodeLocalDNS is disabled, removing NodeLocalDNS components")
	return b.Shoot.Components.SystemComponents.NodeLocalDNS.Destroy(ctx)
}

// isNodeLocalDNSStillDesired indicates whether any node still requires node-local-dns components.
func (b *Botanist) isNodeLocalDNSStillDesired(ctx context.Context) (bool, error) {
	return kubernetesutils.ResourcesExist(ctx, b.ShootClientSet.Client(), &corev1.NodeList{}, b.ShootClientSet.Client().Scheme(), client.MatchingLabels{
		v1beta1constants.LabelNodeLocalDNS: strconv.FormatBool(true),
	})
}

func (b *Botanist) computeWorkerPoolsForNodeLocalDNS(ctx context.Context) ([]nodelocaldns.WorkerPool, error) {
	poolKeyToPoolInfo := make(map[string]nodelocaldns.WorkerPool)

	for _, worker := range b.Shoot.GetInfo().Spec.Provider.Workers {
		kubernetesVersion, err := v1beta1helper.CalculateEffectiveKubernetesVersion(b.Shoot.KubernetesVersion, worker.Kubernetes)
		if err != nil {
			return nil, err
		}

		key := workerPoolKey(worker.Name, kubernetesVersion.String())
		poolKeyToPoolInfo[key] = nodelocaldns.WorkerPool{
			Name:              worker.Name,
			KubernetesVersion: kubernetesVersion,
		}
	}

	nodeList := &corev1.NodeList{}
	if err := b.ShootClientSet.Client().List(ctx, nodeList); err != nil {
		return nil, err
	}

	for _, node := range nodeList.Items {
		poolName, ok1 := node.Labels[v1beta1constants.LabelWorkerPool]
		kubernetesVersionString, ok2 := node.Labels[v1beta1constants.LabelWorkerKubernetesVersion]
		if !ok1 || !ok2 {
			continue
		}
		kubernetesVersion, err := semver.NewVersion(kubernetesVersionString)
		if err != nil {
			return nil, err
		}

		key := workerPoolKey(poolName, kubernetesVersionString)
		poolKeyToPoolInfo[key] = nodelocaldns.WorkerPool{
			Name:              poolName,
			KubernetesVersion: kubernetesVersion,
		}
	}

	var workerPools []nodelocaldns.WorkerPool
	for _, poolInfo := range poolKeyToPoolInfo {
		workerPools = append(workerPools, poolInfo)
	}

	return workerPools, nil
}

func (b *Botanist) SetShootClient(ctx context.Context) (kubernetes.Interface, error) {
	if b.ShootClientSet == nil {
		return nil, fmt.Errorf("ShootClientSet is nil")
	}
	return b.ShootClientSet, nil
}

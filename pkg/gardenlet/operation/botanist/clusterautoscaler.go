// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"fmt"
	"math"
	"math/big"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/imagevector"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component/autoscaling/clusterautoscaler"
	imagevectorutils "github.com/gardener/gardener/pkg/utils/imagevector"
)

// DefaultClusterAutoscaler returns a deployer for the cluster-autoscaler.
func (b *Botanist) DefaultClusterAutoscaler(ctx context.Context) (clusterautoscaler.Interface, error) {
	image, err := imagevector.Containers().FindImage(imagevector.ContainerImageNameClusterAutoscaler, imagevectorutils.RuntimeVersion(b.SeedVersion()), imagevectorutils.TargetVersion(b.ShootVersion()))
	if err != nil {
		return nil, err
	}

	var expanderConfigMap *corev1.ConfigMap = nil
	if b.Shoot.GetInfo().Spec.Kubernetes.ClusterAutoscaler != nil &&
		b.Shoot.GetInfo().Spec.Kubernetes.ClusterAutoscaler.ExpanderConfig != nil {
		err = b.GardenClient.Get(ctx, client.ObjectKey{Name: b.Shoot.GetInfo().Spec.Kubernetes.ClusterAutoscaler.ExpanderConfig.ConfigMapName, Namespace: b.Shoot.GetInfo().Namespace}, expanderConfigMap)
		if err != nil {
			return nil, err
		}
	}
	return clusterautoscaler.New(
		b.SeedClientSet.Client(),
		b.Shoot.SeedNamespace,
		b.SecretsManager,
		image.String(),
		b.Shoot.GetReplicas(1),
		b.Shoot.GetInfo().Spec.Kubernetes.ClusterAutoscaler,
		expanderConfigMap,
		0,
		b.Seed.KubernetesVersion,
	), nil
}

// DeployClusterAutoscaler deploys the Kubernetes cluster-autoscaler.
func (b *Botanist) DeployClusterAutoscaler(ctx context.Context) error {
	if b.Shoot.WantsClusterAutoscaler {
		b.Shoot.Components.ControlPlane.ClusterAutoscaler.SetNamespaceUID(b.SeedNamespaceObject.UID)
		b.Shoot.Components.ControlPlane.ClusterAutoscaler.SetMachineDeployments(b.Shoot.Components.Extensions.Worker.MachineDeployments())

		maxNodesTotal, err := b.CalculateMaxNodesForShoot(b.Shoot.GetInfo())
		if err != nil {
			return err
		}
		b.Shoot.Components.ControlPlane.ClusterAutoscaler.SetMaxNodesTotal(ptr.Deref(maxNodesTotal, 0))

		return b.Shoot.Components.ControlPlane.ClusterAutoscaler.Deploy(ctx)
	}

	return b.Shoot.Components.ControlPlane.ClusterAutoscaler.Destroy(ctx)
}

// ScaleClusterAutoscalerToZero scales cluster-autoscaler replicas to zero.
func (b *Botanist) ScaleClusterAutoscalerToZero(ctx context.Context) error {
	return client.IgnoreNotFound(kubernetes.ScaleDeployment(ctx, b.SeedClientSet.Client(), client.ObjectKey{Namespace: b.Shoot.SeedNamespace, Name: v1beta1constants.DeploymentNameClusterAutoscaler}, 0))
}

// CalculateMaxNodesForShoot returns the maximum number of nodes the shoot supports. Function returns nil if there is no limitation.
func (b *Botanist) CalculateMaxNodesForShoot(shoot *gardencorev1beta1.Shoot) (*int64, error) {
	if shoot.Spec.Networking == nil || len(b.Shoot.Networks.Pods) == 0 {
		return nil, nil
	}
	maxNodesForPodsNetwork, err := b.calculateMaxNodesForPodsNetwork(shoot)
	if err != nil {
		return nil, err
	}
	maxNodesForNodesNetwork, err := b.calculateMaxNodesForNodesNetwork(shoot)
	if err != nil {
		return nil, err
	}

	if maxNodesForPodsNetwork == nil {
		return maxNodesForNodesNetwork, nil
	}
	if maxNodesForNodesNetwork == nil {
		return maxNodesForPodsNetwork, nil
	}

	return ptr.To(min(*maxNodesForPodsNetwork, *maxNodesForNodesNetwork)), nil
}

func (b *Botanist) calculateMaxNodesForPodsNetwork(shoot *gardencorev1beta1.Shoot) (*int64, error) {
	resultPerIPFamily := map[gardencorev1beta1.IPFamily]int64{}
	for _, podNetwork := range b.Shoot.Networks.Pods {
		podCIDRMaskSize, _ := podNetwork.Mask.Size()
		if podCIDRMaskSize == 0 {
			return nil, fmt.Errorf("pod CIDR is not in its canonical form")
		}
		// Calculate how many subnets with nodeCIDRMaskSize can be allocated out of the pod network (with podCIDRMaskSize).
		// This indicates how many Nodes we can host at max from a networking perspective.
		var maxNodeCount = &big.Int{}
		exp := int64(*shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize) - int64(podCIDRMaskSize)
		// Bigger numbers than 2^62 do not fit into an int64 variable and big.Int{}.Int64() is undefined in such cases.
		// The pod network is no limitation in this case anyway.
		if exp > 62 {
			return nil, nil
		}
		maxNodeCount.Exp(big.NewInt(2), big.NewInt(exp), nil)

		if podNetwork.IP.To4() != nil {
			resultPerIPFamily[gardencorev1beta1.IPFamilyIPv4] += maxNodeCount.Int64()
		} else {
			resultPerIPFamily[gardencorev1beta1.IPFamilyIPv6] += maxNodeCount.Int64()
		}
	}

	// In a dual-stack scenario, return the minimum because beyond the minimum dual-stack is no longer possible.
	var result int64 = math.MaxInt64
	for _, value := range resultPerIPFamily {
		result = min(result, value)
	}
	return &result, nil
}

func (b *Botanist) calculateMaxNodesForNodesNetwork(shoot *gardencorev1beta1.Shoot) (*int64, error) {
	if len(b.Shoot.Networks.Nodes) == 0 {
		return nil, nil
	}

	resultPerIPFamily := map[gardencorev1beta1.IPFamily]int64{}
	for _, nodeNetwork := range b.Shoot.Networks.Nodes {
		nodeCIDRMaskSize, _ := nodeNetwork.Mask.Size()
		if nodeCIDRMaskSize == 0 {
			return nil, fmt.Errorf("node CIDR is not in its canonical form")
		}
		ipCIDRMaskSize := int64(32)
		if gardencorev1beta1.IsIPv6SingleStack(shoot.Spec.Networking.IPFamilies) {
			ipCIDRMaskSize = 128
		}
		// Calculate how many "single IP" subnets fit into the node network
		var maxNodeCount = &big.Int{}
		exp := ipCIDRMaskSize - int64(nodeCIDRMaskSize)
		// Bigger numbers than 2^62 do not fit into an int64 variable and big.Int{}.Int64() is undefined in such cases.
		// The node network is no limitation in this case anyway.
		if exp > 62 {
			return nil, nil
		}
		maxNodeCount.Exp(big.NewInt(2), big.NewInt(exp), nil)

		if nodeNetwork.IP.To4() != nil {
			// Subtract the broadcast addresses
			maxNodeCount.Sub(maxNodeCount, big.NewInt(2))
			resultPerIPFamily[gardencorev1beta1.IPFamilyIPv4] += maxNodeCount.Int64()
		} else {
			resultPerIPFamily[gardencorev1beta1.IPFamilyIPv6] += maxNodeCount.Int64()
		}
	}

	// In a dual-stack scenario, return the minimum because beyond the minimum dual-stack is no longer possible.
	var result int64 = math.MaxInt64
	for _, value := range resultPerIPFamily {
		result = min(result, value)
	}
	return &result, nil
}

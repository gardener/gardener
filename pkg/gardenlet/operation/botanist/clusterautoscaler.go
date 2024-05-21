// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"fmt"
	"math/big"
	"net"

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
func (b *Botanist) DefaultClusterAutoscaler() (clusterautoscaler.Interface, error) {
	image, err := imagevector.ImageVector().FindImage(imagevector.ImageNameClusterAutoscaler, imagevectorutils.RuntimeVersion(b.SeedVersion()), imagevectorutils.TargetVersion(b.ShootVersion()))
	if err != nil {
		return nil, err
	}

	maxNodesTotal, err := CalculateMaxNodesForShoot(b.Shoot.GetInfo())
	if err != nil {
		return nil, err
	}

	return clusterautoscaler.New(
		b.SeedClientSet.Client(),
		b.Shoot.SeedNamespace,
		b.SecretsManager,
		image.String(),
		b.Shoot.GetReplicas(1),
		b.Shoot.GetInfo().Spec.Kubernetes.ClusterAutoscaler,
		ptr.Deref(maxNodesTotal, 0),
		b.Seed.KubernetesVersion,
	), nil
}

// DeployClusterAutoscaler deploys the Kubernetes cluster-autoscaler.
func (b *Botanist) DeployClusterAutoscaler(ctx context.Context) error {
	if b.Shoot.WantsClusterAutoscaler {
		b.Shoot.Components.ControlPlane.ClusterAutoscaler.SetNamespaceUID(b.SeedNamespaceObject.UID)
		b.Shoot.Components.ControlPlane.ClusterAutoscaler.SetMachineDeployments(b.Shoot.Components.Extensions.Worker.MachineDeployments())

		return b.Shoot.Components.ControlPlane.ClusterAutoscaler.Deploy(ctx)
	}

	return b.Shoot.Components.ControlPlane.ClusterAutoscaler.Destroy(ctx)
}

// ScaleClusterAutoscalerToZero scales cluster-autoscaler replicas to zero.
func (b *Botanist) ScaleClusterAutoscalerToZero(ctx context.Context) error {
	return client.IgnoreNotFound(kubernetes.ScaleDeployment(ctx, b.SeedClientSet.Client(), client.ObjectKey{Namespace: b.Shoot.SeedNamespace, Name: v1beta1constants.DeploymentNameClusterAutoscaler}, 0))
}

// CalculateMaxNodesForShoot returns the maximum number of nodes the shoot supports. Function returns nil if there is no limitation.
func CalculateMaxNodesForShoot(shoot *gardencorev1beta1.Shoot) (*int64, error) {
	if shoot.Spec.Networking == nil || shoot.Spec.Networking.Pods == nil {
		return nil, nil
	}
	maxNodesForPodsNetwork, err := calculateMaxNodesForPodsNetwork(shoot)
	if err != nil {
		return nil, err
	}
	maxNodesForNodesNetwork, err := calculateMaxNodesForNodesNetwork(shoot)
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

func calculateMaxNodesForPodsNetwork(shoot *gardencorev1beta1.Shoot) (*int64, error) {
	_, podNetwork, err := net.ParseCIDR(*shoot.Spec.Networking.Pods)
	if err != nil {
		return nil, err
	}
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

	return ptr.To(maxNodeCount.Int64()), nil
}

func calculateMaxNodesForNodesNetwork(shoot *gardencorev1beta1.Shoot) (*int64, error) {
	if shoot.Spec.Networking.Nodes == nil {
		return nil, nil
	}
	_, nodeNetwork, err := net.ParseCIDR(*shoot.Spec.Networking.Nodes)
	if err != nil {
		return nil, err
	}
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
	// Subtract the broadcast addresses
	maxNodeCount.Sub(maxNodeCount, big.NewInt(2))

	return ptr.To(maxNodeCount.Int64()), nil
}

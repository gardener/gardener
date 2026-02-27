// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"context"
	"fmt"
	"math"
	"math/big"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/imagevector"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/component/autoscaling/clusterautoscaler"
	"github.com/gardener/gardener/pkg/utils"
	imagevectorutils "github.com/gardener/gardener/pkg/utils/imagevector"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// DefaultClusterAutoscaler returns a deployer for the cluster-autoscaler.
func (b *Botanist) DefaultClusterAutoscaler() (clusterautoscaler.Interface, error) {
	image, err := imagevector.Containers().FindImage(imagevector.ContainerImageNameClusterAutoscaler, imagevectorutils.RuntimeVersion(b.SeedVersion()), imagevectorutils.TargetVersion(b.ShootVersion()))
	if err != nil {
		return nil, err
	}

	return clusterautoscaler.New(
		b.SeedClientSet.Client(),
		b.Shoot.ControlPlaneNamespace,
		b.SecretsManager,
		image.String(),
		b.Shoot.GetReplicas(1),
		b.Shoot.GetInfo().Spec.Kubernetes.ClusterAutoscaler,
		b.Shoot.GetInfo().Spec.Provider.Workers,
		b.Seed.KubernetesVersion,
	), nil
}

// DeployClusterAutoscaler deploys the Kubernetes cluster-autoscaler.
func (b *Botanist) DeployClusterAutoscaler(ctx context.Context) error {
	if b.Shoot.WantsClusterAutoscaler {
		replicas, err := b.determineControllerReplicas(ctx, v1beta1constants.DeploymentNameClusterAutoscaler, 1)
		if err != nil {
			return err
		}
		b.Shoot.Components.ControlPlane.ClusterAutoscaler.SetReplicas(replicas)
		b.Shoot.Components.ControlPlane.ClusterAutoscaler.SetNamespaceUID(b.SeedNamespaceObject.UID)
		b.Shoot.Components.ControlPlane.ClusterAutoscaler.SetMachineDeployments(b.Shoot.Components.Extensions.Worker.MachineDeployments())

		maxNodesTotal, err := b.CalculateMaxNodesTotal(ctx, b.Shoot.GetInfo())
		if err != nil {
			return err
		}
		b.Shoot.Components.ControlPlane.ClusterAutoscaler.SetMaxNodesTotal(maxNodesTotal)

		return b.Shoot.Components.ControlPlane.ClusterAutoscaler.Deploy(ctx)
	}

	return b.Shoot.Components.ControlPlane.ClusterAutoscaler.Destroy(ctx)
}

// ScaleClusterAutoscalerToZero scales cluster-autoscaler replicas to zero.
func (b *Botanist) ScaleClusterAutoscalerToZero(ctx context.Context) error {
	return client.IgnoreNotFound(kubernetesutils.ScaleDeployment(ctx, b.SeedClientSet.Client(), client.ObjectKey{Namespace: b.Shoot.ControlPlaneNamespace, Name: v1beta1constants.DeploymentNameClusterAutoscaler}, 0))
}

// CalculateMaxNodesTotal returns the maximum number of nodes the shoot can have based on the shoot networks and
// the limit configured in the CloudProfile. It returns 0 if there is no limitation.
// If the current number of nodes exceeds the CloudProfile limit, the current count is used instead to
// prevent unintended forceful terminations due to limit decreases.
func (b *Botanist) CalculateMaxNodesTotal(ctx context.Context, shoot *gardencorev1beta1.Shoot) (int64, error) {
	maxNetworks, err := b.CalculateMaxNodesForShootNetworks(shoot)
	if err != nil {
		return 0, fmt.Errorf("failed to calculate max nodes for shoot networks: %w", err)
	}

	var maxLimit int64
	if limits := b.Shoot.CloudProfile.Spec.Limits; limits != nil && limits.MaxNodesTotal != nil {
		var machines metav1.PartialObjectMetadataList
		machines.SetGroupVersionKind(machinev1alpha1.SchemeGroupVersion.WithKind("MachineList"))
		if err := b.SeedClientSet.Client().List(ctx, &machines, client.InNamespace(b.Shoot.ControlPlaneNamespace)); err != nil {
			return 0, fmt.Errorf("failed listing machines: %w", err)
		}
		maxLimit = max(int64(len(machines.Items)), int64(*limits.MaxNodesTotal))
		b.Logger.Info("Setting cluster-autoscaler's maximum node limit based on CloudProfile limit and current machine count", "maxLimit", maxLimit, "cloudProfileLimit", *limits.MaxNodesTotal, "currentMachineCount", len(machines.Items), "maxNetworks", maxNetworks)
	}

	return utils.MinGreaterThanZero(maxNetworks, maxLimit), nil
}

// CalculateMaxNodesForShootNetworks returns the maximum number of nodes the shoot networks supports or 0 if there is no limitation.
func (b *Botanist) CalculateMaxNodesForShootNetworks(shoot *gardencorev1beta1.Shoot) (int64, error) {
	if shoot.Spec.Networking == nil || len(b.Shoot.Networks.Pods) == 0 {
		return 0, nil
	}
	maxNodesForPodsNetwork, err := b.calculateMaxNodesForPodsNetwork(shoot)
	if err != nil {
		return 0, err
	}
	maxNodesForNodesNetwork, err := b.calculateMaxNodesForNodesNetwork()
	if err != nil {
		return 0, err
	}

	return utils.MinGreaterThanZero(maxNodesForPodsNetwork, maxNodesForNodesNetwork), nil
}

func (b *Botanist) calculateMaxNodesForPodsNetwork(shoot *gardencorev1beta1.Shoot) (int64, error) {
	resultPerIPFamily := map[gardencorev1beta1.IPFamily]int64{}
	isDualStack := !gardencorev1beta1.IsIPv4SingleStack(shoot.Spec.Networking.IPFamilies) && !gardencorev1beta1.IsIPv6SingleStack(shoot.Spec.Networking.IPFamilies)

	for _, podNetwork := range b.Shoot.Networks.Pods {
		// In dual-stack scenarios, only consider IPv4
		// For IPv6 the actual effective nodeCIDRMaskSize is often dependent on the infrastructure. Thus, we ignore the pod network for IPv6 in dual-stack scenarios.
		// The limiting factor for the maximum number of nodes is IPv4 in general, because the ranges for IPv6 are much larger than for IPv4 and thus allows more nodes from a networking perspective.
		if isDualStack && podNetwork.IP.To4() == nil {
			continue
		}

		podCIDRMaskSize, _ := podNetwork.Mask.Size()
		if podCIDRMaskSize == 0 {
			return 0, fmt.Errorf("pod CIDR is not in its canonical form")
		}
		// Calculate how many subnets with nodeCIDRMaskSize can be allocated out of the pod network (with podCIDRMaskSize).
		// This indicates how many Nodes we can host at max from a networking perspective.
		var maxNodeCount = &big.Int{}
		var nodeCIDRMaskSize int64
		if podNetwork.IP.To4() != nil {
			nodeCIDRMaskSize = int64(*shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize)
		} else {
			if shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSizeIPv6 != nil {
				nodeCIDRMaskSize = int64(*shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSizeIPv6)
			} else if gardencorev1beta1.IsIPv6SingleStack(shoot.Spec.Networking.IPFamilies) && shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize != nil {
				// For single-stack IPv6, fall back to NodeCIDRMaskSize
				nodeCIDRMaskSize = int64(*shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize)
			}
		}
		exp := nodeCIDRMaskSize - int64(podCIDRMaskSize)
		// Bigger numbers than 2^62 do not fit into an int64 variable and big.Int{}.Int64() is undefined in such cases.
		// The pod network is no limitation in this case anyway.
		if exp > 62 {
			maxNodeCount = big.NewInt(math.MaxInt64)
		} else {
			maxNodeCount.Exp(big.NewInt(2), big.NewInt(exp), nil)
		}

		if podNetwork.IP.To4() != nil {
			resultPerIPFamily[gardencorev1beta1.IPFamilyIPv4] += maxNodeCount.Int64()
		} else {
			resultPerIPFamily[gardencorev1beta1.IPFamilyIPv6] += maxNodeCount.Int64()
		}
	}

	var result int64 = math.MaxInt64
	for _, value := range resultPerIPFamily {
		result = min(result, value)
	}
	return result, nil
}

func (b *Botanist) calculateMaxNodesForNodesNetwork() (int64, error) {
	if len(b.Shoot.Networks.Nodes) == 0 {
		return 0, nil
	}

	resultPerIPFamily := map[gardencorev1beta1.IPFamily]int64{}
	for _, nodeNetwork := range b.Shoot.Networks.Nodes {
		nodeCIDRMaskSize, _ := nodeNetwork.Mask.Size()
		if nodeCIDRMaskSize == 0 {
			return 0, fmt.Errorf("node CIDR is not in its canonical form")
		}
		ipCIDRMaskSize := int64(128)
		if nodeNetwork.IP.To4() != nil {
			ipCIDRMaskSize = int64(32)
		}
		// Calculate how many "single IP" subnets fit into the node network
		var maxNodeCount = &big.Int{}
		exp := ipCIDRMaskSize - int64(nodeCIDRMaskSize)
		// Bigger numbers than 2^62 do not fit into an int64 variable and big.Int{}.Int64() is undefined in such cases.
		// The node network is no limitation in this case anyway.
		if exp > 62 {
			maxNodeCount = big.NewInt(math.MaxInt64)
		} else {
			maxNodeCount.Exp(big.NewInt(2), big.NewInt(exp), nil)
		}

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
	return result, nil
}

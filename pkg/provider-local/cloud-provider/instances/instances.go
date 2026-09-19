// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package instances

import (
	"context"
	"fmt"
	"net/netip"
	"slices"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	cloudprovider "k8s.io/cloud-provider"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	cloudproviderv1alpha1 "github.com/gardener/gardener/pkg/provider-local/cloud-provider/api/v1alpha1"
	machineproviderv1alpha1 "github.com/gardener/gardener/pkg/provider-local/machine-provider/api/v1alpha1"
)

// Provider implements the cloudprovider.InstancesV2 interface for the cloud-provider-local.
// Shoot nodes are backed by machine pods running in the runtime cluster (the kind cluster). A node's "instance" is the
// machine pod with the same name in the configured runtime cluster namespace.
type Provider struct {
	// Config is the full config of cloud-controller-manager-local.
	Config *cloudproviderv1alpha1.CloudProviderConfig

	// RuntimeClient is a Kubernetes client for the runtime cluster (seed) of the shoot cluster, i.e., the kind cluster
	// where the shoot machine pods run.
	RuntimeClient client.Client
}

// InstanceExists returns true if the instance for the given node exists according to the cloud provider.
// Use the node.name or node.spec.providerID field to find the node in the cloud provider.
func (p *Provider) InstanceExists(ctx context.Context, node *corev1.Node) (bool, error) {
	if _, err := p.getMachinePod(ctx, node); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

// InstanceShutdown returns true if the instance is shutdown according to the cloud provider.
// Use the node.name or node.spec.providerID field to find the node in the cloud provider.
func (p *Provider) InstanceShutdown(ctx context.Context, node *corev1.Node) (bool, error) {
	pod, err := p.getMachinePod(ctx, node)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, cloudprovider.InstanceNotFound
		}
		return false, err
	}

	return isShutdown(pod), nil
}

// InstanceMetadata returns the instance's metadata. The values returned in InstanceMetadata are
// translated into specific fields and labels in the Node object on registration.
// Implementations should always check node.spec.providerID first when trying to discover the instance
// for a given node. In cases where node.spec.providerID is empty, implementations can use other
// properties of the node like its name, labels and annotations.
func (p *Provider) InstanceMetadata(ctx context.Context, node *corev1.Node) (*cloudprovider.InstanceMetadata, error) {
	pod, err := p.getMachinePod(ctx, node)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, cloudprovider.InstanceNotFound
		}
		return nil, err
	}

	klog.V(4).InfoS("Found machine pod for node", "node", node.Name, "pod", client.ObjectKeyFromObject(pod))

	return &cloudprovider.InstanceMetadata{
		// The provider ID must match what machine-controller-manager-provider-local reports for the machine, which is
		// the machine pod name. Both the machine's and the node's provider ID must be equal for cluster-autoscaler
		// to find the node group of a node.
		ProviderID:    pod.Name,
		InstanceType:  pod.Labels[machineproviderv1alpha1.LabelInstanceType],
		NodeAddresses: nodeAddresses(node, pod),
		Zone:          pod.Labels[machineproviderv1alpha1.LabelZone],
		Region:        pod.Labels[machineproviderv1alpha1.LabelRegion],
	}, nil
}

// getMachinePod returns the machine pod backing the given node. It returns a NotFound error if the pod does not exist
// or is not a machine pod.
func (p *Provider) getMachinePod(ctx context.Context, node *corev1.Node) (*corev1.Pod, error) {
	if p.Config == nil || p.Config.RuntimeCluster == nil {
		return nil, fmt.Errorf("runtime cluster is not configured")
	}

	// The machine provider uses the pod name as node name and provider ID, so we can look up the machine pod directly by
	// the node name (or the provider ID if set, both are equal).
	name := node.Name
	if node.Spec.ProviderID != "" {
		name = node.Spec.ProviderID
	}

	pod := &corev1.Pod{}
	if err := p.RuntimeClient.Get(ctx, client.ObjectKey{Namespace: p.Config.RuntimeCluster.Namespace, Name: name}, pod); err != nil {
		return nil, err
	}

	if pod.Labels["app"] != "machine" {
		return nil, apierrors.NewNotFound(corev1.Resource("pods"), name)
	}

	return pod, nil
}

// isShutdown returns true if the machine pod is terminating or has terminated.
func isShutdown(pod *corev1.Pod) bool {
	if pod.DeletionTimestamp != nil {
		return true
	}

	switch pod.Status.Phase {
	case corev1.PodSucceeded, corev1.PodFailed:
		return true
	}

	return false
}

// nodeAddresses returns the node addresses for the given machine pod. The pod IPs are reported as internal IPs,
// ordered such that the IP family of the node's current primary internal IP comes first. This preserves the IP family
// preference of the kubelet (e.g., configured via --node-ip="::" for IPv6-preferred shoots), because the order of the
// pod IPs is determined by the primary IP family of the runtime cluster, which may differ from the shoot's.
func nodeAddresses(node *corev1.Node, pod *corev1.Pod) []corev1.NodeAddress {
	var (
		preferIPv6 bool
		ips        = make([]netip.Addr, 0, len(pod.Status.PodIPs))
	)

	for _, address := range node.Status.Addresses {
		if address.Type != corev1.NodeInternalIP {
			continue
		}
		if ip, err := netip.ParseAddr(address.Address); err == nil {
			preferIPv6 = ip.Is6()
		}
		break
	}

	for _, podIP := range pod.Status.PodIPs {
		ip, err := netip.ParseAddr(podIP.IP)
		if err != nil {
			klog.ErrorS(err, "Ignoring invalid pod IP", "pod", client.ObjectKeyFromObject(pod), "ip", podIP.IP)
			continue
		}
		ips = append(ips, ip)
	}

	slices.SortStableFunc(ips, func(a, b netip.Addr) int {
		if a.Is6() == b.Is6() {
			return 0
		}
		if a.Is6() == preferIPv6 {
			return -1
		}
		return 1
	})

	addresses := make([]corev1.NodeAddress, 0, len(ips)+1)
	for _, ip := range ips {
		addresses = append(addresses, corev1.NodeAddress{Type: corev1.NodeInternalIP, Address: ip.String()})
	}
	if len(addresses) == 0 {
		// Without any IPs, we must not report the hostname only, otherwise the node controller would drop the internal
		// IPs determined by the kubelet.
		return nil
	}

	return append(addresses, corev1.NodeAddress{Type: corev1.NodeHostName, Address: pod.Name})
}

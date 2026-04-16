// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package loadbalancer

import (
	"context"
	"fmt"
	"strings"

	"github.com/containerd/errdefs"
	"github.com/docker/docker/api/types/container"
	dockerclient "github.com/docker/docker/client"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	localv1alpha1 "github.com/gardener/gardener/pkg/provider-local/apis/local/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
)

// Provider implements the cloudprovider.LoadBalancer interface for the cloud-provider-local.
type Provider struct {
	// Config is the full config of cloud-controller-manager-local.
	Config *localv1alpha1.CloudProviderConfig

	// DockerClient is the client connected to the host's Docker socket.
	DockerClient *dockerclient.Client
	// RuntimeClient is a Kubernetes client for the runtime cluster (seed) of the shoot cluster, i.e., the kind cluster
	// where the shoot machine pods run. This is only set if the cloud-controller-manager-local is running for a shoot
	// cluster, not for the kind cluster itself.
	RuntimeClient client.Client
}

// GetLoadBalancerName returns the name of the load balancer. Implementations must treat the *v1.Service parameter as
// read-only and not modify it.
func (p *Provider) GetLoadBalancerName(_ context.Context, _ string, service *corev1.Service) string {
	return "gardener-lb-" + utils.ComputeSHA256Hex([]byte(service.UID))[:8]
}

// GetLoadBalancer returns whether the specified load balancer exists, and if so, what its status is.
// Implementations must treat the *v1.Service parameter as read-only and not modify it.
// Parameter 'clusterName' is the name of the cluster as presented to cloud-controller-manager.
func (p *Provider) GetLoadBalancer(ctx context.Context, clusterName string, service *corev1.Service) (status *corev1.LoadBalancerStatus, exists bool, err error) {
	if status, isUnmanaged := p.getLoadBalancerStatusForUnmanagedInfra(service, clusterName); isUnmanaged {
		return status, true, nil
	}

	name := p.GetLoadBalancerName(ctx, clusterName, service)

	info, err := p.DockerClient.ContainerInspect(ctx, name)
	if err != nil {
		if errdefs.IsNotFound(err) {
			return nil, false, nil
		}

		return nil, false, fmt.Errorf("failed to inspect container %s: %w", name, err)
	}

	if !info.State.Running {
		// container exists but is not running, return "unhealthy" status
		return &corev1.LoadBalancerStatus{}, true, nil
	}

	// Only report the load balancer status once the container's Docker health check passes.
	// This prevents publishing the LB status before envoy is ready to serve traffic.
	if info.State.Health == nil || info.State.Health.Status != container.Healthy {
		return &corev1.LoadBalancerStatus{}, true, nil
	}

	loadBalancerStatus, err := getLoadBalancerStatusFromContainer(service, info.NetworkSettings)
	return loadBalancerStatus, true, err
}

// EnsureLoadBalancer creates a new load balancer 'name', or updates the existing one. Returns the status of the
// balancer. Implementations must treat the *v1.Service and []*v1.Node parameters as read-only and not modify them.
// Parameter 'clusterName' is the name of the cluster as presented to cloud-controller-manager.
//
// Implementations may return a (possibly wrapped) api.RetryError to enforce backing off at a fixed duration. This can
// be used for cases like when the load balancer is not ready yet (e.g., it is still being provisioned) and polling at a
// fixed rate is preferred over backing off exponentially in order to minimize latency.
func (p *Provider) EnsureLoadBalancer(ctx context.Context, clusterName string, service *corev1.Service, nodes []*corev1.Node) (*corev1.LoadBalancerStatus, error) {
	if status, isUnmanaged := p.getLoadBalancerStatusForUnmanagedInfra(service, clusterName); isUnmanaged {
		return status, nil
	}

	name := p.GetLoadBalancerName(ctx, clusterName, service)

	info, err := p.DockerClient.ContainerInspect(ctx, name)
	if err != nil && !errdefs.IsNotFound(err) {
		return nil, fmt.Errorf("failed to inspect container %s: %w", name, err)
	}

	var (
		containerExists = !errdefs.IsNotFound(err)
		needsCreation   = !containerExists
	)

	if containerExists {
		// Load balancer container already exists, check if it needs to be recreated.
		portsAreUpToDate, err := containerHasDesiredPortBindings(service, info.NetworkSettings)
		if err != nil {
			return nil, fmt.Errorf("failed to check if container %s has desired port bindings: %w", name, err)
		}

		if !portsAreUpToDate {
			klog.V(2).InfoS("Container has outdated port bindings, recreating container", "container", name, "service", client.ObjectKeyFromObject(service), "ports", info.NetworkSettings.Ports)
			// You can't dynamically update the port bindings of a container, so we need to recreate it if the service ports
			// have changed.
			if err := p.EnsureLoadBalancerDeleted(ctx, clusterName, service); err != nil {
				return nil, fmt.Errorf("failed to delete container %s for recreation due to port changes: %w", name, err)
			}

			needsCreation = true
		}
	}

	if needsCreation {
		klog.V(2).InfoS("Creating load balancer container", "container", name, "service", client.ObjectKeyFromObject(service))
		ips, err := p.createLoadBalancerContainer(ctx, name, service, clusterName)
		if err != nil {
			return nil, fmt.Errorf("failed to create container %s: %w", name, err)
		}
		klog.V(2).InfoS("Allocated load balancer IPs",
			"container", name,
			"service", client.ObjectKeyFromObject(service),
			"internalIPv4", ipStringOrZero(ips.internal.ipv4),
			"internalIPv6", ipStringOrZero(ips.internal.ipv6),
			"externalIPv4", ipStringOrZero(ips.external.ipv4),
			"externalIPv6", ipStringOrZero(ips.external.ipv6),
		)

		// Write the static envoy bootstrap config only on container creation.
		if err := p.writeEnvoyStaticConfig(ctx, name); err != nil {
			return nil, fmt.Errorf("failed to write bootstrap config for container %s: %w", name, err)
		}
	}

	// Write the dynamic listener and cluster discovery configs (lds.yaml, cds.yaml, i.e., the LoadBalancer ports and
	// backends). Envoy watches these files via path_config_source (inotify) and reloads automatically.
	if err := p.writeEnvoyDynamicConfig(ctx, name, service, nodes); err != nil {
		return nil, fmt.Errorf("failed to write config for container %s: %w", name, err)
	}

	if err := p.DockerClient.ContainerStart(ctx, name, container.StartOptions{}); err != nil {
		// When starting new load balancer containers, we instruct Docker to use the next free IP address in the internal
		// load balancer range (the last addresses of kind network range) as the container IP. Together with the mapping
		// to the external IP range, this is used as the IPAM for load balancer IPs (see ipam.go).
		// However, when creating multiple load balancers concurrently, we might try to start multiple containers with the
		// internal IP. Docker refuses to start containers with IPs that are already allocated in the same network and
		// returns an error like:
		//   failed to set up container networking: Address already in use
		// The container with the conflicting IP is automatically deleted by Docker.
		// If this happens, it means we also tried to use an external IP for the new load balancer that has already been
		// allocated. To handle the IP conflict, we simply need to retry (with exponential backoff) until we read all
		// existing allocated IPs and use the next free IP instead.
		// We return an explicit error message to make this situation visible in the logs.
		if strings.Contains(err.Error(), "Address already in use") {
			return nil, fmt.Errorf("recreating container %s due to IP conflict: %w", name, err)
		}

		return nil, fmt.Errorf("failed to ensure container %s is started: %w", name, err)
	}

	// IP routes must be added after the container is started (docker exec requires a running container).
	if err := p.ensureIPRoutes(ctx, name, nodes); err != nil {
		return nil, err
	}

	// We need to inspect the container again to ensure the desired external addresses were successfully assigned before
	// adding them to the Service status.
	info, err = p.DockerClient.ContainerInspect(ctx, name)
	if err != nil && !errdefs.IsNotFound(err) {
		return nil, fmt.Errorf("failed to inspect container %s: %w", name, err)
	}

	// Only publish the LB status once Docker's health check confirms envoy is ready.
	if info.State.Health == nil {
		return nil, fmt.Errorf("envoy proxy container %s is missing health status", name)
	}
	if info.State.Health.Status != container.Healthy {
		return nil, fmt.Errorf("envoy proxy container %s is in status: %s", name, info.State.Health.Status)
	}

	return getLoadBalancerStatusFromContainer(service, info.NetworkSettings)
}

// UpdateLoadBalancer updates hosts under the specified load balancer.
// Implementations must treat the *v1.Service and []*v1.Node parameters as read-only and not modify them.
// Parameter 'clusterName' is the name of the cluster as presented to cloud-controller-manager
func (p *Provider) UpdateLoadBalancer(ctx context.Context, clusterName string, service *corev1.Service, nodes []*corev1.Node) error {
	if _, isUnmanaged := p.getLoadBalancerStatusForUnmanagedInfra(service, clusterName); isUnmanaged {
		return nil
	}

	name := p.GetLoadBalancerName(ctx, clusterName, service)

	klog.V(2).InfoS("Updating load balancer", "container", name, "service", client.ObjectKeyFromObject(service), "nodes", len(nodes))
	if err := p.writeEnvoyDynamicConfig(ctx, name, service, nodes); err != nil {
		return fmt.Errorf("failed to write config for container %s: %w", name, err)
	}

	if err := p.ensureIPRoutes(ctx, name, nodes); err != nil {
		return err
	}

	return nil
}

// EnsureLoadBalancerDeleted deletes the specified load balancer if it exists, returning nil if the load balancer
// specified either didn't exist or was successfully deleted.
// This construction is useful because many cloud providers' load balancers have multiple underlying components, meaning
// a Get could say that the LB doesn't exist even if some part of it is still laying around.
// Implementations must treat the *v1.Service parameter as read-only and not modify it.
// Parameter 'clusterName' is the name of the cluster as presented to cloud-controller-manager
func (p *Provider) EnsureLoadBalancerDeleted(ctx context.Context, clusterName string, service *corev1.Service) error {
	if _, isUnmanaged := p.getLoadBalancerStatusForUnmanagedInfra(service, clusterName); isUnmanaged {
		return nil
	}

	name := p.GetLoadBalancerName(ctx, clusterName, service)

	klog.V(2).InfoS("Deleting load balancer container", "container", name, "service", client.ObjectKeyFromObject(service))
	if err := p.DockerClient.ContainerRemove(ctx, name, container.RemoveOptions{Force: true}); err != nil && !errdefs.IsNotFound(err) {
		return fmt.Errorf("failed to remove container %s: %w", name, err)
	}

	return nil
}

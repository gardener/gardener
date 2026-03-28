// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package loadbalancer

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"slices"
	"strings"
	"time"

	"github.com/containerd/errdefs"
	"github.com/docker/docker/api/types/container"
	imagetypes "github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	dockerclient "github.com/docker/docker/client"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/cloud-provider/api"
	"sigs.k8s.io/controller-runtime/pkg/client"

	localv1alpha1 "github.com/gardener/gardener/pkg/provider-local/apis/local/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
)

const (
	labelRoleKey     = "gardener.cloud/role"
	labelRoleValue   = "loadbalancer"
	labelSelector    = labelRoleKey + "=" + labelRoleValue
	labelServiceKey  = "gardener.cloud/service"
	labelClusterName = "kubernetes.io/cluster"
	defaultNetwork   = "kind"
	// TODO: this can conflict with load balancer ports
	envoyAdminPort     = 10000
	envoyReadyInterval = time.Second
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
	name := p.GetLoadBalancerName(ctx, clusterName, service)

	info, err := p.DockerClient.ContainerInspect(ctx, name)
	if err != nil {
		if errdefs.IsNotFound(err) {
			return nil, false, nil
		}

		return nil, false, fmt.Errorf("failed to inspect container %s: %w", name, err)
	}

	if !info.State.Running {
		// TODO: container exists but is not running -> handle?
		return nil, true, nil
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
	name := p.GetLoadBalancerName(ctx, clusterName, service)

	info, err := p.DockerClient.ContainerInspect(ctx, name)
	if err != nil && !errdefs.IsNotFound(err) {
		return nil, fmt.Errorf("failed to inspect container %s: %w", name, err)
	}

	var (
		ips             *loadBalancerIPs
		containerExists = !errdefs.IsNotFound(err)
		needsCreation   = !containerExists
	)

	if containerExists {
		// Load balancer container already exists, check if it needs to be recreated.
		portsAreUpToDate, err := containerHasDesiredPortBindings(service, info.NetworkSettings)
		if err != nil {
			return nil, fmt.Errorf("failed to check if container %s has desired port bindings: %w", name, err)
		}

		if portsAreUpToDate {
			// Container exists and has the correct port bindings, so we just get the assigned IPs for the status.
			ips, err = getLoadBalancerIPsFromContainer(info.NetworkSettings)
			if err != nil {
				return nil, fmt.Errorf("failed to get external IPs for container %s: %w", name, err)
			}
		} else {
			// You can't dynamically update the port bindings of a container, so we need to recreate it if the service ports
			// have changed.
			if err := p.EnsureLoadBalancerDeleted(ctx, name, service); err != nil {
				return nil, fmt.Errorf("failed to recreate container %s due to port changes: %w", name, err)
			}

			needsCreation = true
		}
	}

	if needsCreation {
		// Load balancer container does not exist, create it
		ips, err = p.createLoadBalancerContainer(ctx, name, service, clusterName)
		if err != nil {
			return nil, fmt.Errorf("failed to create container %s: %w", name, err)
		}
	}

	// TODO: reduce unnecessary writes of the config
	// TODO: if updating the config, we would ideally check if envoy has reloaded the config
	if err := p.writeLoadBalancerConfig(ctx, name, service, nodes); err != nil {
		return nil, fmt.Errorf("failed to write config for container %s: %w", name, err)
	}

	if err := p.DockerClient.ContainerStart(ctx, name, container.StartOptions{}); err != nil {
		// If we allocate multiple IPs concurrently, we can run into conflicts (between multiple services or even between
		// multiple cloud-controller-manager-local instances) where we try to start multiple containers with the same IPs:
		//   failed to set up container networking: Address already in use
		// If this happens, Docker refuses to start another container with the same IP in the kind network and deletes the
		// container. We simply retry the IP allocation with a short retry interval.
		if strings.Contains(err.Error(), "Address already in use") {
			return nil, api.NewRetryError(fmt.Sprintf("recreating container %s due to IP conflict: %v", name, err), time.Second)
		}

		return nil, fmt.Errorf("failed to ensure container %s is started: %w", name, err)
	}

	// IP routes must be added after the container is started (docker exec requires a running container).
	if err := p.ensureIPRoutes(ctx, name, nodes); err != nil {
		return nil, err
	}

	// TODO: on creation, the first invocation fails immediately with "connection refused"
	if err := p.healthCheckContainer(ctx, name, ips.internal.PreferredAddr(service.Spec.IPFamilies)); err != nil {
		return nil, err
	}

	// We need to inspect the container again to ensure the desired external addresses were successfully assigned before
	// adding them to the Service status.
	info, err = p.DockerClient.ContainerInspect(ctx, name)
	if err != nil && !errdefs.IsNotFound(err) {
		return nil, fmt.Errorf("failed to inspect container %s: %w", name, err)
	}

	return getLoadBalancerStatusFromContainer(service, info.NetworkSettings)
}

// UpdateLoadBalancer updates hosts under the specified load balancer.
// Implementations must treat the *v1.Service and []*v1.Node parameters as read-only and not modify them.
// Parameter 'clusterName' is the name of the cluster as presented to cloud-controller-manager
func (p *Provider) UpdateLoadBalancer(ctx context.Context, clusterName string, service *corev1.Service, nodes []*corev1.Node) error {
	name := p.GetLoadBalancerName(ctx, clusterName, service)

	if err := p.writeLoadBalancerConfig(ctx, name, service, nodes); err != nil {
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
	name := p.GetLoadBalancerName(ctx, clusterName, service)

	if err := p.DockerClient.ContainerRemove(ctx, name, container.RemoveOptions{Force: true}); err != nil && !errdefs.IsNotFound(err) {
		return fmt.Errorf("failed to remove container %s: %w", name, err)
	}

	return nil
}

func (p *Provider) ensureImage(ctx context.Context, image string) error {
	_, err := p.DockerClient.ImageInspect(ctx, image)
	if err == nil {
		return nil
	}

	reader, err := p.DockerClient.ImagePull(ctx, image, imagetypes.PullOptions{})
	if err != nil {
		return fmt.Errorf("failed to pull image %s: %w", image, err)
	}
	defer reader.Close()

	// Drain the reader to complete the pull
	_, _ = io.Copy(io.Discard, reader)
	return nil
}

func (p *Provider) createLoadBalancerContainer(ctx context.Context, name string, service *corev1.Service, clusterName string) (*loadBalancerIPs, error) {
	if err := p.ensureImage(ctx, p.Config.LoadBalancer.Image); err != nil {
		return nil, err
	}

	ips, err := p.allocateLoadBalancerIPs(ctx, service)
	if err != nil {
		return nil, fmt.Errorf("failed to allocate IPs for load balancer: %w", err)
	}

	portBindings, err := portBindingsForService(service, ips.external.AsSlice())
	if err != nil {
		return nil, err
	}

	_, err = p.DockerClient.ContainerCreate(
		ctx,
		&container.Config{
			Hostname: name,
			Image:    p.Config.LoadBalancer.Image,
			Cmd:      []string{"envoy", "-c", "/home/envoy/envoy.yaml"},
			Labels: map[string]string{
				labelRoleKey:     labelRoleValue,
				labelServiceKey:  client.ObjectKeyFromObject(service).String(),
				labelClusterName: clusterName,
			},
		},
		&container.HostConfig{
			PortBindings:  portBindings,
			Privileged:    true,
			RestartPolicy: container.RestartPolicy{Name: container.RestartPolicyOnFailure},
			Sysctls:       map[string]string{"net.ipv4.ip_forward": "1"},
		},
		&network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{
				defaultNetwork: {
					IPAMConfig: &network.EndpointIPAMConfig{
						IPv4Address: ipStringOrZero(ips.internal.ipv4),
						IPv6Address: ipStringOrZero(ips.internal.ipv6),
					},
				},
			},
		},
		nil,
		name,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create container %s: %w", name, err)
	}

	return ips, nil
}

func (p *Provider) writeLoadBalancerConfig(ctx context.Context, name string, service *corev1.Service, nodes []*corev1.Node) error {
	ldsConfig, cdsConfig, err := generateProxyConfig(service, nodes)
	if err != nil {
		return fmt.Errorf("failed to generate proxy config: %w", err)
	}

	if err := p.copyFilesToContainer(ctx, name, "/home/envoy/", map[string][]byte{
		"envoy.yaml": []byte(dynamicFilesystemConfig),
		"lds.yaml":   ldsConfig,
		"cds.yaml":   cdsConfig,
	}); err != nil {
		return fmt.Errorf("failed to seed config in %s: %w", name, err)
	}

	return nil
}

func (p *Provider) copyFilesToContainer(ctx context.Context, containerID, destDir string, files map[string][]byte) error {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for name, content := range files {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(content))}); err != nil {
			return err
		}
		if _, err := tw.Write(content); err != nil {
			return err
		}
	}
	if err := tw.Close(); err != nil {
		return err
	}
	return p.DockerClient.CopyToContainer(ctx, containerID, destDir, &buf, container.CopyToContainerOptions{})
}

// ensureIPRoutes adds IP routes inside the envoy container so it can reach shoot node IPs.
// Shoot nodes are pods inside the kind cluster with pod IPs (e.g., 10.0.0.0/16).
// The envoy container runs in the Docker kind network and can't reach pod IPs directly.
// We add routes so that traffic for shoot node IPs is routed via the kind node where the machine pod runs.
// For this, we look up the kind node of the respective machine pods using the runtime client.
// This is not needed if cloud-controller-manager-local runs for the kind cluster itself, as the kind node IPs are
// directly reachable within the kind network.
func (p *Provider) ensureIPRoutes(ctx context.Context, containerName string, nodes []*corev1.Node) error {
	if p.Config.RuntimeCluster == nil {
		return nil
	}

	machinePods := &corev1.PodList{}
	if err := p.RuntimeClient.List(ctx, machinePods, client.InNamespace(p.Config.RuntimeCluster.Namespace), client.MatchingLabels{
		"app": "machine",
	}); err != nil {
		return fmt.Errorf("failed to list machine pods: %w", err)
	}

	shootNodeToGateway := make(map[netip.Addr]netip.Addr)
	for _, shootNode := range nodes {
		// find the machine pod matching the node name
		podIndex := slices.IndexFunc(machinePods.Items, func(pod corev1.Pod) bool { return pod.Name == shootNode.Name })
		if podIndex < 0 {
			continue
		}
		machinePod := machinePods.Items[podIndex]

		// get the node's internal IPs (machine pod IPs)
		shootNodeIPs, err := getInternalNodeIPs(shootNode)
		if err != nil {
			return fmt.Errorf("could not get internal IPs of node %s: %w", shootNode.Name, err)
		}

		// get the IPs of the kind node running the machine pod (container IPs in the kind network)
		kindNodeIPs, err := getHostIPs(&machinePod)
		if err != nil {
			return fmt.Errorf("could not get host IPs of pod %s: %w", machinePod.Name, err)
		}
		if kindNodeIPs.Len() == 0 {
			continue
		}

		// put together a routing table: shoot node IP (machine pod IP) via seed node IP (kind node container IP)
		if shootNodeIPs.ipv4.IsValid() && kindNodeIPs.ipv4.IsValid() {
			shootNodeToGateway[shootNodeIPs.ipv4] = kindNodeIPs.ipv4
		}
		if shootNodeIPs.ipv6.IsValid() && kindNodeIPs.ipv6.IsValid() {
			shootNodeToGateway[shootNodeIPs.ipv6] = kindNodeIPs.ipv6
		}
	}

	return p.writeIPRoutes(ctx, containerName, shootNodeToGateway)
}

func (p *Provider) writeIPRoutes(ctx context.Context, containerName string, nodeToGateway map[netip.Addr]netip.Addr) error {
	for nodeIP, gatewayIP := range nodeToGateway {
		cmd := []string{"ip", "route", "replace", nodeIP.String() + "/32", "via", gatewayIP.String()}

		if nodeIP.Is6() {
			cmd = append(cmd[:1], append([]string{"-6"}, cmd[1:]...)...)
		}

		// ip route replace <shoot-node-ip>/32 via <kind-node-ip>
		// Using "replace" instead of "add" to be idempotent.
		exec, err := p.DockerClient.ContainerExecCreate(ctx, containerName, container.ExecOptions{
			Cmd: cmd,
		})
		if err != nil {
			return fmt.Errorf("failed to create exec for ip route in container %s: %w", containerName, err)
		}

		if err := p.DockerClient.ContainerExecStart(ctx, exec.ID, container.ExecStartOptions{}); err != nil {
			return fmt.Errorf("failed to exec ip route in container %s: %w", containerName, err)
		}
	}

	return nil
}

func (p *Provider) healthCheckContainer(ctx context.Context, name string, internalIP netip.Addr) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://%s:%d/ready", internalIP, envoyAdminPort), nil)
	if err != nil {
		return fmt.Errorf("failed to create request to check readiness of container %s: %w", name, err)
	}

	resp, err := http.DefaultClient.Do(request)
	if err != nil {
		return api.NewRetryError(fmt.Sprintf("failed to check readiness of container %s: %v", name, err), envoyReadyInterval)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return api.NewRetryError(fmt.Sprintf("envoy container %s is not ready, expected status code 200 but got %d", name, resp.StatusCode), envoyReadyInterval)
	}

	return nil
}

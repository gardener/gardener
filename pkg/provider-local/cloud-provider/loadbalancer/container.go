// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package loadbalancer

import (
	"context"
	"fmt"
	"io"
	"maps"
	"net/netip"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const kindNetwork = "kind"

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
			Cmd:      []string{"envoy", "-c", envoyConfigFilePath},
			Healthcheck: &container.HealthConfig{
				Test:     []string{"CMD", "curl", "-sf", "--unix-socket", envoyAdminSocket, "http://localhost/ready"},
				Interval: time.Second,
				Timeout:  time.Second,
				Retries:  3,
			},
			Labels: map[string]string{
				"gardener.cloud/role":    "loadbalancer",
				"gardener.cloud/service": client.ObjectKeyFromObject(service).String(),
				"kubernetes.io/cluster":  clusterName,
			},
		},
		&container.HostConfig{
			PortBindings:  portBindings,
			NetworkMode:   kindNetwork,
			Privileged:    true,
			RestartPolicy: container.RestartPolicy{Name: container.RestartPolicyOnFailure},
			Sysctls: map[string]string{
				// explicitly enable IPv4 forwarding for all interfaces by default if not enabled by the OS image already
				"net.ipv4.conf.all.forwarding":     "1",
				"net.ipv4.conf.default.forwarding": "1",
				// explicitly enable IPv6 forwarding for all interfaces by default if not enabled by the OS image already
				"net.ipv6.conf.all.forwarding":     "1",
				"net.ipv6.conf.default.forwarding": "1",
			},
		},
		&network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{
				kindNetwork: {
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

func (p *Provider) ensureImage(ctx context.Context, imageName string) error {
	_, err := p.DockerClient.ImageInspect(ctx, imageName)
	if err == nil {
		return nil
	}

	reader, err := p.DockerClient.ImagePull(ctx, imageName, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("failed to pull image %s: %w", imageName, err)
	}
	defer reader.Close()

	// Drain the reader to complete the pull
	_, _ = io.Copy(io.Discard, reader)
	return nil
}

// ensureIPRoutes adds IP routes inside the envoy container so it can reach shoot node IPs.
// Shoot nodes are pods inside the kind cluster with pod IPs (e.g., 10.0.0.0/16).
// The envoy container runs in the Docker kind network and can't reach pod IPs directly.
// We add routes so that traffic for shoot node IPs is routed via the kind node where the machine pod runs.
// For this, we look up the kind node of the respective machine pods using the runtime client.
// This is not needed if cloud-controller-manager-local runs for the kind cluster itself, as the kind node IPs are
// directly reachable within the kind network.
func (p *Provider) ensureIPRoutes(ctx context.Context, containerName string, nodes []*corev1.Node) error {
	if p.RuntimeClient == nil {
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

	klog.V(2).InfoS("Adding IP routes in load balancer container for shoot node IPs via kind node gateways", "container", containerName, "routes", shootNodeToGateway)
	return p.writeIPRoutes(ctx, containerName, shootNodeToGateway)
}

func (p *Provider) writeIPRoutes(ctx context.Context, containerName string, nodeToGateway map[netip.Addr]netip.Addr) error {
	klog.V(2).InfoS("Ensuring IP routes in load balancer container", "container", containerName, "routes", len(nodeToGateway))
	for nodeIP, gatewayIP := range nodeToGateway {
		cmd := []string{"ip"}
		if nodeIP.Is6() {
			cmd = append(cmd, "-6")
		}

		cmd = append(cmd, "route", "replace")
		if nodeIP.Is4() {
			cmd = append(cmd, nodeIP.String()+"/32")
		} else {
			cmd = append(cmd, nodeIP.String()+"/128")
		}
		cmd = append(cmd, "via", gatewayIP.String())

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

func getLoadBalancerStatusFromContainer(service *corev1.Service, networkSettings *container.NetworkSettings) (*corev1.LoadBalancerStatus, error) {
	ips, err := getLoadBalancerIPsFromContainer(networkSettings)
	if err != nil {
		return nil, fmt.Errorf("failed to get external IPs for container: %w", err)
	}

	return loadBalancerStatusWithIPs(service, ips.external, corev1.LoadBalancerIPModeProxy), nil
}

func getLoadBalancerIPsFromContainer(networkSettings *container.NetworkSettings) (*loadBalancerIPs, error) {
	if networkSettings == nil {
		return nil, fmt.Errorf("missing network settings")
	}

	containerIPs, ok := networkSettings.Networks[kindNetwork]
	if !ok {
		return nil, fmt.Errorf("container is missing network settings for network %s", kindNetwork)
	}

	ips := new(loadBalancerIPs)

	if containerIPs.IPAddress != "" {
		parsedIPv4, err := netip.ParseAddr(containerIPs.IPAddress)
		if err != nil {
			return nil, fmt.Errorf("could not parse internal IPv4 address %q: %w", containerIPs.IPAddress, err)
		}
		ips.internal.ipv4 = parsedIPv4
	}
	if containerIPs.GlobalIPv6Address != "" {
		parsedIPv6, err := netip.ParseAddr(containerIPs.GlobalIPv6Address)
		if err != nil {
			return nil, fmt.Errorf("could not parse internal IPv6 address %q: %w", containerIPs.GlobalIPv6Address, err)
		}
		ips.internal.ipv6 = parsedIPv6
	}

	for _, bindings := range networkSettings.Ports {
		for _, binding := range bindings {
			parsedIP, err := netip.ParseAddr(binding.HostIP)
			if err != nil {
				return nil, fmt.Errorf("could not parse IP %q: %w", binding.HostIP, err)
			}

			if parsedIP.Is4() {
				if ips.external.ipv4.IsValid() && ips.external.ipv4 != parsedIP {
					return nil, fmt.Errorf("container has multiple external IPv4 addresses: %s and %s", ips.external.ipv4, parsedIP)
				}
				ips.external.ipv4 = parsedIP
			} else {
				if ips.external.ipv6.IsValid() && ips.external.ipv6 != parsedIP {
					return nil, fmt.Errorf("container has multiple external IPv6 addresses: %s and %s", ips.external.ipv6, parsedIP)
				}
				ips.external.ipv6 = parsedIP
			}
		}
	}

	return ips, nil
}

func loadBalancerStatusWithIPs(service *corev1.Service, externalIPs ipSet, ipMode corev1.LoadBalancerIPMode) *corev1.LoadBalancerStatus {
	ingresses := make([]corev1.LoadBalancerIngress, 0, externalIPs.Len())

	if slices.Contains(service.Spec.IPFamilies, corev1.IPv4Protocol) {
		ingresses = append(ingresses, corev1.LoadBalancerIngress{
			IP:     externalIPs.ipv4.String(),
			IPMode: ptr.To(ipMode),
		})
	}

	if slices.Contains(service.Spec.IPFamilies, corev1.IPv6Protocol) {
		ingresses = append(ingresses, corev1.LoadBalancerIngress{
			IP:     externalIPs.ipv6.String(),
			IPMode: ptr.To(ipMode),
		})
	}

	return &corev1.LoadBalancerStatus{
		Ingress: ingresses,
	}
}

func portBindingsForService(service *corev1.Service, externalIPs []netip.Addr) (nat.PortMap, error) {
	portMap := nat.PortMap{}
	for _, port := range service.Spec.Ports {
		supportedProtocols := []corev1.Protocol{corev1.ProtocolTCP, corev1.ProtocolUDP}
		if !slices.Contains(supportedProtocols, port.Protocol) {
			return nil, fmt.Errorf("unsupported protocol %s for port %s, expected on of: %v", port.Protocol, debugPortName(port), supportedProtocols)
		}

		portBindings := make([]nat.PortBinding, len(externalIPs))
		for i, ip := range externalIPs {
			portBindings[i] = nat.PortBinding{
				HostIP:   ip.String(),
				HostPort: strconv.FormatInt(int64(port.Port), 10),
			}
		}

		containerPort := nat.Port(fmt.Sprintf("%d/%s", port.Port, strings.ToLower(string(port.Protocol))))
		portMap[containerPort] = portBindings
	}

	return portMap, nil
}

// containerHasDesiredPortBindings checks if the given container has successfully applied the port bindings
// corresponding to the service ports without checking the external IPs. I.e., it calculates the list of desired
// container ports for the service and checks if the container's network settings have bindings for those ports with
// host IPs (ignoring the host IP value).
func containerHasDesiredPortBindings(service *corev1.Service, networkSettings *container.NetworkSettings) (bool, error) {
	desiredPortBindings, err := portBindingsForService(service, nil)
	if err != nil {
		return false, err
	}
	desiredPorts := slices.Sorted(maps.Keys(desiredPortBindings))

	actualPorts := make([]nat.Port, 0, len(networkSettings.Ports))
	for actualPort, portBindings := range networkSettings.Ports {
		if len(portBindings) != len(service.Spec.IPFamilies) {
			// For each IPFamily, we expect one host port binding. If the host port binding did not succeed (e.g., because
			// the host IP is already allocated) the port binding value will be missing in the container's network settings.
			// That's why we use the network settings instead of the HostConfig (which would still contain the desired port
			// bindings even if they were not successfully applied to the container) to check if the container has the desired
			// port bindings.
			// Also, the envoy image specifies `EXPOSE` for the `10000/tcp` port in its Dockerfile. With this, the ports in
			// the network settings might contain `10000/tcp` with a `null` value (depending on the docker daemon).
			// We ignore such ports because they are not published on the host IP and are irrelevant for the load balancer.
			continue
		}

		actualPorts = append(actualPorts, actualPort)
	}
	slices.Sort(actualPorts)

	return slices.Equal(desiredPorts, actualPorts), nil
}

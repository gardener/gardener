// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package loadbalancer

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"net/netip"
	"text/template"

	"github.com/docker/docker/api/types/container"
	corev1 "k8s.io/api/core/v1"
)

const (
	envoyDir = "/home/envoy/"

	envoyConfigFileName = "envoy.yaml"
	envoyConfigFilePath = envoyDir + envoyConfigFileName

	ldsConfigFileName = "lds.yaml"
	ldsConfigFilePath = envoyDir + ldsConfigFileName

	cdsConfigFileName = "cds.yaml"
	cdsConfigFilePath = envoyDir + cdsConfigFileName

	// envoyAdminSocket is the path to the unix domain socket in the load balancer container where Envoy listens for admin
	// API requests. We don't expose the admin API on a port as usual to prevent blocking load balancer ports.
	// For now, we only use the admin API for health checks (configured in the container).
	envoyAdminSocket = envoyDir + "admin.sock"
)

func (p *Provider) writeEnvoyStaticConfig(ctx context.Context, name string) error {
	if err := p.copyFilesToContainer(ctx, name, envoyDir, map[string][]byte{
		envoyConfigFileName: []byte(envoyConfig),
	}); err != nil {
		return fmt.Errorf("failed to write static envoy config to %s: %w", name, err)
	}
	return nil
}

func (p *Provider) writeEnvoyDynamicConfig(ctx context.Context, name string, service *corev1.Service, nodes []*corev1.Node) error {
	ldsConfig, cdsConfig, err := generateEnvoyDynamicConfig(service, nodes)
	if err != nil {
		return fmt.Errorf("failed to generate dynamic envoy config for %s: %w", name, err)
	}

	if err := p.copyFilesToContainer(ctx, name, envoyDir, map[string][]byte{
		ldsConfigFileName: ldsConfig,
		cdsConfigFileName: cdsConfig,
	}); err != nil {
		return fmt.Errorf("failed to write dynamic envoy config to %s: %w", name, err)
	}

	return nil
}

func (p *Provider) copyFilesToContainer(ctx context.Context, containerID, destDir string, files map[string][]byte) error {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	defer tw.Close()

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

const envoyConfig = `node:
  cluster: cloud-controller-manager-local
  id: cloud-controller-manager-local-id
dynamic_resources:
  cds_config:
    resource_api_version: V3
    path_config_source:
      path: ` + cdsConfigFilePath + `
  lds_config:
    resource_api_version: V3
    path_config_source:
      path: ` + ldsConfigFilePath + `
admin:
  access_log:
  - name: envoy.access_loggers.file
    typed_config:
      "@type": type.googleapis.com/envoy.extensions.access_loggers.file.v3.FileAccessLog
      path: /dev/stdout
    filter:
      header_filter:
        header:
          name: ":path"
          exact_match: "/ready"
          invert_match: true
  address:
    pipe:
      path: ` + envoyAdminSocket + `
`

const ldsTemplate = `resources:
{{- range $key, $port := .ServicePorts }}
- "@type": type.googleapis.com/envoy.config.listener.v3.Listener
  name: listener_{{ $key }}
  address:
    socket_address:
      address: "::"
      ipv4_compat: true
      port_value: {{ $port.Listener.Port }}
      protocol: {{ $port.Protocol }}
{{- if eq $port.Protocol "UDP" }}
  udp_listener_config:
    downstream_socket_config:
      max_rx_datagram_size: 9000
  listener_filters:
  - name: envoy.filters.udp_listener.udp_proxy
    typed_config:
      "@type": type.googleapis.com/envoy.extensions.filters.udp.udp_proxy.v3.UdpProxyConfig
      stat_prefix: service
      matcher:
        on_no_match:
          action:
            name: route
            typed_config:
              "@type": type.googleapis.com/envoy.extensions.filters.udp.udp_proxy.v3.Route
              cluster: cluster_{{ $key }}
      access_log:
      - name: envoy.access_loggers.file
        typed_config:
          "@type": type.googleapis.com/envoy.extensions.access_loggers.file.v3.FileAccessLog
          path: /dev/stdout
          log_format:
            text_format_source:
              inline_string: "[%START_TIME%] %DOWNSTREAM_REMOTE_ADDRESS% -> %UPSTREAM_HOST% (%BYTES_RECEIVED%/%BYTES_SENT%) %DURATION%ms\n"
{{- else }}
  filter_chains:
  - filters:
    - name: envoy.filters.network.tcp_proxy
      typed_config:
        "@type": type.googleapis.com/envoy.extensions.filters.network.tcp_proxy.v3.TcpProxy
        stat_prefix: service
        cluster: cluster_{{ $key }}
        access_log:
        - name: envoy.access_loggers.file
          typed_config:
            "@type": type.googleapis.com/envoy.extensions.access_loggers.file.v3.FileAccessLog
            path: /dev/stdout
            log_format:
              text_format_source:
                inline_string: "[%START_TIME%] %DOWNSTREAM_REMOTE_ADDRESS% -> %UPSTREAM_HOST% (%BYTES_RECEIVED%/%BYTES_SENT%) %DURATION%ms\n"
{{- end }}
{{- end }}
`

const cdsTemplate = `resources:
{{- range $key, $port := .ServicePorts }}
- "@type": type.googleapis.com/envoy.config.cluster.v3.Cluster
  name: cluster_{{ $key }}
  connect_timeout: 3s
  type: STATIC
  lb_policy: RANDOM
  common_lb_config:
    healthy_panic_threshold:
      value: 0
  health_checks:
  - timeout: 3s
    interval: 5s
    unhealthy_threshold: 3
    healthy_threshold: 1
    http_health_check:
      # kube-proxy health check endpoint
      path: /healthz
      host: healthcheck
  load_assignment:
    cluster_name: cluster_{{ $key }}
    endpoints:
    - lb_endpoints:
{{- range $endpoint := $port.Cluster }}
      - endpoint:
          health_check_config:
            # kube-proxy health check port
            port_value: 10256
          address:
            socket_address:
              address: {{ $endpoint.Address }}
              port_value: {{ $endpoint.Port }}
              protocol: {{ $port.Protocol }}
{{- end }}
{{- end }}
`

var (
	ldsTemplateParsed = template.Must(template.New("lds").Parse(ldsTemplate))
	cdsTemplateParsed = template.Must(template.New("cds").Parse(cdsTemplate))
)

type envoyConfigData struct {
	ServicePorts map[string]servicePort
}

type servicePort struct {
	Protocol string
	Listener listener
	Cluster  []endpoint
}

type listener struct {
	Port int32
}

type endpoint struct {
	Address string
	Port    int32
}

func generateEnvoyDynamicConfig(service *corev1.Service, nodes []*corev1.Node) (ldsConfig, cdsConfig []byte, err error) {
	allNodeIPs := make([]netip.Addr, 0, len(nodes))
	for _, node := range nodes {
		nodeIPs, err := getInternalNodeIPs(node)
		if err != nil {
			return nil, nil, fmt.Errorf("could not get internal IPs of node %s: %w", node.Name, err)
		}

		allNodeIPs = append(allNodeIPs, nodeIPs.AsSlice()...)
	}

	data := &envoyConfigData{
		ServicePorts: make(map[string]servicePort, len(service.Spec.Ports)),
	}

	for _, port := range service.Spec.Ports {
		if port.NodePort == 0 {
			continue
		}

		sp := servicePort{
			Listener: listener{
				Port: port.Port,
			},
		}

		for _, nodeIP := range allNodeIPs {
			sp.Cluster = append(sp.Cluster, endpoint{
				Address: nodeIP.String(),
				Port:    port.NodePort,
			})
		}

		data.ServicePorts[fmt.Sprintf("%d_%s", port.Port, string(port.Protocol))] = sp
	}

	var ldsBuf, cdsBuf bytes.Buffer
	if err := ldsTemplateParsed.Execute(&ldsBuf, data); err != nil {
		return nil, nil, fmt.Errorf("error rendering LDS config: %w", err)
	}
	if err := cdsTemplateParsed.Execute(&cdsBuf, data); err != nil {
		return nil, nil, fmt.Errorf("error rendering CDS config: %w", err)
	}

	return ldsBuf.Bytes(), cdsBuf.Bytes(), nil
}

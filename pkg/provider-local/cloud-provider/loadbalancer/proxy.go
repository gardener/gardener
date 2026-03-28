// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package loadbalancer

import (
	"bytes"
	"fmt"
	"net/netip"
	"text/template"

	corev1 "k8s.io/api/core/v1"
)

// TODO: consider adding HTTP access logs

const dynamicFilesystemConfig = `node:
  cluster: gardener-provider-local
  id: gardener-provider-local-id
dynamic_resources:
  cds_config:
    resource_api_version: V3
    path_config_source:
      path: /home/envoy/cds.yaml
  lds_config:
    resource_api_version: V3
    path_config_source:
      path: /home/envoy/lds.yaml
admin:
  access_log:
  - name: envoy.access_loggers.file
    typed_config:
      "@type": type.googleapis.com/envoy.extensions.access_loggers.file.v3.FileAccessLog
      path: /dev/stdout
  address:
    socket_address:
      address: "::"
      ipv4_compat: true
      port_value: 10000
`

const ldsTemplate = `resources:
{{- range $key, $sp := .ServicePorts }}
- "@type": type.googleapis.com/envoy.config.listener.v3.Listener
  name: listener_{{ $key }}
  address:
    socket_address:
      address: "::"
      ipv4_compat: true
      port_value: {{ $sp.Listener.Port }}
      protocol: {{ $sp.Listener.Protocol }}
{{- if eq $sp.Listener.Protocol "UDP" }}
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
{{- else }}
  filter_chains:
  - filters:
    - name: envoy.filters.network.tcp_proxy
      typed_config:
        "@type": type.googleapis.com/envoy.extensions.filters.network.tcp_proxy.v3.TcpProxy
        stat_prefix: service
        cluster: cluster_{{ $key }}
{{- end }}
{{- end }}
`

const cdsTemplate = `resources:
{{- range $key, $sp := .ServicePorts }}
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
      path: /healthz
      host: healthcheck
  load_assignment:
    cluster_name: cluster_{{ $key }}
    endpoints:
    - lb_endpoints:
{{- range $ep := $sp.Cluster }}
      - endpoint:
          health_check_config:
            port_value: 10256
          address:
            socket_address:
              address: {{ $ep.Address }}
              port_value: {{ $ep.Port }}
              protocol: {{ $ep.Protocol }}
{{- end }}
{{- end }}
`

var (
	ldsTemplateParsed = template.Must(template.New("lds").Parse(ldsTemplate))
	cdsTemplateParsed = template.Must(template.New("cds").Parse(cdsTemplate))
)

type proxyConfigData struct {
	ServicePorts map[string]servicePort
}

type servicePort struct {
	Listener endpoint
	Cluster  []endpoint
}

type endpoint struct {
	Address  string
	Port     int32
	Protocol string
}

func getInternalNodeIPs(node *corev1.Node) (ipSet, error) {
	var out ipSet

	for _, addr := range node.Status.Addresses {
		if addr.Type != corev1.NodeInternalIP {
			continue
		}

		ip, err := netip.ParseAddr(addr.Address)
		if err != nil {
			return out, fmt.Errorf("could not parse internal node IP %q: %w", addr.Address, err)
		}

		if ip.Is4() {
			if out.ipv4.IsValid() && out.ipv4 != ip {
				return out, fmt.Errorf("multiple internal IPv4 addresses found for node: %q and %q", out.ipv4, ip)
			}
			out.ipv4 = ip
		} else if ip.Is6() {
			if out.ipv6.IsValid() && out.ipv6 != ip {
				return out, fmt.Errorf("multiple internal IPv6 addresses found for node: %q and %q", out.ipv6, ip)
			}
			out.ipv6 = ip
		}
	}

	if out.Len() == 0 {
		return out, fmt.Errorf("no address of type %s found", corev1.NodeInternalIP)
	}

	return out, nil
}

func getHostIPs(pod *corev1.Pod) (ipSet, error) {
	var out ipSet

	for _, addr := range pod.Status.HostIPs {
		ip, err := netip.ParseAddr(addr.IP)
		if err != nil {
			return out, fmt.Errorf("could not parse host IP %q: %w", addr, err)
		}

		if ip.Is4() {
			if out.ipv4.IsValid() {
				return out, fmt.Errorf("multiple host IPv4 addresses found for pod: %q and %q", out.ipv4, ip)
			}
			out.ipv4 = ip
		} else if ip.Is6() {
			if out.ipv6.IsValid() {
				return out, fmt.Errorf("multiple host IPv6 addresses found for pod: %q and %q", out.ipv6, ip)
			}
			out.ipv6 = ip
		}
	}

	return out, nil
}

func generateProxyConfig(service *corev1.Service, nodes []*corev1.Node) (ldsConfig, cdsConfig []byte, err error) {
	var allNodeIPs []netip.Addr
	for _, node := range nodes {
		nodeIPs, err := getInternalNodeIPs(node)
		if err != nil {
			return nil, nil, fmt.Errorf("could not get internal IPs of node %s: %w", node.Name, err)
		}

		allNodeIPs = append(allNodeIPs, nodeIPs.AsSlice()...)
	}

	data := &proxyConfigData{
		ServicePorts: make(map[string]servicePort),
	}

	for _, port := range service.Spec.Ports {
		if port.NodePort == 0 {
			continue
		}

		proto := string(port.Protocol)

		sp := servicePort{
			Listener: endpoint{
				Address:  "0.0.0.0",
				Port:     port.Port,
				Protocol: proto,
			},
		}

		for _, nodeIP := range allNodeIPs {
			sp.Cluster = append(sp.Cluster, endpoint{
				Address:  nodeIP.String(),
				Port:     port.NodePort,
				Protocol: proto,
			})
		}

		data.ServicePorts[fmt.Sprintf("%d_%s", port.Port, proto)] = sp
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

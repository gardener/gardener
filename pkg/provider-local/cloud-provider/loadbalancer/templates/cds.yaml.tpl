resources:
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

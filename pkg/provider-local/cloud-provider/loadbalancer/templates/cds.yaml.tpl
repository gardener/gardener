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
  - timeout: 5s
    interval: 3s
    unhealthy_threshold: 2
    healthy_threshold: 1
    no_traffic_interval: 5s
    event_log_path: /dev/stdout
    http_health_check:
      # kube-proxy health check endpoint
      path: /healthz
  load_assignment:
    cluster_name: cluster_{{ $key }}
    endpoints:
    - lb_endpoints:
{{- range $endpoint := $port.Cluster }}
      - endpoint:
          health_check_config:
            # kube-proxy health check port
            # - defaults to 10256 for externalTrafficPolicy=Cluster
            # - Service.spec.healthCheckNodePort for externalTrafficPolicy=Local
            port_value: {{ $.HealthCheckPort }}
          address:
            socket_address:
              address: {{ $endpoint.Address }}
              port_value: {{ $endpoint.Port }}
              protocol: {{ $port.Protocol }}
{{- end }}
{{- end }}

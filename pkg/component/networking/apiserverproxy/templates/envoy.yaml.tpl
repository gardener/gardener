layered_runtime:
  layers:
    - name: static_layer_0
      static_layer:
        envoy:
          resource_limits:
            listener:
              kube_apiserver:
                connection_limit: 10000
        overload:
          global_downstream_max_connections: 10000
admin:
  access_log:
  - name: envoy.access_loggers.stdout
    # Remove spammy readiness/liveness probes and metrics requests from access log
    filter:
      and_filter:
        filters:
        - header_filter:
            header:
              name: :Path
              string_match:
                exact: /ready
              invert_match: true
        - header_filter:
            header:
              name: :Path
              string_match:
                exact: /stats/prometheus
              invert_match: true
    typed_config:
      "@type": type.googleapis.com/envoy.extensions.access_loggers.stream.v3.StdoutAccessLog
  address:
    pipe:
      # The admin interface should not be exposed as a TCP address.
      # It's only used and exposed via the metrics lister that
      # exposes only /stats/prometheus path for metrics scrape.
      path: /etc/admin-uds/admin.socket
static_resources:
  listeners:
  - name: kube_apiserver
    address:
      socket_address:
        address: {{ .advertiseIPAddress }}
        port_value: 443
    per_connection_buffer_limit_bytes: 32768 # 32 KiB
    filter_chains:
    - filters:
      - name: envoy.filters.network.tcp_proxy
        typed_config:
          "@type": type.googleapis.com/envoy.extensions.filters.network.tcp_proxy.v3.TcpProxy
          stat_prefix: kube_apiserver
          cluster: kube_apiserver
          tunneling_config:
            # hostname is irrelevant as it will be dropped by envoy, we still need it for the configuration though
            hostname: "{{ .proxySeedServerHost }}:443"
            headers_to_add:
            - header:
                key: Reversed-VPN
                value: "outbound|443||kube-apiserver.{{ .seedNamespace }}.svc.cluster.local"
          access_log:
          - name: envoy.access_loggers.stdout
            typed_config:
              "@type": type.googleapis.com/envoy.extensions.access_loggers.stream.v3.StdoutAccessLog
              log_format:
                text_format_source:
                  inline_string: "[%START_TIME%] %RESPONSE_CODE% %RESPONSE_FLAGS% %BYTES_RECEIVED% rx %BYTES_SENT% tx %DURATION%ms \"%DOWNSTREAM_REMOTE_ADDRESS%\" \"%UPSTREAM_HOST%\"\n"
  - name: metrics
    address:
      socket_address:
        address: "0.0.0.0"
        port_value: {{ .adminPort }}
    additional_addresses:
    - address:
        socket_address:
          address: "::"
          port_value: {{ .adminPort }}
    filter_chains:
    - filters:
      - name: envoy.filters.network.http_connection_manager
        typed_config:
          "@type": type.googleapis.com/envoy.extensions.filters.network.http_connection_manager.v3.HttpConnectionManager
          stat_prefix: ingress_http
          use_remote_address: true
          common_http_protocol_options:
            idle_timeout: 8s
            max_connection_duration: 10s
            max_headers_count: 20
            max_stream_duration: 8s
            headers_with_underscores_action: REJECT_REQUEST
          http2_protocol_options:
            max_concurrent_streams: 5
            initial_stream_window_size: 65536
            initial_connection_window_size: 1048576
          stream_idle_timeout: 8s
          request_timeout: 9s
          codec_type: AUTO
          route_config:
            name: local_route
            virtual_hosts:
            - name: local_service
              domains: ["*"]
              routes:
              - match:
                  path: /metrics
                route:
                  cluster: uds_admin
                  prefix_rewrite: /stats/prometheus
              - match:
                  path: /ready
                route:
                  cluster: uds_admin
          http_filters:
          - name: envoy.filters.http.router
            typed_config:
              "@type": type.googleapis.com/envoy.extensions.filters.http.router.v3.Router

  clusters:
  - name: kube_apiserver
    connect_timeout: 5s
    per_connection_buffer_limit_bytes: 32768 # 32 KiB
    type: LOGICAL_DNS
    dns_lookup_family: {{ .dnsLookupFamily }}
    lb_policy: ROUND_ROBIN
    load_assignment:
      cluster_name: kube_apiserver
      endpoints:
      - lb_endpoints:
        - endpoint:
            address:
              socket_address:
                address: {{ .proxySeedServerHost }}
                port_value: {{ .proxySeedServerPort }}
    upstream_connection_options:
      tcp_keepalive:
        keepalive_time: 7200
        keepalive_interval: 55
  - name: uds_admin
    connect_timeout: 0.25s
    type: STATIC
    lb_policy: ROUND_ROBIN
    load_assignment:
      cluster_name: uds_admin
      endpoints:
      - lb_endpoints:
          - endpoint:
              address:
                pipe:
                  path: /etc/admin-uds/admin.socket
    transport_socket:
      name: envoy.transport_sockets.raw_buffer
      typed_config:
        "@type": type.googleapis.com/envoy.extensions.transport_sockets.raw_buffer.v3.RawBuffer

static_resources:
  listeners:
  - name: listener_0
    address:
      socket_address:
        protocol: TCP
        address: "{{ .listenAddress }}"
        port_value: {{ .envoyPort }}
    additional_addresses:
    - address:
        socket_address:
          address: "{{ .listenAddressV6 }}"
          port_value: {{ .envoyPort }}
    listener_filters:
    - name: "envoy.filters.listener.tls_inspector"
      typed_config:
        "@type": type.googleapis.com/envoy.extensions.filters.listener.tls_inspector.v3.TlsInspector
    filter_chains:
    - transport_socket:
        name: envoy.transport_sockets.tls
        typed_config:
          "@type": type.googleapis.com/envoy.extensions.transport_sockets.tls.v3.DownstreamTlsContext
          # require_client_certificate ensures that only clients presenting a certificate signed by
          # the trusted CA (ca-vpn) may use the CONNECT proxy.
          require_client_certificate: true
          common_tls_context:
            tls_certificates:
            - certificate_chain: { filename: {{ .certChain }} }
              private_key: { filename: {{ .privateKey }} }
            validation_context:
              trusted_ca: { filename: {{ .caCert }} }
      filters:
      - name: envoy.filters.network.http_connection_manager
        typed_config:
          "@type": type.googleapis.com/envoy.extensions.filters.network.http_connection_manager.v3.HttpConnectionManager
          stat_prefix: ingress_http
          access_log:
          - name: envoy.access_loggers.stdout
            filter:
              or_filter:
                filters:
                - status_code_filter:
                    comparison:
                      op: GE
                      value:
                        default_value: 500
                        runtime_key: "null"
                - duration_filter:
                    comparison:
                      op: GE
                      value:
                        default_value: 1000
                        runtime_key: "null"
            typed_config:
              "@type": type.googleapis.com/envoy.extensions.access_loggers.stream.v3.StdoutAccessLog
              log_format:
                text_format_source:
                  inline_string: "[%START_TIME%] \"%REQ(:METHOD)% %REQ(X-ENVOY-ORIGINAL-PATH?:PATH)% %PROTOCOL%\" %RESPONSE_CODE% %RESPONSE_FLAGS% %BYTES_RECEIVED% rx %BYTES_SENT% tx %DURATION%ms \"%DOWNSTREAM_REMOTE_ADDRESS%\" \"%REQ(X-REQUEST-ID)%\" \"%REQ(:AUTHORITY)%\" \"%UPSTREAM_HOST%\"\n"
          route_config:
            name: local_route
            virtual_hosts:
            - name: local_service
              domains:
              - "*"
              routes:
              - match:
                  connect_matcher: {}
                route:
                  cluster: dynamic_forward_proxy_cluster
                  upgrade_configs:
                  - upgrade_type: CONNECT
                    connect_config: {}
          http_filters:
          - name: envoy.filters.http.dynamic_forward_proxy
            typed_config:
              "@type": type.googleapis.com/envoy.extensions.filters.http.dynamic_forward_proxy.v3.FilterConfig
              dns_cache_config:
                name: dynamic_forward_proxy_cache_config
                dns_lookup_family: {{ .dnsLookupFamily }}
                max_hosts: 8192
                # resolved_address_filter restricts the CONNECT proxy to the shoot's own node, pod,
                # and service networks. It matches on the RESOLVED upstream IP (so it also covers
                # IP-literal CONNECT targets and resists DNS rebinding). With invert_match: true the
                # listed CIDRs are the allow-list: any resolved address outside them is removed from
                # the DNS response, so the connection fails (counter dns_cache.<name>.dns_address_filter_out).
                # This prevents the proxy from being used as a pivot into seed-adjacent networks.
                resolved_address_filter:
                  invert_match: true
                  {{- if .allowedDestinationCIDRs }}
                  ranges:
                  {{- range .allowedDestinationCIDRs }}
                  - address_prefix: "{{ .Address }}"
                    prefix_len: {{ .PrefixLen }}
                  {{- end }}
                  {{- else }}
                  # No shoot CIDRs configured: an empty allow-list with invert_match removes every
                  # resolved address, denying all connections (fail-closed).
                  ranges: []
                  {{- end }}
          - name: envoy.filters.http.router
            typed_config:
              "@type": type.googleapis.com/envoy.extensions.filters.http.router.v3.Router
          http_protocol_options:
            accept_http_10: true
          upgrade_configs:
          - upgrade_type: CONNECT
  - name: metrics_listener
    address:
      socket_address:
        address: "{{ .listenAddress }}"
        port_value: {{ .metricsPort }}
    additional_addresses:
    - address:
        socket_address:
          address: "{{ .listenAddressV6 }}"
          port_value: {{ .metricsPort }}
    filter_chains:
    - filters:
      - name: envoy.filters.network.http_connection_manager
        typed_config:
          "@type": type.googleapis.com/envoy.extensions.filters.network.http_connection_manager.v3.HttpConnectionManager
          stat_prefix: stats_server
          route_config:
            virtual_hosts:
            - name: admin_interface
              domains:
              - "*"
              routes:
              - match:
                  prefix: "/metrics"
                  headers:
                  - name: ":method"
                    string_match:
                      exact: GET
                route:
                  cluster: prometheus_stats
                  prefix_rewrite: "/stats/prometheus"
          http_filters:
          - name: envoy.filters.http.router
            typed_config:
              "@type": type.googleapis.com/envoy.extensions.filters.http.router.v3.Router
  clusters:
  - name: dynamic_forward_proxy_cluster
    connect_timeout: 20s
    circuitBreakers:
      thresholds:
      - maxConnections: 8192
    lb_policy: CLUSTER_PROVIDED
    cluster_type:
      name: envoy.clusters.dynamic_forward_proxy
      typed_config:
        "@type": type.googleapis.com/envoy.extensions.clusters.dynamic_forward_proxy.v3.ClusterConfig
        dns_cache_config:
          name: dynamic_forward_proxy_cache_config
          dns_lookup_family: {{ .dnsLookupFamily }}
          max_hosts: 8192
          # Must stay identical to the dns_cache_config on the dynamic_forward_proxy HTTP filter
          # (same named cache). See the resolved_address_filter explanation there.
          resolved_address_filter:
            invert_match: true
            {{- if .allowedDestinationCIDRs }}
            ranges:
            {{- range .allowedDestinationCIDRs }}
            - address_prefix: "{{ .Address }}"
              prefix_len: {{ .PrefixLen }}
            {{- end }}
            {{- else }}
            ranges: []
            {{- end }}
  - name: prometheus_stats
    connect_timeout: 0.25s
    type: static
    load_assignment:
      cluster_name: prometheus_stats
      endpoints:
      - lb_endpoints:
        - endpoint:
            address:
              pipe:
                path: /home/nonroot/envoy.admin
admin:
  address:
    pipe:
      path: /home/nonroot/envoy.admin

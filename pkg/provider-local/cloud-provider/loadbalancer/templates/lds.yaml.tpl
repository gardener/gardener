resources:
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

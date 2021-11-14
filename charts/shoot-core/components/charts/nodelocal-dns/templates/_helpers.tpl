{{- define "node-local-dns.config.data" -}}
Corefile: |
  {{ .Values.domain }}:53 {
      errors
      cache {
              success 9984 30
              denial 9984 5
      }
      reload
      loop
      bind {{ .Values.nodeLocal }} {{ .Values.dnsServer }}
      forward . {{ .Values.clusterDNS }} {
{{- if .Values.forceTcpToClusterDNS }}
              force_tcp
{{- else }}
              prefer_udp
{{- end }}
      }
      prometheus :{{ .Values.prometheus.port }}
      health {{ .Values.nodeLocal }}:8080
      }
  in-addr.arpa:53 {
      errors
      cache 30
      reload
      loop
      bind {{ .Values.nodeLocal }} {{ .Values.dnsServer }}
      forward . {{ .Values.clusterDNS }} {
{{- if .Values.forceTcpToClusterDNS }}
              force_tcp
{{- else }}
              prefer_udp
{{- end }}
      }
      prometheus :{{ .Values.prometheus.port }}
      }
  ip6.arpa:53 {
      errors
      cache 30
      reload
      loop
      bind {{ .Values.nodeLocal }} {{ .Values.dnsServer }}
      forward . {{ .Values.clusterDNS }} {
{{- if .Values.forceTcpToClusterDNS }}
              force_tcp
{{- else }}
              prefer_udp
{{- end }}
      }
      prometheus :{{ .Values.prometheus.port }}
      }
  .:53 {
      errors
      cache 30
      reload
      loop
      bind {{ .Values.nodeLocal }} {{ .Values.dnsServer }}
      forward . __PILLAR__UPSTREAM__SERVERS__ {
{{- if .Values.forceTcpToUpstreamDNS }}
              force_tcp
{{- else }}
              prefer_udp
{{- end }}
      }
      prometheus :{{ .Values.prometheus.port }}
      }
{{- end -}}

{{- define "node-local-dns.config.name" -}}
node-local-dns-{{ include "node-local-dns.config.data" . | sha256sum | trunc 8 }}
{{- end }}

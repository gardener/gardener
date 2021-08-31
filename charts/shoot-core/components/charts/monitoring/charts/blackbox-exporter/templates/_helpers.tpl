{{- define "blackbox-exporter.config.data" -}}
blackbox.yaml: |
  modules:
    http_kubernetes_service:
      prober: http
      timeout: 10s
      http:
        headers:
          Accept: "*/*"
          Accept-Language: "en-US"
        tls_config:
          ca_file: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt
        bearer_token_file: /var/run/secrets/kubernetes.io/serviceaccount/token
        preferred_ip_protocol: "ip4"
{{- end -}}

{{- define "blackbox-exporter.config.name" -}}
blackbox-exporter-config-{{ include "blackbox-exporter.config.data" . | sha256sum | trunc 8 }}
{{- end }}

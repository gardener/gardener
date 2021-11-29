{{- define "prometheus.service-endpoints.relabel-config" -}}
- source_labels: [__meta_kubernetes_service_annotation_prometheus_io_scrape]
  action: keep
  regex: true
- source_labels: [__meta_kubernetes_service_annotation_prometheus_io_scheme]
  action: replace
  target_label: __scheme__
  regex: (https?)
- source_labels: [__meta_kubernetes_service_annotation_prometheus_io_path]
  action: replace
  target_label: __metrics_path__
  regex: (.+)
- source_labels: [__address__, __meta_kubernetes_service_annotation_prometheus_io_port]
  action: replace
  target_label: __address__
  regex: ([^:]+)(?::\d+)?;(\d+)
  replacement: $1:$2
- action: labelmap
  regex: __meta_kubernetes_service_label_(.+)
- source_labels: [__meta_kubernetes_service_name]
  action: replace
  target_label: job
- source_labels: [__meta_kubernetes_service_annotation_prometheus_io_name]
  action: replace
  target_label: job
  regex: (.+)
- source_labels: [__meta_kubernetes_pod_name]
  target_label: pod
{{- end -}}

{{/*
Drops metrics which produce lots of time-series without much gain.
*/}}
{{- define "prometheus.drop-metrics.metric-relabel-config" -}}
- source_labels: [ __name__ ]
  regex: ^rest_client_request_latency_seconds.+$
  action: drop
{{- end -}}

{{- define "prometheus.keep-metrics.metric-relabel-config" -}}
- source_labels: [ __name__ ]
  regex: ^({{ . | join "|" }})$
  action: keep
{{- end -}}

{{- define "prometheus.kube-auth" -}}
tls_config:
  ca_file: /etc/prometheus/seed/ca.crt
authorization:
  type: Bearer
  credentials_file: /var/run/secrets/gardener.cloud/shoot/token/token
{{- end -}}

{{- define "prometheus.alertmanager.namespaces" -}}
- garden
{{- if  (index .Values.rules.optional "alertmanager" ).enabled }}
- {{ .Release.Namespace }}
{{- end }}
{{- end -}}


{{- define "prometheus.keep-metrics.metric-relabel-config" -}}
- source_labels: [ __name__ ]
  regex: ^({{ . | join "|" }})$
  action: keep
{{- end -}}

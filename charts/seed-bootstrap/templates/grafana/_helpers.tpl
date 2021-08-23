{{- define "grafana.providers.data" -}}
default.yaml: |-
  apiVersion: 1
  providers:
  - name: 'default'
    orgId: 1
    folder: ''
    type: file
    disableDeletion: false
    editable: false
    options:
      path: /var/lib/grafana/dashboards
{{- end -}}

{{- define "grafana.providers.name" -}}
grafana-dashboard-providers-{{ include "grafana.providers.data" . | sha256sum | trunc 8 }}
{{- end }}

{{- define "grafana.datasources.data" -}}
datasources.yaml: |-
  # config file version
  apiVersion: 1

  # list of datasources that should be deleted from the database
  deleteDatasources:
  - name: Graphite
    orgId: 1

  # list of datasources to insert/update depending
  # whats available in the database
  datasources:
  - name: prometheus
    type: prometheus
    access: proxy
    url: http://aggregate-prometheus-web:80
    basicAuth: false
    isDefault: true
    version: 1
    editable: false
    jsonData:
      timeInterval: 1m
  - name: seed-prometheus
    type: prometheus
    access: proxy
    url: http://seed-prometheus-web:80
    basicAuth: false
    version: 1
    editable: false
    jsonData:
      timeInterval: 1m
  - name: loki
    type: loki
    access: proxy
    url: http://loki.garden.svc:3100
    jsonData:
      maxLines: 5000
{{- end -}}

{{- define "grafana.datasources.name" -}}
grafana-datasources-{{ include "grafana.datasources.data" . | sha256sum | trunc 8 }}
{{- end }}

{{- define "grafana.dashboards.data" -}}
{{ range $name, $bytes := .Files.Glob "dashboards/**.json" }}
{{ base $name }}: |-
{{ toString $bytes | indent 4}}
{{ end }}
{{ if .Values.istio.enabled }}
{{ range $name, $bytes := .Files.Glob "dashboards/istio/**.json" }}
{{ base $name }}: |-
{{ toString $bytes | indent 4}}
{{ end }}
{{- end }}
{{- end -}}

{{- define "grafana.dashboards.name" -}}
grafana-dashboards-{{ include "grafana.dashboards.data" . | sha256sum | trunc 8 }}
{{- end }}

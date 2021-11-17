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
grafana-{{ .Values.role }}-dashboard-providers-{{ include "grafana.providers.data" . | sha256sum | trunc 8 }}
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
    url: http://prometheus-web:80
    basicAuth: false
    isDefault: true
    version: 1
    editable: false
    jsonData:
      timeInterval: 1m
  - name: loki
    type: loki
    access: proxy
    url: http://loki.{{ .Release.Namespace }}.svc:3100
    jsonData:
      maxLines: 1000
{{- end -}}

{{- define "grafana.datasources.name" -}}
grafana-{{ .Values.role }}-datasources-{{ include "grafana.datasources.data" . | sha256sum | trunc 8 }}
{{- end }}

{{- define "grafana.dashboards.data" -}}
{{- if .Values.sni.enabled }}
{{ range $name, $bytes := .Files.Glob "dashboards/operators/istio/**.json" }}
{{ base $name }}: |-
{{ toString $bytes | indent 2 }}
{{- end }}
{{- end }}
{{- if .Values.nodeLocalDNS.enabled }}
{{ range $name, $bytes := .Files.Glob "dashboards/dns/**.json" }}
{{ base $name }}: |-
{{ toString $bytes | indent 2 }}
{{- end }}
{{- end }}
{{ if eq .Values.role "users" }}
{{ range $name, $bytes := .Files.Glob "dashboards/owners/**.json" }}
{{ if not (and (eq $name "dashboards/owners/shoot-vpa-dashboard.json") (eq $.Values.vpaEnabled false)) }}
{{ base $name }}: |-
{{ toString $bytes | indent 2 }}
{{ end }}
{{ end }}
{{ else }}
{{ range $name, $bytes := .Files.Glob "dashboards/owners/**.json" }}
{{ base $name }}: |-
{{ toString $bytes | indent 2 }}
{{ end }}
{{ range $name, $bytes := .Files.Glob "dashboards/operators/**.json" }}
{{ base $name }}: |-
{{ toString $bytes | indent 2 }}
{{ end }}
{{ end }}
{{- if .Values.extensions.dashboards }}
{{- toString .Values.extensions.dashboards }}
{{ end }}
{{- end -}}

{{- define "grafana.dashboards.name" -}}
grafana-{{ .Values.role }}-dashboards-{{ include "grafana.dashboards.data" . | sha256sum | trunc 8 }}
{{- end }}

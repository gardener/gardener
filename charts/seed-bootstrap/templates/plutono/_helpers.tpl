{{- define "plutono.providers.data" -}}
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
      path: /var/lib/plutono/dashboards
{{- end -}}

{{- define "plutono.providers.name" -}}
plutono-dashboard-providers-{{ include "plutono.providers.data" . | sha256sum | trunc 8 }}
{{- end }}

{{- define "plutono.datasources.data" -}}
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
  - name: vali
    type: vali
    access: proxy
    url: http://logging.garden.svc:3100
    jsonData:
      maxLines: 5000
{{- end -}}

{{- define "plutono.datasources.name" -}}
plutono-datasources-{{ include "plutono.datasources.data" . | sha256sum | trunc 8 }}
{{- end }}

{{- define "plutono.toCompactedJson" -}}
{{ . | fromJson | toJson}}
{{- end }}

{{- define "plutono.dashboards.data" -}}
{{ range $name, $bytes := .Files.Glob "dashboards/**.json" }}
{{ base $name }}: |-
{{ toString $bytes | include "plutono.toCompactedJson" | indent 2}}
{{ end }}
{{ if .Values.istio.enabled }}
{{ range $name, $bytes := .Files.Glob "dashboards/istio/**.json" }}
{{ base $name }}: |-
{{ toString $bytes | include "plutono.toCompactedJson" | indent 2}}
{{ end }}
{{- end }}
{{- end -}}

{{- define "plutono.dashboards.name" -}}
plutono-dashboards-{{ include "plutono.dashboards.data" . | sha256sum | trunc 8 }}
{{- end }}

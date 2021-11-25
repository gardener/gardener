{{- define "vpa.rbac-name-infix" -}}
{{- if eq .Values.clusterType "shoot" -}}
target
{{- else -}}
{{ .Values.clusterType }}{{/* TODO: Rename this to "source" once the VPA Helm charts gets refactored into a Golang component. */}}
{{- end -}}
{{- end -}}

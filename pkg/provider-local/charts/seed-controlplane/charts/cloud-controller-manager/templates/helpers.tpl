{{- define "is-shoot" -}}
{{- if hasPrefix "shoot-" .Values.clusterName }}
true
{{- end -}}
{{- end -}}

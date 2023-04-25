{{- define "gardenlet.apiserver-sni-enabled" -}}
{{- if .Values.config.featureGates -}}
{{- if hasKey .Values.config.featureGates "APIServerSNI" -}}
{{- .Values.config.featureGates.APIServerSNI -}}
{{- else -}}
true
{{- end -}}
{{- else -}}
true
{{- end -}}
{{- end -}}

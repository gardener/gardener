{{- define "gardenlet.apiserver-sni-enabled" -}}
{{- if .Values.global.gardenlet.config.featureGates -}}
{{- if hasKey .Values.global.gardenlet.config.featureGates "APIServerSNI" -}}
{{- .Values.global.gardenlet.config.featureGates.APIServerSNI -}}
{{- else -}}
true
{{- end -}}
{{- else -}}
true
{{- end -}}
{{- end -}}

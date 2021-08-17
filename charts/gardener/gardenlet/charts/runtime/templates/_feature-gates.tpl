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

{{- define "gardenlet.managed-istio-enabled" -}}
{{- if .Values.global.gardenlet.config.featureGates -}}
{{- if hasKey .Values.global.gardenlet.config.featureGates "ManagedIstio" -}}
{{- .Values.global.gardenlet.config.featureGates.ManagedIstio -}}
{{- else -}}
true
{{- end -}}
{{- else -}}
true
{{- end -}}
{{- end -}}

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

{{- define "gardenlet.managed-istio-enabled" -}}
{{- if .Values.config.featureGates -}}
{{- if hasKey .Values.config.featureGates "ManagedIstio" -}}
{{- .Values.config.featureGates.ManagedIstio -}}
{{- else -}}
true
{{- end -}}
{{- else -}}
true
{{- end -}}
{{- end -}}

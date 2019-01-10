{{- define "cloud-controller-manager.featureGates" -}}
{{- if .Values.featureGates }}
- --feature-gates={{ range $feature, $enabled := .Values.featureGates }}{{ $feature }}={{ $enabled }},{{ end }}
{{- end }}
{{- end -}}

{{- define "cloud-controller-manager.port" -}}
{{- if semverCompare ">= 1.13" .Values.kubernetesVersion -}}
10258
{{- else -}}
10253
{{- end -}}
{{- end -}}

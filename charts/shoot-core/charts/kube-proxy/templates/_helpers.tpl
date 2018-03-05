{{- /* https://github.com/kubernetes/kubernetes/pull/57962 */ -}}
{{- define "kube-proxy.featureGates" -}}
{{- if .Values.featureGates }}
featureGates: {{ range $feature, $enabled := .Values.featureGates }}{{ $feature }}={{ $enabled }},{{ end }}
{{- end }}
{{- end -}}

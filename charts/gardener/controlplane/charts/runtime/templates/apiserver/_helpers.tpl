{{- define "gardener-apiserver.featureGates" }}
{{- if .Values.global.apiserver.featureGates }}
- --feature-gates={{ range $feature, $enabled := .Values.global.apiserver.featureGates }}{{ $feature }}={{ $enabled }},{{ end }}
{{- end }}
{{- end -}}

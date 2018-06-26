{{- define "gardener-apiserver.featureGates" }}
{{- if .Values.apiserver.featureGates }}
- --feature-gates={{ range $feature, $enabled := .Values.apiserver.featureGates }}{{ $feature }}={{ $enabled }},{{ end }}
{{- end }}
{{- end -}}

{{- define "kube-controller-manager.featureGates" -}}
{{- if .Values.FeatureGates }}
- --feature-gates={{ range $feature, $enabled := .Values.FeatureGates }}{{ $feature }}={{ $enabled }},{{ end }}
{{- end }}
{{- end -}}

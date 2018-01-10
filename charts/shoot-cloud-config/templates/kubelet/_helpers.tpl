{{- define "kubelet.featureGates" -}}
{{- if .kubernetes.kubelet.featureGates }}
--feature-gates={{ range $feature, $enabled := .kubernetes.kubelet.featureGates }}{{ $feature }}={{ $enabled }},{{ end }}
{{- end }}
{{- end -}}

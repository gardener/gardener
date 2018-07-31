{{- define "kube-scheduler.featureGates" -}}
{{- if .Values.featureGates }}
- --feature-gates={{ range $feature, $enabled := .Values.featureGates }}{{ $feature }}={{ $enabled }},{{ end }}
{{- end }}
{{- if semverCompare "< 1.11" .Values.kubernetesVersion }}
- --feature-gates=PodPriority=true
{{- end }}
{{- end -}}

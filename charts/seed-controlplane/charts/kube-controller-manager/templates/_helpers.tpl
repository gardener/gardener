{{- define "kube-controller-manager.featureGates" -}}
{{- if .Values.featureGates }}
- --feature-gates={{ range $feature, $enabled := .Values.featureGates }}{{ $feature }}={{ $enabled }},{{ end }}
{{- end }}
{{- end -}}

{{- define "kube-controller-manager.controllers" -}}
{{- if and (semverCompare "< 1.10" .Values.kubernetesVersion) (ne .Values.cloudProvider "") }}
- --controllers=*,bootstrapsigner,tokencleaner,-service,-route
{{- else }}
- --controllers=*,bootstrapsigner,tokencleaner
{{- end }}
{{- end -}}

{{- define "kube-controller-manager.cloudProviderFlags" -}}
{{- if (ne .Values.cloudProvider "") }}
{{- if semverCompare "< 1.10" .Values.kubernetesVersion }}
- --cloud-provider={{ .Values.cloudProvider }}
{{- else }}
- --cloud-provider=external
- --external-cloud-volume-plugin={{ .Values.cloudProvider }}
{{- end }}
{{- end }}
{{- end -}}

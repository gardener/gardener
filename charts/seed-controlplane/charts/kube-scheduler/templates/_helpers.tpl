{{- define "kube-scheduler.featureGates" -}}
{{- if .Values.featureGates }}
- --feature-gates={{ range $feature, $enabled := .Values.featureGates }}{{ $feature }}={{ $enabled }},{{ end }}
{{- end }}
{{- if semverCompare "< 1.11" .Values.kubernetesVersion }}
- --feature-gates=PodPriority=true
{{- end }}
{{- end -}}

{{- define "kube-scheduler.componentconfigversion" -}}
{{ if semverCompare ">= 1.12" .Values.kubernetesVersion -}}
kubescheduler.config.k8s.io/v1alpha1
{{- else -}}
componentconfig/v1alpha1
{{- end }}
{{- end -}}

{{- define "kube-scheduler.port" -}}
{{- if semverCompare ">= 1.13" .Values.kubernetesVersion -}}
10259
{{- else -}}
10251
{{- end -}}
{{- end -}}

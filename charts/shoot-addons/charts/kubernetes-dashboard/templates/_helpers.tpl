{{- define "kubernetes-dashboard.namespace" -}}
{{- if semverCompare ">= 1.16" .Capabilities.KubeVersion.GitVersion -}}
kubernetes-dashboard
{{- else -}}
kube-system
{{- end -}}
{{- end -}}

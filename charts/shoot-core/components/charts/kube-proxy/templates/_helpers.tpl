{{- define "kube-proxy.name" -}}
{{- if eq .name "" -}}
kube-proxy
{{- else -}}
kube-proxy-{{ .name }}-v{{ .kubernetesVersion }}
{{- end -}}
{{- end -}}

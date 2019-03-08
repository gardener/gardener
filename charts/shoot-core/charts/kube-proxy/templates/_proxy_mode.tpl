{{- define "kube-proxy.mode" -}}
{{- if .Values.enableIPVS -}}
ipvs
{{- else -}}
iptables
{{- end -}}
{{- end -}}

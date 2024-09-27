{{- define "name" -}}
{{- if .Values.gardener.runtimeCluster.enabled -}}
gardener-extension-provider-local-runtime
{{- else -}}
gardener-extension-provider-local
{{- end }}
{{- end -}}

{{- define "labels.app.key" -}}
app.kubernetes.io/name
{{- end -}}
{{- define "labels.app.value" -}}
{{ include "name" . }}
{{- end -}}

{{- define "labels" -}}
{{ include "labels.app.key" . }}: {{ include "labels.app.value" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{- define "poddisruptionbudgetversion" -}}
policy/v1
{{- end -}}

{{- define "coredns.enabled" -}}
{{- if .Values.gardener.runtimeCluster.enabled -}}
false
{{- else -}}
{{ .Values.coredns.enabled }}
{{- end }}
{{- end -}}

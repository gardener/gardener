{{- define "name" -}}
gardener-extension-provider-local
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
{{- if semverCompare ">= 1.21-0" .Capabilities.KubeVersion.GitVersion -}}
policy/v1
{{- else -}}
policy/v1beta1
{{- end -}}
{{- end -}}

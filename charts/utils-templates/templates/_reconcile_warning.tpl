x{{/* vim: set filetype=mustache: */}}
{{/*
Warsn that this resource cannot be modified.
*/}}
{{- define "utils-templates.reconcile-warning-annotation" -}}
gardener.cloud/description: |
{{ include "utils-templates.reconcile-warning-annotation-text" . | indent 2 }}
{{- end -}}

{{- define "utils-templates.reconcile-warning-annotation-text" -}}
DO NOT EDIT - This resource is managed by Gardener.
Any modifications are discarded and the resource is returned to the original state.
{{- end -}}

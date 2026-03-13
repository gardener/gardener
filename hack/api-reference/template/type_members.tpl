{{- define "type_members" -}}
{{- $field := . -}}
{{- if eq $field.Name "metadata" -}}
Refer to the Kubernetes API documentation for the fields of the <code>metadata</code> field.
{{- else -}}
{{ if index $field.Markers "optional" }}<em>(Optional)</em>
{{ end -}}
<p>{{ markdownRenderFieldDoc $field.Doc }}</p>
{{- end -}}
{{- end -}}

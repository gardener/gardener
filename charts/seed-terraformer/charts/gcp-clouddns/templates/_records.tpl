{{- define "gcp-clouddns.records" -}}
{{- range $j, $record := .values }}
"{{ $record }}{{ if ne (required "type is required" $.type) "ip" }}.{{ end }}",
{{- end -}}
{{- end -}}

{{- define "gcp-clouddns.records" -}}
{{- range $j, $record := .record.values }}
"{{ $record }}{{ if ne (required "record.type is required" $.record.type) "ip" }}.{{ end }}",
{{- end -}}
{{- end -}}

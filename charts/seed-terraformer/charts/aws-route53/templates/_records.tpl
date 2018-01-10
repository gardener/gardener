{{- define "aws-route53.records" -}}
{{- range $j, $record := .record.values }}
"{{ $record }}",
{{- end -}}
{{- end -}}

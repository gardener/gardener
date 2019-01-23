{{- define "aws-route53.records" -}}
{{- range $j, $record := .values }}
"{{ $record }}",
{{- end -}}
{{- end -}}

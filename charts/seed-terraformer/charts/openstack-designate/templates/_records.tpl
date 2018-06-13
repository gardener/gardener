{{- define "openstack-designate.records" -}}
{{- range $j, $record := .record.values }}
{{- if eq $.record.type "ip"}}
"{{ $record }}",
{{- else }}
"{{ $record | trimSuffix "."}}.",
{{- end }}
{{- end -}}
{{- end -}}

{{- define "openstack-designate.records" -}}
{{- range $j, $record := .record.values }}
"{{ $record }}",
{{- end -}}
{{- end -}}

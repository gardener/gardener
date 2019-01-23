{{- define "openstack-designate.records" -}}
{{- range $j, $record := .values }}
{{- if eq $.type "ip"}}
"{{ $record }}",
{{- else }}
"{{ $record | trimSuffix "."}}.",
{{- end }}
{{- end -}}
{{- end -}}

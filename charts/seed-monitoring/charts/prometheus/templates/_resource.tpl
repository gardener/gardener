{{/*
resource.quantity returns resource quantity based on number of objects (such as nodes, pods etc..),
resource per object, object weight and base resource quantity.
*/}}
{{- define "resource.quantity" -}}
{{- range $resourceKey, $resourceValue := $.resources }}
{{ $resourceKey }}:
{{- range $_, $r := $resourceValue }}
  {{ $r.name }}: {{ printf "%d%s" ( add $r.base ( mul ( div $.objectCount $r.weight ) $r.perObject $r.weight ) ) $r.unit }}
{{- end -}}
{{- end -}}
{{- end -}}
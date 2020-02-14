{{/*
util-templates.resource-quantity returns resource quantity based on number of objects (such as nodes, pods etc..),
resource per object, object weight and base resource quantity.
*/}}
{{- define "utils-templates.resource-quantity" -}}
{{- range $resourceKey, $resourceValue := (required "$.resource is required" $.resources) }}
{{ $resourceKey }}:
{{- range $unit, $r := $resourceValue }}
  {{ $unit }}: {{ printf "%d%s" ( add $r.base ( mul ( div $.objectCount $r.weight ) $r.perObject $r.weight ) ) $r.unit }}
{{- end -}}
{{- end -}}
{{- end -}}

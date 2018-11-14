{{- define "jvm.memory" -}}
{{- $r := $.resources.requests.memory -}}
{{- $base := $.jvmHeapBase -}}
{{ printf "%d%s" ( add $base ( mul ( div $.objectCount $r.weight ) 79 $r.weight ) ) "m" }}
{{- end -}}
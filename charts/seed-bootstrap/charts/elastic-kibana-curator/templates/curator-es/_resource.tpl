{{- define "curator.disk_threshold" -}}
{{- printf "%d" ( add $.base_disk_threshold ( mul $.base_disk_threshold ( sub $.objectCount 1 ) ) ) -}}
{{- end -}}
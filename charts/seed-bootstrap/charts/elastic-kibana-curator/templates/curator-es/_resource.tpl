{{- define "curator.disk_threshold" -}}
{{ printf "%d" ( div ( mul (add ( mul 13 (div $.baseDiskThreshold 1000000) ) ( mul 1000 ( sub $.objectCount 1 ) ) ) 1000000 ) 13 ) }}
{{- end -}}

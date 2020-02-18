{{- define "init-containerd-config-script" -}}
#!/bin/bash

FILE=/etc/containerd/config.toml
if [ ! -f "$FILE" ]; then
  mkdir -p /etc/containerd
  containerd config default > "$FILE"
fi
{{- end -}}
{{- define "init-containerd-config-script" -}}
- path: /opt/bin/init-containerd
  permissions: 0744
  content:
    inline:
      encoding: ""
      data: |
        #!/bin/bash

        FILE=/etc/containerd/config.toml
        if [ ! -f "$FILE" ]; then
          mkdir -p /etc/containerd
          containerd config default > "$FILE"
        fi
{{- end -}}
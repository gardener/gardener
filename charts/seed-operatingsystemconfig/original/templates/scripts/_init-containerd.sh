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

        BIN_PATH={{ .Values.osc.cri.containerRuntimesBinaryPath }}
        mkdir -p $BIN_PATH

        ENV_FILE=/etc/systemd/system/containerd.service.d/30-env_config.conf
        if [ ! -f "$ENV_FILE" ]; then
          cat <<EOF | tee $ENV_FILE
        [Service]
        Environment="PATH=$BIN_PATH:$PATH"
        EOF
          systemctl daemon-reload
        fi
{{- end -}}
{{ define "terraformer-common.cloud-config.user-data" -}}
#cloud-config

coreos:
  update:
    reboot-strategy: off
  units:
  - name: cloud-config-downloader.service
    command: start
    enable: true
    content: |
      [Unit]
      Description=Downloads the real cloud-config from Shoot API Server and execute it
      After=docker.service docker.socket
      Wants=docker.socket
      [Service]
      Restart=always
      RestartSec=30
      EnvironmentFile=/etc/environment
      ExecStart=/bin/sh /var/lib/cloud-config-downloader/download-cloud-config.sh
write_files:
- path: /var/lib/cloud-config-downloader/kubeconfig
  permissions: 0644
  encoding: b64
  content: {{ ( required "cloudConfig.kubeconfig is required" .cloudConfig.kubeconfig ) | b64enc }}
- path: /var/lib/cloud-config-downloader/download-cloud-config.sh
  permissions: 0644
  encoding: b64
  content: {{ include "terraformer-common.cloud-config-downloader" . | b64enc }}
{{- end }}

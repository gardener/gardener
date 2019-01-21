{{ define "cloud-config.user-data" -}}
#cloud-config

coreos:
  update:
    reboot-strategy: off
  units:
  - name: update-engine.service
    mask: true
  - name: locksmithd.service
    mask: true
{{ include "logrotate" . | indent 2 }}
{{ include "docker-monitor" . | indent 2 }}
{{ include "kubelet" . | indent 2 }}
{{ include "kubelet-monitor" . | indent 2 }}
{{ include "update-ca-certs" . | indent 2 }}
{{ include "systemd-sysctl" . | indent 2 }}
write_files:
{{ include "logrotate-config" . }}
{{ include "journald-config" . }}
{{ include "kubelet-binary" . }}
{{ include "root-certs" . }}
{{ include "kernel-config" . }}
{{ include "health-monitor" . }}
{{- end }}

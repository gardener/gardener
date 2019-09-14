{{- define "seed-operatingsystemconfig.gardener-user" -}}
#!/bin/bash -eu

id gardener || useradd gardener -mU
mkdir -p /home/gardener/.ssh
find /home -name authorized_keys -not -path "/home/gardener/*" -exec cp -f {} /home/gardener/.ssh/authorized_keys \;
chown gardener:gardener /home/gardener/.ssh/authorized_keys
cat >/etc/sudoers.d/99-gardener-user <<EOF
gardener ALL=(ALL) NOPASSWD:ALL
EOF
{{- end }}

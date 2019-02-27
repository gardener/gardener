{{- define "shoot-cloud-config.execution-script" -}}
#!/bin/bash -eu

DIR_KUBELET="/var/lib/kubelet"
DIR_CLOUDCONFIG_DOWNLOADER="/var/lib/cloud-config-downloader"
DIR_CLOUDCONFIG="$DIR_CLOUDCONFIG_DOWNLOADER/downloads"

PATH_CLOUDCONFIG_DOWNLOADER_SERVER="$DIR_CLOUDCONFIG_DOWNLOADER/credentials/server"
PATH_CLOUDCONFIG_DOWNLOADER_CA_CERT="$DIR_CLOUDCONFIG_DOWNLOADER/credentials/ca.crt"
PATH_CLOUDCONFIG="{{ .configFilePath }}"
PATH_CLOUDCONFIG_OLD="${PATH_CLOUDCONFIG}.old"

mkdir -p "$DIR_CLOUDCONFIG" "$DIR_KUBELET"

function docker-preload() {
  name="$1"
  image="$2"
  echo "Checking whether to preload $name from $image"
  if [ -z $(docker images -q "$image") ]; then
    echo "Preloading $name from $image"
    docker pull "$image"
  else
    echo "No need to preload $name from $image"
  fi
}

{{ range $name, $image := (required ".images is required" .images) -}}
docker-preload "{{ $name }}" "{{ $image }}"
{{ end }}

cat << 'EOF' | base64 -d > "$PATH_CLOUDCONFIG"
{{ .worker.cloudConfig | b64enc }}
EOF

if [ ! -f "$PATH_CLOUDCONFIG_OLD" ]; then
  touch "$PATH_CLOUDCONFIG_OLD"
fi

if [[ ! -f "$DIR_KUBELET/kubeconfig-real" ]]; then
  cat <<EOF > "$DIR_KUBELET/kubeconfig-bootstrap"
---
apiVersion: v1
kind: Config
current-context: kubelet-bootstrap@default
clusters:
- cluster:
    certificate-authority-data: $(cat "$PATH_CLOUDCONFIG_DOWNLOADER_CA_CERT" | base64 | tr -d '\n')
    server: $(cat "$PATH_CLOUDCONFIG_DOWNLOADER_SERVER")
  name: default
contexts:
- context:
    cluster: default
    user: kubelet-bootstrap
  name: kubelet-bootstrap@default
users:
- name: kubelet-bootstrap
  user:
    as-user-extra: {}
    token: {{ required ".bootstrapToken is required" .bootstrapToken }}
EOF

else
  rm -f "$DIR_KUBELET/kubeconfig-bootstrap"
fi

if ! diff "$PATH_CLOUDCONFIG" "$PATH_CLOUDCONFIG_OLD" >/dev/null; then
  echo "Seen newer cloud config version"
  if {{ .worker.command }}; then
    echo "Successfully applied new cloud config version"
    systemctl daemon-reload
{{- range $name := (required ".worker.units is required" .worker.units) }}
{{- if ne $name "docker.service" }}
    systemctl restart {{ $name }}
{{- end }}
{{- end }}
    echo "Successfully restarted all units referenced in the cloud config."
    cp "$PATH_CLOUDCONFIG" "$PATH_CLOUDCONFIG_OLD"
  fi
fi

rm "$PATH_CLOUDCONFIG"
{{- end}}

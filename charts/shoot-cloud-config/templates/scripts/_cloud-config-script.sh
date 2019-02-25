{{- define "shoot-cloud-config.execution-script" -}}
#!/bin/bash -eu

DIR_KUBELET="/var/lib/kubelet"
DIR_CLOUDCONFIG_DOWNLOADER="/var/lib/cloud-config-downloader"
DIR_CLOUDCONFIG="$DIR_CLOUDCONFIG_DOWNLOADER/downloads"

PATH_KUBECONFIG="$DIR_CLOUDCONFIG_DOWNLOADER/credentials/kubeconfig"
PATH_CLOUDCONFIG="{{ .configFilePath }}"
PATH_CLOUDCONFIG_OLD="${PATH_CLOUDCONFIG}.old"

mkdir -p "$DIR_CLOUDCONFIG" "$DIR_KUBELET"

function docker-run() {
  /usr/bin/docker run --rm "$@"
}

function kubectl() {
  docker-run \
    --net host \
    -v "$DIR_CLOUDCONFIG"/:"$DIR_CLOUDCONFIG" \
    -v "$DIR_CLOUDCONFIG_DOWNLOADER"/:"$DIR_CLOUDCONFIG_DOWNLOADER" \
    -e "KUBECONFIG=$PATH_KUBECONFIG" \
    {{ index .images "hyperkube" }} \
    kubectl "$@"
}

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

if [[ ! -f "$DIR_KUBELET/kubeconfig-real" ]]; then
  if ! SERVER="$(kubectl config view -o go-template='{{ "{{" }}index .clusters 0 "cluster" "server"{{ "}}" }}' --raw)"; then
    echo "Could not retrieve the kube-apiserver address."
    exit 1
  fi
  if ! CA_CRT="$(kubectl config view -o go-template='{{ "{{" }}index .clusters 0 "cluster" "certificate-authority-data"{{ "}}" }}' --raw)"; then
    echo "Could not retrieve the CA certificate."
    exit 1
  fi

  cat <<EOF > "$DIR_KUBELET/kubeconfig-bootstrap"
---
apiVersion: v1
kind: Config
current-context: kubelet-bootstrap@default
clusters:
- cluster:
    certificate-authority-data: $CA_CRT
    server: $SERVER
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

if [ ! -f "$PATH_CLOUDCONFIG_OLD" ]; then
  touch "$PATH_CLOUDCONFIG_OLD"
fi

if ! diff "$PATH_CLOUDCONFIG" "$PATH_CLOUDCONFIG_OLD" >/dev/null; then
  echo "Seen newer cloud config version"
  if {{ .worker.command }}; then
    echo "Successfully applied new cloud config version"
    if [ "$(stat -c%s "$PATH_CLOUDCONFIG_OLD")" -ne "0" ]; then
      systemctl daemon-reload
{{- range $name := (required ".worker.units is required" .worker.units) }}
{{- if ne $name "docker.service" }}
      systemctl restart {{ $name }}
{{- end }}
{{- end }}
      echo "Successfully restarted all units referenced in the cloud config."
    fi
    cp "$PATH_CLOUDCONFIG" "$PATH_CLOUDCONFIG_OLD"
  fi
fi

rm "$PATH_CLOUDCONFIG"
{{- end}}

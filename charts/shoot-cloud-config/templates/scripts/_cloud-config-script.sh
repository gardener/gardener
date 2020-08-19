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

{{- if eq .worker.cri.name "docker" }}
function docker-image-preload(){
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
{{- else }}
function cri-image-preload() {
  name="$1"
  image="$2"
  echo "Preloading $name from $image"
  /usr/local/bin/crictl --image-endpoint {{ required "worker.cri.socket is required" .worker.cri.socket }} pull "$image"
}
{{- end }}

function preload-images(){
  {{- if eq .worker.cri.name "docker" }}
  preload="docker-image-preload"
  {{- else }}
  /usr/local/bin/crictl --image-endpoint {{ required "worker.cri.socket is required" .worker.cri.socket }} images > /dev/null
  if [ $? -ne 0 ];then
    echo "Unable to preload image - CRI or crictl unavailable."
    return 0
  fi
  preload="cri-image-preload"
  {{- end }}

  {{ range $name, $image := (required ".images is required" .images) -}}
  $preload "{{ $name }}" "{{ $image }}"
  {{ end -}}
}

{{- if .worker.kubeletDataVolume }}
function format-data-device() {
  LABEL=KUBEDEV
  if ! blkid --label $LABEL >/dev/null; then
    DEVICES=$(lsblk -dbsnP -o NAME,PARTTYPE,FSTYPE,SIZE)
    MATCHING_DEVICES=$(echo "$DEVICES" | grep 'PARTTYPE="".*FSTYPE="".*SIZE="{{.worker.kubeletDataVolume.size}}"')
    echo "Matching kubelet data device by size : {{.worker.kubeletDataVolume.size}}"
    TARGET_DEVICE_NAME=$(echo "$MATCHING_DEVICES" | head -n1 | cut -f2 -d\")
    echo "detected kubelet data device $TARGET_DEVICE_NAME"
    mkfs.xfs -L $LABEL /dev/$TARGET_DEVICE_NAME
    echo "formatted and labeled data device $TARGET_DEVICE_NAME"
    mkdir /tmp/varlibcp
    mount LABEL=$LABEL /tmp/varlibcp
    echo "mounted temp copy dir on data device $TARGET_DEVICE_NAME"
    cp -a /var/lib/* /tmp/varlibcp/
    umount /tmp/varlibcp
    echo "copied /var/lib to data device $TARGET_DEVICE_NAME"
    mount LABEL=$LABEL /var/lib
    echo "mounted /var/lib on data device $TARGET_DEVICE_NAME"
  fi
}

format-data-device
{{- end}}

preload-images

cat << 'EOF' | base64 -d > "$PATH_CLOUDCONFIG"
{{ .worker.cloudConfig | b64enc }}
EOF

if [ ! -f "$PATH_CLOUDCONFIG_OLD" ]; then
  touch "$PATH_CLOUDCONFIG_OLD"
fi

if [[ ! -f "$DIR_KUBELET/kubeconfig-real" ]] || [[ ! -f "$DIR_KUBELET/pki/kubelet-client-current.pem" ]]; then
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
{{- if and (ne $name "docker.service") (ne $name "var-lib.mount") }}
    systemctl enable {{ $name }} && systemctl restart --no-block {{ $name }}
{{- end }}
{{- end }}
    echo "Successfully restarted all units referenced in the cloud config."
    cp "$PATH_CLOUDCONFIG" "$PATH_CLOUDCONFIG_OLD"
  fi
fi

rm "$PATH_CLOUDCONFIG"
{{- end}}

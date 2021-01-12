{{- define "shoot-cloud-config.execution-script" -}}
#!/bin/bash -eu

DIR_KUBELET="/var/lib/kubelet"
DIR_CLOUDCONFIG_DOWNLOADER="/var/lib/cloud-config-downloader"
DIR_CLOUDCONFIG="$DIR_CLOUDCONFIG_DOWNLOADER/downloads"

PATH_CLOUDCONFIG_DOWNLOADER_SERVER="$DIR_CLOUDCONFIG_DOWNLOADER/credentials/server"
PATH_CLOUDCONFIG_DOWNLOADER_CA_CERT="$DIR_CLOUDCONFIG_DOWNLOADER/credentials/ca.crt"
PATH_CLOUDCONFIG="{{ .configFilePath }}"
PATH_CLOUDCONFIG_OLD="${PATH_CLOUDCONFIG}.old"
PATH_CHECKSUM="$DIR_CLOUDCONFIG_DOWNLOADER/downloaded_checksum"

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

{{- if .worker.kubeletDataVolume }}
function format-data-device() {
  LABEL=KUBEDEV
  if ! blkid --label $LABEL >/dev/null; then
    DEVICES=$(lsblk -dbsnP -o NAME,PARTTYPE,FSTYPE,SIZE)
    MATCHING_DEVICES=$(echo "$DEVICES" | grep 'PARTTYPE="".*FSTYPE="".*SIZE="{{.worker.kubeletDataVolume.size}}"')
    echo "Matching kubelet data device by size : {{.worker.kubeletDataVolume.size}}"
    TARGET_DEVICE_NAME=$(echo "$MATCHING_DEVICES" | head -n1 | cut -f2 -d\")
    echo "detected kubelet data device $TARGET_DEVICE_NAME"
    mkfs.ext4 -L $LABEL -O quota -E lazy_itable_init=0,lazy_journal_init=0,quotatype=usrquota:grpquota:prjquota  /dev/$TARGET_DEVICE_NAME
    echo "formatted and labeled data device $TARGET_DEVICE_NAME"
    mkdir /tmp/varlibcp
    mount LABEL=$LABEL /tmp/varlibcp
    echo "mounted temp copy dir on data device $TARGET_DEVICE_NAME"
    cp -a /var/lib/* /tmp/varlibcp/
    umount /tmp/varlibcp
    echo "copied /var/lib to data device $TARGET_DEVICE_NAME"
    mount LABEL=$LABEL /var/lib -o defaults,prjquota,errors=remount-ro
    echo "mounted /var/lib on data device $TARGET_DEVICE_NAME"
  fi
}

format-data-device
{{- end}}

{{ range $name, $image := (required ".images is required" .images) -}}
docker-preload "{{ $name }}" "{{ $image }}"
{{ end }}

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

ANNOTATION_RESTART_SYSTEMD_SERVICES="worker.gardener.cloud/restart-systemd-services"

# Try to find Node object for this machine
if [[ -f "$DIR_KUBELET/kubeconfig-real" ]]; then
  {{`NODE="$(/opt/bin/kubectl --kubeconfig="$DIR_KUBELET/kubeconfig-real" get node -o go-template="{{ range \$i, \$item := .items }}{{ range \$j, \$address := \$item.status.addresses }}{{ if and (eq \$address.type \"Hostname\") (eq \$address.address \"$(hostname)\") }}{{ \$item.metadata.name }}{{ if (index \$item.metadata.annotations \"$ANNOTATION_RESTART_SYSTEMD_SERVICES\") }} {{ index \$item.metadata.annotations \"$ANNOTATION_RESTART_SYSTEMD_SERVICES\" }}{{ end }}{{ end }}{{ end }}{{ end }}")"`}}

  if [[ ! -z "$NODE" ]]; then
    NODENAME="$(echo "$NODE" | awk '{print $1}')"
    SYSTEMD_SERVICES_TO_RESTART="$(echo "$NODE" | awk '{print $2}')"
  fi

  # Update checksum/cloud-config-data annotation on Node object if possible
  if [[ ! -z "$NODENAME" ]] && [[ -f "$PATH_CHECKSUM" ]]; then
    /opt/bin/kubectl --kubeconfig="$DIR_KUBELET/kubeconfig-real" annotate node "$NODENAME" "checksum/cloud-config-data=$(cat "$PATH_CHECKSUM")" --overwrite
  fi

  # Restart systemd services if requested
  for service in $(echo "$SYSTEMD_SERVICES_TO_RESTART" | sed "s/,/ /g"); do
    echo "Restarting systemd service $service due to $ANNOTATION_RESTART_SYSTEMD_SERVICES annotation"
    systemctl restart "$service"
  done && /opt/bin/kubectl --kubeconfig="$DIR_KUBELET/kubeconfig-real" annotate node "$NODENAME" "${ANNOTATION_RESTART_SYSTEMD_SERVICES}-"
fi
{{- end}}

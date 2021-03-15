#!/bin/bash -eu

PATH_CLOUDCONFIG_DOWNLOADER_SERVER="{{ .pathCredentialsServer }}"
PATH_CLOUDCONFIG_DOWNLOADER_CA_CERT="{{ .pathCredentialsCACert }}"
PATH_CLOUDCONFIG="{{ .pathDownloadedCloudConfig }}"
PATH_CLOUDCONFIG_OLD="${PATH_CLOUDCONFIG}.old"
PATH_CHECKSUM="{{ .pathDownloadedChecksum }}"
PATH_CCD_SCRIPT="{{ .pathCCDScript }}"
PATH_CCD_SCRIPT_CHECKSUM="{{ .pathCCDScriptChecksum }}"
PATH_CCD_SCRIPT_CHECKSUM_OLD="${PATH_CCD_SCRIPT_CHECKSUM}.old"
PATH_EXECUTION_DELAY_SECONDS="{{ .pathExecutionDelaySeconds }}"
PATH_EXECUTION_LAST_DATE="{{ .pathExecutionLastDate }}"

mkdir -p "{{ .pathDownloadsDirectory }}" "{{ .pathKubeletDirectory }}"

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

{{ if .kubeletDataVolume -}}
function format-data-device() {
  LABEL=KUBEDEV
  if ! blkid --label $LABEL >/dev/null; then
    DEVICES=$(lsblk -dbsnP -o NAME,PARTTYPE,FSTYPE,SIZE)
    MATCHING_DEVICES=$(echo "$DEVICES" | grep 'PARTTYPE="".*FSTYPE="".*SIZE="{{ .kubeletDataVolume.size }}"')
    echo "Matching kubelet data device by size : {{ .kubeletDataVolume.size }}"
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
{{- end }}

{{ range $name, $image := .images -}}
docker-preload "{{ $name }}" "{{ $image }}"
{{ end }}

cat << 'EOF' | base64 -d > "$PATH_CLOUDCONFIG"
{{ .cloudConfigUserData }}
EOF

if [ ! -f "$PATH_CLOUDCONFIG_OLD" ]; then
  touch "$PATH_CLOUDCONFIG_OLD"
fi

if [ ! -f "$PATH_CCD_SCRIPT_CHECKSUM_OLD" ]; then
  touch "$PATH_CCD_SCRIPT_CHECKSUM_OLD"
fi

md5sum ${PATH_CCD_SCRIPT} > ${PATH_CCD_SCRIPT_CHECKSUM}

if [[ ! -f "{{ .pathKubeletKubeconfigReal }}" ]] || [[ ! -f "{{ .pathKubeletDirectory }}/pki/kubelet-client-current.pem" ]]; then
  cat <<EOF > "{{ .pathKubeletKubeconfigBootstrap }}"
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
    token: {{ .bootstrapToken }}
EOF

else
  rm -f "{{ .pathKubeletKubeconfigBootstrap }}"
fi

NODENAME=
ANNOTATION_RESTART_SYSTEMD_SERVICES="worker.gardener.cloud/restart-systemd-services"

# Try to find Node object for this machine if already registered to the cluster.
if [[ -f "{{ .pathKubeletKubeconfigReal }}" ]]; then
  {{`NODE="$(/opt/bin/kubectl --kubeconfig="`}}{{ .pathKubeletKubeconfigReal }}{{`" get node -l "kubernetes.io/hostname=$(hostname)" -o go-template="{{ if .items }}{{ (index .items 0).metadata.name }}{{ if (index (index .items 0).metadata.annotations \"$ANNOTATION_RESTART_SYSTEMD_SERVICES\") }} {{ index (index .items 0).metadata.annotations \"$ANNOTATION_RESTART_SYSTEMD_SERVICES\" }}{{ end }}{{ end }}")"`}}

  if [[ ! -z "$NODE" ]]; then
    NODENAME="$(echo "$NODE" | awk '{print $1}')"
    SYSTEMD_SERVICES_TO_RESTART="$(echo "$NODE" | awk '{print $2}')"
  fi

  # Restart systemd services if requested
  restart_ccd=n
  for service in $(echo "$SYSTEMD_SERVICES_TO_RESTART" | sed "s/,/ /g"); do
    if [[ ${service} == {{ .cloudConfigDownloaderName }}* ]]; then
      restart_ccd=y
      continue
    fi
    echo "Restarting systemd service $service due to $ANNOTATION_RESTART_SYSTEMD_SERVICES annotation"
    systemctl restart "$service" || true
  done
  /opt/bin/kubectl --kubeconfig="{{ .pathKubeletKubeconfigReal }}" annotate node "$NODENAME" "${ANNOTATION_RESTART_SYSTEMD_SERVICES}-"
  if [[ ${restart_ccd} == "y" ]]; then
    echo "Restarting systemd service {{ .unitNameCloudConfigDownloader }} due to $ANNOTATION_RESTART_SYSTEMD_SERVICES annotation"
    systemctl restart "{{ .unitNameCloudConfigDownloader }}" || true
  fi

  # If the time difference from the last execution till now is smaller than the node-specific delay then we exit early
  # and don't apply the (potentially updated) cloud-config user data. This is to spread the restarts of the systemd
  # units and to prevent too many restarts happening on the nodes at roughly the same time.
  if [[ ! -f "$PATH_EXECUTION_DELAY_SECONDS" ]]; then
    echo $(({{ .executionMinDelaySeconds }} + $RANDOM % {{ .executionMaxDelaySeconds }})) > "$PATH_EXECUTION_DELAY_SECONDS"
  fi
  execution_delay_seconds=$(cat "$PATH_EXECUTION_DELAY_SECONDS")

  if [[ -f "$PATH_EXECUTION_LAST_DATE" ]]; then
    execution_last_date=$(cat "$PATH_EXECUTION_LAST_DATE")
    now_date=$(date +%s)

    if [[ $((now_date - execution_last_date)) -lt $execution_delay_seconds ]]; then
      echo "$(date) Execution delay for this node is $execution_delay_seconds seconds, and the last execution was at $(date -d @$execution_last_date). Exiting now."
      exit 0
    fi
  fi
fi

# Apply most recent cloud-config user-data, reload the systemd daemon and restart the units to make the changes
# effective.
if ! diff "$PATH_CLOUDCONFIG" "$PATH_CLOUDCONFIG_OLD" >/dev/null || ! diff "$PATH_CCD_SCRIPT_CHECKSUM" "$PATH_CCD_SCRIPT_CHECKSUM_OLD" >/dev/null; then
  echo "Seen newer cloud config or cloud config downloader version"
  if {{ .reloadConfigCommand }}; then
    echo "Successfully applied new cloud config version"
    systemctl daemon-reload
{{- range $name := .units }}
{{- if and (ne $name $.unitNameDocker) (ne $name $.unitNameVarLibMount) (ne $name $.unitNameCloudConfigDownloader) }}
    systemctl enable {{ $name }} && systemctl restart --no-block {{ $name }}
{{- end }}
{{- end }}
    echo "Successfully restarted all units referenced in the cloud config."
    cp "$PATH_CLOUDCONFIG" "$PATH_CLOUDCONFIG_OLD"
    md5sum ${PATH_CCD_SCRIPT} > "$PATH_CCD_SCRIPT_CHECKSUM_OLD" # As the file can be updated above, get fresh checksum.
  fi
fi

rm "$PATH_CLOUDCONFIG" "$PATH_CCD_SCRIPT_CHECKSUM"

# Now that the most recent cloud-config user data was applied, let's update the checksum/cloud-config-data annotation on
# the Node object if possible and store the current date.
if [[ ! -z "$NODENAME" ]] && [[ -f "$PATH_CHECKSUM" ]]; then
  /opt/bin/kubectl --kubeconfig="{{ .pathKubeletKubeconfigReal }}" annotate node "$NODENAME" "checksum/cloud-config-data=$(cat "$PATH_CHECKSUM")" --overwrite
fi
date +%s > "$PATH_EXECUTION_LAST_DATE"

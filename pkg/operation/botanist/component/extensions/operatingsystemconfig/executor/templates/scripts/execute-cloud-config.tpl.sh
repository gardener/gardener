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
PATH_HYPERKUBE_DOWNLOADS="{{ .pathHyperkubeDownloads }}"
PATH_LAST_DOWNLOADED_HYPERKUBE_IMAGE="{{ .pathLastDownloadedHyperkubeImage }}"
PATH_HYPERKUBE_IMAGE_USED_FOR_LAST_COPY_KUBELET="{{ .pathHyperKubeImageUsedForLastCopyKubelet }}"
PATH_HYPERKUBE_IMAGE_USED_FOR_LAST_COPY_KUBECTL="{{ .pathHyperKubeImageUsedForLastCopyKubectl }}"

mkdir -p "{{ .pathDownloadsDirectory }}" "{{ .pathKubeletDirectory }}" "$PATH_HYPERKUBE_DOWNLOADS"

{{ if .kubeletDataVolume -}}
function format-data-device() {
  LABEL=KUBEDEV
  if ! blkid --label $LABEL >/dev/null; then
    DISK_DEVICES=$(lsblk -dbsnP -o NAME,PARTTYPE,FSTYPE,SIZE,PATH,TYPE | grep 'TYPE="disk"')
    while IFS= read -r line; do
      MATCHING_DEVICE_CANDIDATE=$(echo "$line" | grep 'PARTTYPE="".*FSTYPE="".*SIZE="{{ .kubeletDataVolume.size }}"')
      if [ -z "$MATCHING_DEVICE_CANDIDATE" ]; then
        continue
      fi

      # Skip device if it's already mounted.
      DEVICE_NAME=$(echo "$MATCHING_DEVICE_CANDIDATE" | cut -f2 -d\")
      DEVICE_MOUNTS=$(lsblk -dbsnP -o NAME,MOUNTPOINT,TYPE | grep -e "^NAME=\"$DEVICE_NAME.*\".*TYPE=\"part\"$")
      if echo "$DEVICE_MOUNTS" | awk '{print $2}' | grep "MOUNTPOINT=\"\/.*\"" > /dev/null; then
        continue
      fi

      TARGET_DEVICE_NAME="$DEVICE_NAME"
      break
    done <<< "$DISK_DEVICES"

    if [ -z "$TARGET_DEVICE_NAME" ]; then
      echo "No kubelet data device found"
      return
    fi

    echo "Matching kubelet data device by size : {{ .kubeletDataVolume.size }}"
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

LAST_DOWNLOADED_HYPERKUBE_IMAGE=""
if [[ -f "$PATH_LAST_DOWNLOADED_HYPERKUBE_IMAGE" ]]; then
  LAST_DOWNLOADED_HYPERKUBE_IMAGE="$(cat "$PATH_LAST_DOWNLOADED_HYPERKUBE_IMAGE")"
fi

HYPERKUBE_IMAGE_USED_FOR_LAST_COPY_KUBELET=""
if [[ -f "$PATH_HYPERKUBE_IMAGE_USED_FOR_LAST_COPY_KUBELET" ]]; then
  HYPERKUBE_IMAGE_USED_FOR_LAST_COPY_KUBELET="$(cat "$PATH_HYPERKUBE_IMAGE_USED_FOR_LAST_COPY_KUBELET")"
fi

HYPERKUBE_IMAGE_USED_FOR_LAST_COPY_KUBECTL=""
if [[ -f "$PATH_HYPERKUBE_IMAGE_USED_FOR_LAST_COPY_KUBECTL" ]]; then
  HYPERKUBE_IMAGE_USED_FOR_LAST_COPY_KUBECTL="$(cat "$PATH_HYPERKUBE_IMAGE_USED_FOR_LAST_COPY_KUBECTL")"
fi

echo "Checking whether we need to preload a new hyperkube image..."
if [[ "$LAST_DOWNLOADED_HYPERKUBE_IMAGE" != "{{ .hyperkubeImage }}" ]]; then
  echo "Preloading hyperkube image ({{ .hyperkubeImage }}) because last downloaded image ($LAST_DOWNLOADED_HYPERKUBE_IMAGE) is outdated"
  {{ .pathDockerBinary }} pull "{{ .hyperkubeImage }}"

  echo "Starting temporary hyperkube container to copy binaries to host"

{{- if semverCompare "< 1.17" .kubernetesVersion }}
  {{ .pathDockerBinary }} run --rm -v "$PATH_HYPERKUBE_DOWNLOADS":"$PATH_HYPERKUBE_DOWNLOADS":rw "{{ .hyperkubeImage }}" /bin/sh -c "cp /usr/local/bin/kubelet $PATH_HYPERKUBE_DOWNLOADS"
  {{ .pathDockerBinary }} run --rm -v "$PATH_HYPERKUBE_DOWNLOADS":"$PATH_HYPERKUBE_DOWNLOADS":rw "{{ .hyperkubeImage }}" /bin/sh -c "cp /usr/local/bin/kubectl $PATH_HYPERKUBE_DOWNLOADS"
{{- else if semverCompare "< 1.19" .kubernetesVersion }}
  {{ .pathDockerBinary }} run --rm -v "$PATH_HYPERKUBE_DOWNLOADS":"$PATH_HYPERKUBE_DOWNLOADS":rw --entrypoint /bin/sh "{{ .hyperkubeImage }}" -c "cp /usr/local/bin/kubelet $PATH_HYPERKUBE_DOWNLOADS"
  {{ .pathDockerBinary }} run --rm -v "$PATH_HYPERKUBE_DOWNLOADS":"$PATH_HYPERKUBE_DOWNLOADS":rw --entrypoint /bin/sh "{{ .hyperkubeImage }}" -c "cp /usr/local/bin/kubectl $PATH_HYPERKUBE_DOWNLOADS"
{{- else }}
  HYPERKUBE_CONTAINER_ID="$({{ .pathDockerBinary }} run --rm -d -v "$PATH_HYPERKUBE_DOWNLOADS":"$PATH_HYPERKUBE_DOWNLOADS":rw "{{ .hyperkubeImage }}")"
  {{ .pathDockerBinary }} cp   "$HYPERKUBE_CONTAINER_ID":/kubelet "$PATH_HYPERKUBE_DOWNLOADS"
  {{ .pathDockerBinary }} cp   "$HYPERKUBE_CONTAINER_ID":/kubectl "$PATH_HYPERKUBE_DOWNLOADS"
  {{ .pathDockerBinary }} stop "$HYPERKUBE_CONTAINER_ID"
{{- end }}
  chmod +x "$PATH_HYPERKUBE_DOWNLOADS/kubelet"
  chmod +x "$PATH_HYPERKUBE_DOWNLOADS/kubectl"

  echo "{{ .hyperkubeImage }}" > "$PATH_LAST_DOWNLOADED_HYPERKUBE_IMAGE"
  LAST_DOWNLOADED_HYPERKUBE_IMAGE="$(cat "$PATH_LAST_DOWNLOADED_HYPERKUBE_IMAGE")"
else
  echo "No need to preload new hyperkube image because binaries for $LAST_DOWNLOADED_HYPERKUBE_IMAGE were found in $PATH_HYPERKUBE_DOWNLOADS"
fi

cat << 'EOF' | base64 -d > "{{ .pathScriptCopyKubernetesBinary }}"
{{ .scriptCopyKubernetesBinary }}
EOF
chmod +x "{{ .pathScriptCopyKubernetesBinary }}"

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
  BOOTSTRAP_TOKEN="{{ .bootstrapToken }}"
  # If a bootstrap token file exists and the placeholder got replaced by the Worker extension then use it
  if [[ -f "{{ .pathBootstrapToken }}" ]]; then
    FILE_CONTENT="$(cat "{{ .pathBootstrapToken }}")"
    if [[ $FILE_CONTENT != "{{ .bootstrapTokenPlaceholder }}" ]] && [[ $FILE_CONTENT != "{{ .bootstrapTokenPlaceholderB64 }}" ]]; then
      BOOTSTRAP_TOKEN="$FILE_CONTENT"
    fi
  fi

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
    token: "$BOOTSTRAP_TOKEN"
EOF

else
  rm -f "{{ .pathKubeletKubeconfigBootstrap }}"
  rm -f "{{ .pathBootstrapToken }}"
fi

NODENAME=
ANNOTATION_RESTART_SYSTEMD_SERVICES="worker.gardener.cloud/restart-systemd-services"

# Try to find Node object for this machine if already registered to the cluster.
if [[ -f "{{ .pathKubeletKubeconfigReal }}" ]]; then
  {{`NODE="$(`}}{{ .pathBinaries }}{{`/kubectl --kubeconfig="`}}{{ .pathKubeletKubeconfigReal }}{{`" get node -l "kubernetes.io/hostname=$(hostname)" -o go-template="{{ if .items }}{{ (index .items 0).metadata.name }}{{ if (index (index .items 0).metadata.annotations \"$ANNOTATION_RESTART_SYSTEMD_SERVICES\") }} {{ index (index .items 0).metadata.annotations \"$ANNOTATION_RESTART_SYSTEMD_SERVICES\" }}{{ end }}{{ end }}")"`}}

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
  {{ .pathBinaries }}/kubectl --kubeconfig="{{ .pathKubeletKubeconfigReal }}" annotate node "$NODENAME" "${ANNOTATION_RESTART_SYSTEMD_SERVICES}-"
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
if ! diff "$PATH_CLOUDCONFIG" "$PATH_CLOUDCONFIG_OLD" >/dev/null || \
   ! diff "$PATH_CCD_SCRIPT_CHECKSUM" "$PATH_CCD_SCRIPT_CHECKSUM_OLD" >/dev/null || \
   [[ "$HYPERKUBE_IMAGE_USED_FOR_LAST_COPY_KUBELET" != "$LAST_DOWNLOADED_HYPERKUBE_IMAGE" ]] ||
   [[ "$HYPERKUBE_IMAGE_USED_FOR_LAST_COPY_KUBECTL" != "$LAST_DOWNLOADED_HYPERKUBE_IMAGE" ]]; then

  echo "Seen newer cloud config or cloud config downloader version or hyperkube image"
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
  else
    echo "failed to apply the cloud config."
    exit 1
  fi
fi

echo "Cloud config is up to date."
rm "$PATH_CLOUDCONFIG" "$PATH_CCD_SCRIPT_CHECKSUM"

# Now that the most recent cloud-config user data was applied, let's update the checksum/cloud-config-data annotation on
# the Node object if possible and store the current date.
if [[ ! -z "$NODENAME" ]] && [[ -f "$PATH_CHECKSUM" ]]; then
  {{ .pathBinaries }}/kubectl --kubeconfig="{{ .pathKubeletKubeconfigReal }}" annotate node "$NODENAME" "checksum/cloud-config-data=$(cat "$PATH_CHECKSUM")" --overwrite
fi
date +%s > "$PATH_EXECUTION_LAST_DATE"

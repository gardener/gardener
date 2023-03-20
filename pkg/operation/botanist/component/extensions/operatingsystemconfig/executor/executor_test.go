// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package executor_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/operatingsystemconfig/downloader"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/operatingsystemconfig/executor"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/operatingsystemconfig/original/components/docker"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/operatingsystemconfig/original/components/varlibmount"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/imagevector"
)

var _ = Describe("Executor", func() {
	Describe("#Script", func() {
		var (
			cloudConfigUserData                 []byte
			cloudConfigExecutionMaxDelaySeconds int
			hyperkubeImage                      *imagevector.Image
			reloadConfigCommand                 string
			units                               []string

			defaultKubeletDataVolume     = &gardencorev1beta1.DataVolume{VolumeSize: "64Gi"}
			defaultKubeletDataVolumeSize = pointer.String("68719476736")
		)

		BeforeEach(func() {
			cloudConfigUserData = []byte("user-data")
			cloudConfigExecutionMaxDelaySeconds = 300
			hyperkubeImage = &imagevector.Image{Repository: "bar", Tag: pointer.String("v1.0")}
			reloadConfigCommand = "/var/bin/reload"
			units = []string{
				docker.UnitName,
				"unit1",
				"unit2",
				varlibmount.UnitName,
				"unit3",
				downloader.UnitName,
				"unit4",
			}
		})

		DescribeTable("should correctly render the executor script",
			func(kubernetesVersion string, copyKubernetesBinariesFn func(*imagevector.Image) string, kubeletDataVol *gardencorev1beta1.DataVolume, kubeletDataVolSize *string) {
				script, err := executor.Script(cloudConfigUserData, cloudConfigExecutionMaxDelaySeconds, hyperkubeImage, kubernetesVersion, kubeletDataVol, reloadConfigCommand, units)
				Expect(err).ToNot(HaveOccurred())
				testScript := scriptFor(cloudConfigUserData, hyperkubeImage, kubernetesVersion, copyKubernetesBinariesFn, kubeletDataVolSize, reloadConfigCommand, units)
				Expect(string(script)).To(Equal(testScript))
			},

			Entry("k8s 1.20, w/o kubelet data volume", "1.20.6", copyKubernetesBinariesFromHyperkubeImageForVersionsGreaterEqual119, nil, nil),
			Entry("k8s 1.20, w/ kubelet data volume", "1.20.6", copyKubernetesBinariesFromHyperkubeImageForVersionsGreaterEqual119, defaultKubeletDataVolume, defaultKubeletDataVolumeSize),

			Entry("k8s 1.21, w/o kubelet data volume", "1.21.7", copyKubernetesBinariesFromHyperkubeImageForVersionsGreaterEqual119, nil, nil),
			Entry("k8s 1.21, w/ kubelet data volume", "1.21.7", copyKubernetesBinariesFromHyperkubeImageForVersionsGreaterEqual119, defaultKubeletDataVolume, defaultKubeletDataVolumeSize),

			Entry("k8s 1.22, w/o kubelet data volume", "1.22.8", copyKubernetesBinariesFromHyperkubeImageForVersionsGreaterEqual119, nil, nil),
			Entry("k8s 1.22, w/ kubelet data volume", "1.22.8", copyKubernetesBinariesFromHyperkubeImageForVersionsGreaterEqual119, defaultKubeletDataVolume, defaultKubeletDataVolumeSize),
		)

		It("should return an error because the data volume size cannot be parsed", func() {
			kubeletDataVolume := &gardencorev1beta1.DataVolume{VolumeSize: "not-parsable"}

			script, err := executor.Script(cloudConfigUserData, cloudConfigExecutionMaxDelaySeconds, hyperkubeImage, "1.2.3", kubeletDataVolume, reloadConfigCommand, units)
			Expect(script).To(BeNil())
			Expect(err).To(MatchError(ContainSubstring("quantities must match the regular expression")))
		})
	})

	Describe("#Secret", func() {
		It("should return a secret object", func() {
			var (
				name      = "name"
				namespace = "namespace"
				poolName  = "pool"
				script    = []byte("script")
			)

			Expect(executor.Secret(name, namespace, poolName, script)).To(Equal(&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
					Annotations: map[string]string{
						"checksum/data-script": "762ab2979c97da7384f04a460a2c09c4e6c8aa25cdc2c9845800f2ee644a3e62",
					},
					Labels: map[string]string{
						"gardener.cloud/role":        "cloud-config",
						"worker.gardener.cloud/pool": poolName,
					},
				},
				Data: map[string][]byte{
					downloader.DataKeyScript: script,
				},
			}))
		})
	})
})

func scriptFor(
	cloudConfigUserData []byte,
	hyperkubeImage *imagevector.Image,
	kubernetesVersion string,
	copyKubernetesBinariesFn func(*imagevector.Image) string,
	kubeletDataVolumeSize *string,
	reloadConfigCommand string,
	units []string,
) string {
	headerPart := `#!/bin/bash -eu

PATH_CLOUDCONFIG_DOWNLOADER_SERVER="/var/lib/cloud-config-downloader/credentials/server"
PATH_CLOUDCONFIG_DOWNLOADER_CA_CERT="/var/lib/cloud-config-downloader/credentials/ca.crt"
PATH_CLOUDCONFIG="/var/lib/cloud-config-downloader/downloads/cloud_config"
PATH_CLOUDCONFIG_OLD="${PATH_CLOUDCONFIG}.old"
PATH_CHECKSUM="/var/lib/cloud-config-downloader/downloads/execute-cloud-config-checksum"
PATH_CCD_SCRIPT="/var/lib/cloud-config-downloader/download-cloud-config.sh"
PATH_CCD_SCRIPT_CHECKSUM="/var/lib/cloud-config-downloader/download-cloud-config.md5"
PATH_CCD_SCRIPT_CHECKSUM_OLD="${PATH_CCD_SCRIPT_CHECKSUM}.old"
PATH_EXECUTION_DELAY_SECONDS="/var/lib/cloud-config-downloader/execution_delay_seconds"
PATH_EXECUTION_LAST_DATE="/var/lib/cloud-config-downloader/execution_last_date"
PATH_HYPERKUBE_DOWNLOADS="/var/lib/cloud-config-downloader/downloads/hyperkube"
PATH_LAST_DOWNLOADED_HYPERKUBE_IMAGE="/var/lib/cloud-config-downloader/downloads/hyperkube/last_downloaded_hyperkube_image"
PATH_HYPERKUBE_IMAGE_USED_FOR_LAST_COPY_KUBELET="/opt/bin/hyperkube_image_used_for_last_copy_of_kubelet"
PATH_HYPERKUBE_IMAGE_USED_FOR_LAST_COPY_KUBECTL="/opt/bin/hyperkube_image_used_for_last_copy_of_kubectl"

mkdir -p "/var/lib/kubelet" "$PATH_HYPERKUBE_DOWNLOADS"

`

	kubeletDataVolumePart := ""
	if kubeletDataVolumeSize != nil {
		kubeletDataVolumePart = `function format-data-device() {
  LABEL=KUBEDEV
  if ! blkid --label $LABEL >/dev/null; then
    DISK_DEVICES=$(lsblk -dbsnP -o NAME,PARTTYPE,FSTYPE,SIZE,PATH,TYPE | grep 'TYPE="disk"')
    while IFS= read -r line; do
      MATCHING_DEVICE_CANDIDATE=$(echo "$line" | grep 'PARTTYPE="".*FSTYPE="".*SIZE="` + *kubeletDataVolumeSize + `"')
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

    echo "Matching kubelet data device by size : ` + *kubeletDataVolumeSize + `"
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

format-data-device`
	}

	imagesPreloadingPart := `

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
if [[ "$LAST_DOWNLOADED_HYPERKUBE_IMAGE" != "` + hyperkubeImage.String() + `" ]]; then
  if ! /usr/bin/docker info &> /dev/null ; then
    echo "docker daemon is not available, cannot preload hyperkube image"
    exit 1
  fi

  echo "Preloading hyperkube image (` + hyperkubeImage.String() + `) because last downloaded image ($LAST_DOWNLOADED_HYPERKUBE_IMAGE) is outdated"
  if ! /usr/bin/docker pull "` + hyperkubeImage.String() + `" ; then
    echo "hyperkube image preload failed"
    exit 1
  fi

  # append image reference checksum to copied filenames in order to easily check if copying the binaries succeeded
  hyperkubeImageSHA="7eb590a802776879e4db84c42ec60a2bd2094659ed3773252753287b880912fb"

  echo "Starting temporary hyperkube container to copy binaries to host"` +
		copyKubernetesBinariesFn(hyperkubeImage) + `
  chmod +x "$PATH_HYPERKUBE_DOWNLOADS/kubelet-$hyperkubeImageSHA"
  chmod +x "$PATH_HYPERKUBE_DOWNLOADS/kubectl-$hyperkubeImageSHA"

  if ! [ -f "$PATH_HYPERKUBE_DOWNLOADS/kubelet-$hyperkubeImageSHA" -a -f "$PATH_HYPERKUBE_DOWNLOADS/kubectl-$hyperkubeImageSHA" ]; then
    echo "extracting kubernetes binaries from hyperkube image failed"
    exit 1
  fi

  # only write to $PATH_LAST_DOWNLOADED_HYPERKUBE_IMAGE if copy operation succeeded
  # this is done to retry failed operations on the execution
  mv "$PATH_HYPERKUBE_DOWNLOADS/kubelet-$hyperkubeImageSHA" "$PATH_HYPERKUBE_DOWNLOADS/kubelet" && \
    mv "$PATH_HYPERKUBE_DOWNLOADS/kubectl-$hyperkubeImageSHA" "$PATH_HYPERKUBE_DOWNLOADS/kubectl" && \
    echo "` + hyperkubeImage.String() + `" > "$PATH_LAST_DOWNLOADED_HYPERKUBE_IMAGE"

  LAST_DOWNLOADED_HYPERKUBE_IMAGE="$(cat "$PATH_LAST_DOWNLOADED_HYPERKUBE_IMAGE")"
else
  echo "No need to preload new hyperkube image because binaries for $LAST_DOWNLOADED_HYPERKUBE_IMAGE were found in $PATH_HYPERKUBE_DOWNLOADS"
fi

cat << 'EOF' | base64 -d > "/var/lib/kubelet/copy-kubernetes-binary.sh"
` + utils.EncodeBase64([]byte(scriptCopyKubernetesBinary)) + `
EOF
chmod +x "/var/lib/kubelet/copy-kubernetes-binary.sh"
`

	footerPart := `
cat << 'EOF' | base64 -d > "$PATH_CLOUDCONFIG"
` + utils.EncodeBase64(cloudConfigUserData) + `
EOF

if [ ! -f "$PATH_CLOUDCONFIG_OLD" ]; then
  touch "$PATH_CLOUDCONFIG_OLD"
fi

if [ ! -f "$PATH_CCD_SCRIPT_CHECKSUM_OLD" ]; then
  touch "$PATH_CCD_SCRIPT_CHECKSUM_OLD"
fi

md5sum ${PATH_CCD_SCRIPT} > ${PATH_CCD_SCRIPT_CHECKSUM}

if [[ ! -f "/var/lib/kubelet/kubeconfig-real" ]] || [[ ! -f "/var/lib/kubelet/pki/kubelet-client-current.pem" ]]; then
  cat <<EOF > "/var/lib/kubelet/kubeconfig-bootstrap"
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
    token: "$(cat "/var/lib/cloud-config-downloader/credentials/bootstrap-token")"
EOF

else
  rm -f "/var/lib/kubelet/kubeconfig-bootstrap"
  rm -f "/var/lib/cloud-config-downloader/credentials/bootstrap-token"
fi

# Try to find Node object for this machine if already registered to the cluster.
NODENAME=
if [[ -s "/var/lib/kubelet/nodename" ]] && [[ ! -z "$(cat "/var/lib/kubelet/nodename")" ]]; then
  NODENAME="$(cat "/var/lib/kubelet/nodename")"
elif [[ -f "/var/lib/kubelet/kubeconfig-real" ]]; then
  NODENAME="$(/opt/bin/kubectl --kubeconfig="/var/lib/kubelet/kubeconfig-real" get nodes -l "kubernetes.io/hostname=$(hostname | tr '[:upper:]' '[:lower:]')" -o go-template="{{ if .items }}{{ (index .items 0).metadata.name }}{{ end }}")"
  echo -n "$NODENAME" > "/var/lib/kubelet/nodename"
fi

# Check if node is annotated with information about to-be-restarted systemd services
ANNOTATION_RESTART_SYSTEMD_SERVICES="worker.gardener.cloud/restart-systemd-services"
if [[ -f "/var/lib/kubelet/kubeconfig-real" ]]; then
  if [[ ! -z "$NODENAME" ]]; then
    SYSTEMD_SERVICES_TO_RESTART="$(/opt/bin/kubectl --kubeconfig="/var/lib/kubelet/kubeconfig-real" get node "$NODENAME" -o go-template="{{ if index .metadata.annotations \"$ANNOTATION_RESTART_SYSTEMD_SERVICES\" }}{{ index .metadata.annotations \"$ANNOTATION_RESTART_SYSTEMD_SERVICES\" }}{{ end }}")"

    # Restart systemd services if requested
    if [[ ! -z "$SYSTEMD_SERVICES_TO_RESTART" ]]; then
      restart_ccd=n
      for service in $(echo "$SYSTEMD_SERVICES_TO_RESTART" | sed "s/,/ /g"); do
        if [[ ${service} == cloud-config-downloader* ]]; then
          restart_ccd=y
          continue
        fi

        echo "Restarting systemd service $service due to $ANNOTATION_RESTART_SYSTEMD_SERVICES annotation"
        systemctl restart "$service" || true
      done

      /opt/bin/kubectl --kubeconfig="/var/lib/kubelet/kubeconfig-real" annotate node "$NODENAME" "${ANNOTATION_RESTART_SYSTEMD_SERVICES}-"

      if [[ ${restart_ccd} == "y" ]]; then
        echo "Restarting systemd service cloud-config-downloader.service due to $ANNOTATION_RESTART_SYSTEMD_SERVICES annotation"
        systemctl restart "cloud-config-downloader.service" || true
      fi
    fi
  fi

  # If the time difference from the last execution till now is smaller than the node-specific delay then we exit early
  # and don't apply the (potentially updated) cloud-config user data. This is to spread the restarts of the systemd
  # units and to prevent too many restarts happening on the nodes at roughly the same time.
  if [[ ! -f "$PATH_EXECUTION_DELAY_SECONDS" ]]; then
    if [[ "300" -gt "0" ]]; then
      echo $((30 + $RANDOM % 300)) > "$PATH_EXECUTION_DELAY_SECONDS"
    else
      echo "30" > "$PATH_EXECUTION_DELAY_SECONDS"
    fi
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
  if ` + reloadConfigCommand + `; then
    echo "Successfully applied new cloud config version"
    systemctl daemon-reload
`

	for _, name := range units {
		if name != docker.UnitName && name != varlibmount.UnitName && name != downloader.UnitName {
			footerPart += `    systemctl enable ` + name + ` && systemctl restart --no-block ` + name + `
`
		}
	}

	footerPart += `    echo "Successfully restarted all units referenced in the cloud config."
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
if [[ ! -z "$NODENAME" ]]; then
  /opt/bin/kubectl --kubeconfig="/var/lib/kubelet/kubeconfig-real" label node "$NODENAME" "worker.gardener.cloud/kubernetes-version=` + kubernetesVersion + `" --overwrite

  if [[ -f "$PATH_CHECKSUM" ]]; then
    /opt/bin/kubectl --kubeconfig="/var/lib/kubelet/kubeconfig-real" annotate node "$NODENAME" "checksum/cloud-config-data=$(cat "$PATH_CHECKSUM")" --overwrite
  fi
fi
date +%s > "$PATH_EXECUTION_LAST_DATE"
`

	return headerPart + kubeletDataVolumePart + imagesPreloadingPart + footerPart
}

func copyKubernetesBinariesFromHyperkubeImageForVersionsGreaterEqual119(hyperkubeImage *imagevector.Image) string {
	return `
  HYPERKUBE_CONTAINER_ID="$(/usr/bin/docker run -d -v "$PATH_HYPERKUBE_DOWNLOADS":"$PATH_HYPERKUBE_DOWNLOADS":rw "` + hyperkubeImage.String() + `")"
  /usr/bin/docker cp   "$HYPERKUBE_CONTAINER_ID":/kubelet "$PATH_HYPERKUBE_DOWNLOADS/kubelet-$hyperkubeImageSHA"
  /usr/bin/docker cp   "$HYPERKUBE_CONTAINER_ID":/kubectl "$PATH_HYPERKUBE_DOWNLOADS/kubectl-$hyperkubeImageSHA"
  /usr/bin/docker stop "$HYPERKUBE_CONTAINER_ID"
  /usr/bin/docker rm "$HYPERKUBE_CONTAINER_ID"`
}

const scriptCopyKubernetesBinary = `#!/bin/bash -eu

BINARY="$1"

PATH_HYPERKUBE_DOWNLOADS="/var/lib/cloud-config-downloader/downloads/hyperkube"
PATH_LAST_DOWNLOADED_HYPERKUBE_IMAGE="/var/lib/cloud-config-downloader/downloads/hyperkube/last_downloaded_hyperkube_image"
PATH_HYPERKUBE_IMAGE_USED_FOR_LAST_COPY=""

if [[ "$BINARY" == "kubelet" ]]; then
  PATH_HYPERKUBE_IMAGE_USED_FOR_LAST_COPY="/opt/bin/hyperkube_image_used_for_last_copy_of_kubelet"
elif [[ "$BINARY" == "kubectl" ]]; then
  PATH_HYPERKUBE_IMAGE_USED_FOR_LAST_COPY="/opt/bin/hyperkube_image_used_for_last_copy_of_kubectl"
else
  echo "$BINARY cannot be handled. Only 'kubelet' and 'kubectl' are valid arguments."
  exit 1
fi

LAST_DOWNLOADED_HYPERKUBE_IMAGE=""
if [[ -f "$PATH_LAST_DOWNLOADED_HYPERKUBE_IMAGE" ]]; then
  LAST_DOWNLOADED_HYPERKUBE_IMAGE="$(cat "$PATH_LAST_DOWNLOADED_HYPERKUBE_IMAGE")"
fi

HYPERKUBE_IMAGE_USED_FOR_LAST_COPY=""
if [[ -f "$PATH_HYPERKUBE_IMAGE_USED_FOR_LAST_COPY" ]]; then
  HYPERKUBE_IMAGE_USED_FOR_LAST_COPY="$(cat "$PATH_HYPERKUBE_IMAGE_USED_FOR_LAST_COPY")"
fi

echo "Checking whether to copy new $BINARY binary from hyperkube image to /opt/bin folder..."
if [[ "$HYPERKUBE_IMAGE_USED_FOR_LAST_COPY" != "$LAST_DOWNLOADED_HYPERKUBE_IMAGE" ]]; then
  echo "$BINARY binary in /opt/bin is outdated (image used for last copy: $HYPERKUBE_IMAGE_USED_FOR_LAST_COPY). Need to update it to $LAST_DOWNLOADED_HYPERKUBE_IMAGE".
  rm -f "/opt/bin/$BINARY" &&
    cp "$PATH_HYPERKUBE_DOWNLOADS/$BINARY" "/opt/bin" &&
    echo "$LAST_DOWNLOADED_HYPERKUBE_IMAGE" > "$PATH_HYPERKUBE_IMAGE_USED_FOR_LAST_COPY"
else
  echo "No need to copy $BINARY binary from a new hyperkube image because binary found in $PATH_HYPERKUBE_DOWNLOADS is up-to-date."
fi
`

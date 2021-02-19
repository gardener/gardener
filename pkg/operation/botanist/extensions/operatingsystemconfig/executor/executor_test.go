// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/operation/botanist/extensions/operatingsystemconfig/downloader"
	"github.com/gardener/gardener/pkg/operation/botanist/extensions/operatingsystemconfig/executor"
	"github.com/gardener/gardener/pkg/operation/botanist/extensions/operatingsystemconfig/original/components/docker"
	"github.com/gardener/gardener/pkg/operation/botanist/extensions/operatingsystemconfig/original/components/varlibmount"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/test/matchers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

var _ = Describe("Executor", func() {
	Describe("#Script", func() {
		var (
			bootstrapToken      string
			cloudConfigUserData []byte
			images              map[string]interface{}
			kubeletDataVolume   *gardencorev1beta1.DataVolume
			reloadConfigCommand string
			units               []string
		)

		BeforeEach(func() {
			bootstrapToken = "token"
			cloudConfigUserData = []byte("user-data")
			images = map[string]interface{}{"foo": "bar:v1.0"}
			kubeletDataVolume = nil
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

		It("should correctly render the executor script (w/o kubelet data volume)", func() {
			script, err := executor.Script(bootstrapToken, cloudConfigUserData, images, kubeletDataVolume, reloadConfigCommand, units)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(script)).To(matchers.DeepEqual(scriptFor(bootstrapToken, cloudConfigUserData, images, nil, reloadConfigCommand, units)))
		})

		It("should correctly render the executor script (w/ kubelet data volume)", func() {
			kubeletDataVolume = &gardencorev1beta1.DataVolume{VolumeSize: "64Gi"}

			script, err := executor.Script(bootstrapToken, cloudConfigUserData, images, kubeletDataVolume, reloadConfigCommand, units)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(script)).To(matchers.DeepEqual(scriptFor(bootstrapToken, cloudConfigUserData, images, pointer.StringPtr("68719476736"), reloadConfigCommand, units)))
		})

		It("should return an error because the data volume size cannot be parsed", func() {
			kubeletDataVolume = &gardencorev1beta1.DataVolume{VolumeSize: "not-parsable"}

			script, err := executor.Script(bootstrapToken, cloudConfigUserData, images, kubeletDataVolume, reloadConfigCommand, units)
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
	bootstrapToken string,
	cloudConfigUserData []byte,
	images map[string]interface{},
	kubeletDataVolumeSize *string,
	reloadConfigCommand string,
	units []string,
) string {
	headerPart := `#!/bin/bash -eu

PATH_CLOUDCONFIG_DOWNLOADER_SERVER="/var/lib/cloud-config-downloader/credentials/server"
PATH_CLOUDCONFIG_DOWNLOADER_CA_CERT="/var/lib/cloud-config-downloader/credentials/ca.crt"
PATH_CLOUDCONFIG="/var/lib/cloud-config-downloader/downloads/cloud_config"
PATH_CLOUDCONFIG_OLD="${PATH_CLOUDCONFIG}.old"
PATH_CHECKSUM="/var/lib/cloud-config-downloader/downloaded_checksum"
PATH_CCD_SCRIPT="/var/lib/cloud-config-downloader/download-cloud-config.sh"
PATH_CCD_SCRIPT_CHECKSUM="/var/lib/cloud-config-downloader/download-cloud-config.md5"
PATH_CCD_SCRIPT_CHECKSUM_OLD="${PATH_CCD_SCRIPT_CHECKSUM}.old"
mkdir -p "/var/lib/cloud-config-downloader/downloads" "/var/lib/kubelet"

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

`

	kubeletDataVolumePart := ""
	if kubeletDataVolumeSize != nil {
		kubeletDataVolumePart = `function format-data-device() {
  LABEL=KUBEDEV
  if ! blkid --label $LABEL >/dev/null; then
    DEVICES=$(lsblk -dbsnP -o NAME,PARTTYPE,FSTYPE,SIZE)
    MATCHING_DEVICES=$(echo "$DEVICES" | grep 'PARTTYPE="".*FSTYPE="".*SIZE="` + *kubeletDataVolumeSize + `"')
    echo "Matching kubelet data device by size : ` + *kubeletDataVolumeSize + `"
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

format-data-device`
	}

	imagesPreloadingPart := `

`
	for name, image := range images {
		imagesPreloadingPart += `docker-preload "` + name + `" "` + image.(string) + `"
`
	}

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
    token: ` + bootstrapToken + `
EOF

else
  rm -f "/var/lib/kubelet/kubeconfig-bootstrap"
fi

md5sum ${PATH_CCD_SCRIPT} > ${PATH_CCD_SCRIPT_CHECKSUM}

if ! diff "$PATH_CLOUDCONFIG" "$PATH_CLOUDCONFIG_OLD" >/dev/null || ! diff "$PATH_CCD_SCRIPT_CHECKSUM" "$PATH_CCD_SCRIPT_CHECKSUM_OLD" >/dev/null; then
  echo "Seen newer cloud config or cloud config downloder version"
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
  fi
fi

rm "$PATH_CLOUDCONFIG" "$PATH_CCD_SCRIPT_CHECKSUM"

ANNOTATION_RESTART_SYSTEMD_SERVICES="worker.gardener.cloud/restart-systemd-services"

# Try to find Node object for this machine
if [[ -f "/var/lib/kubelet/kubeconfig-real" ]]; then
  NODE="$(/opt/bin/kubectl --kubeconfig="/var/lib/kubelet/kubeconfig-real" get node -l "kubernetes.io/hostname=$(hostname)" -o go-template="{{ if .items }}{{ (index .items 0).metadata.name }}{{ if (index (index .items 0).metadata.annotations \"$ANNOTATION_RESTART_SYSTEMD_SERVICES\") }} {{ index (index .items 0).metadata.annotations \"$ANNOTATION_RESTART_SYSTEMD_SERVICES\" }}{{ end }}{{ end }}")"

  if [[ ! -z "$NODE" ]]; then
    NODENAME="$(echo "$NODE" | awk '{print $1}')"
    SYSTEMD_SERVICES_TO_RESTART="$(echo "$NODE" | awk '{print $2}')"
  fi

  # Update checksum/cloud-config-data annotation on Node object if possible
  if [[ ! -z "$NODENAME" ]] && [[ -f "$PATH_CHECKSUM" ]]; then
    /opt/bin/kubectl --kubeconfig="/var/lib/kubelet/kubeconfig-real" annotate node "$NODENAME" "checksum/cloud-config-data=$(cat "$PATH_CHECKSUM")" --overwrite
  fi

  # Restart systemd services if requested
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
    echo "Restarting systemd service cloud-config-downloader due to $ANNOTATION_RESTART_SYSTEMD_SERVICES annotation"
    systemctl restart "cloud-config-downloader" || true
  fi
fi
`

	return headerPart + kubeletDataVolumePart + imagesPreloadingPart + footerPart
}

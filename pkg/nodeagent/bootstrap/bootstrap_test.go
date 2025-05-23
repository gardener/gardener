// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bootstrap_test

import (
	"context"
	"io/fs"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/afero"
	"k8s.io/utils/ptr"

	nodeagentconfigv1alpha1 "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
	. "github.com/gardener/gardener/pkg/nodeagent/bootstrap"
	fakedbus "github.com/gardener/gardener/pkg/nodeagent/dbus/fake"
	"github.com/gardener/gardener/pkg/utils/test"
)

var _ = Describe("Bootstrap", func() {
	Describe("#Bootstrap", func() {
		var (
			ctx = context.Background()
			log = logr.Discard()

			fakeFS   afero.Afero
			fakeDBus *fakedbus.DBus

			bootstrapConfig *nodeagentconfigv1alpha1.BootstrapConfiguration

			expectedGNAUnitContent = `[Unit]
Description=Gardener Node Agent
After=network-online.target

[Service]
LimitMEMLOCK=infinity
ExecStart=/opt/bin/gardener-node-agent --config=/var/lib/gardener-node-agent/config.yaml
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target`
		)

		BeforeEach(func() {
			fakeFS = afero.Afero{Fs: afero.NewMemMapFs()}
			fakeDBus = fakedbus.New()

			bootstrapConfig = &nodeagentconfigv1alpha1.BootstrapConfiguration{}
		})

		assertions := func() {
			By("Ensure that gardener-node-agent unit file was written")
			assertFileOnDisk(fakeFS, "/etc/systemd/system/gardener-node-agent.service", expectedGNAUnitContent, 0644)

			By("Ensure that gardener-node-agent was started and gardener-node-init was disabled")
			ExpectWithOffset(1, fakeDBus.Actions).To(ConsistOf(
				fakedbus.SystemdAction{Action: fakedbus.ActionDaemonReload},
				fakedbus.SystemdAction{Action: fakedbus.ActionEnable, UnitNames: []string{"gardener-node-agent.service"}},
				fakedbus.SystemdAction{Action: fakedbus.ActionStart, UnitNames: []string{"gardener-node-agent.service"}},
				fakedbus.SystemdAction{Action: fakedbus.ActionDisable, UnitNames: []string{"gardener-node-init.service"}},
			))
		}

		When("kubelet data volume size is not set", func() {
			It("should start gardener-node-agent and stop gardener-node-init", func() {
				Expect(Bootstrap(ctx, log, fakeFS, fakeDBus, bootstrapConfig)).To(Succeed())
				assertions()
			})
		})

		When("kubelet data volume size is set", func() {
			BeforeEach(func() {
				bootstrapConfig.KubeletDataVolumeSize = ptr.To[int64](1234)

				DeferCleanup(test.WithVar(&ExecScript, func(scriptPath string) ([]byte, error) {
					script, err := fakeFS.ReadFile(scriptPath)
					ExpectWithOffset(1, string(script)).To(Equal(`#!/usr/bin/env bash

LABEL=KUBEDEV
if ! blkid --label $LABEL >/dev/null; then
  DISK_DEVICES=$(lsblk -dbsnP -o NAME,PARTTYPE,FSTYPE,SIZE,PATH,TYPE | grep 'TYPE="disk"')
  while IFS= read -r line; do
    MATCHING_DEVICE_CANDIDATE=$(echo "$line" | grep 'PARTTYPE="".*FSTYPE="".*SIZE="1234"')
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
    exit 1
  fi

  echo "Matching kubelet data device by size : 1234"
  echo "detected kubelet data device $TARGET_DEVICE_NAME"
  mkfs.ext4 -L $LABEL -O quota -E lazy_itable_init=0,lazy_journal_init=0,quotatype=usrquota:grpquota:prjquota  /dev/$TARGET_DEVICE_NAME
  echo "formatted and labeled data device $TARGET_DEVICE_NAME"
  mkdir /tmp/varlibkubeletcp
  mount LABEL=$LABEL /tmp/varlibkubeletcp
  echo "mounted temp copy dir on data device $TARGET_DEVICE_NAME"
  cp -a /var/lib/kubelet/* /tmp/varlibkubeletcp/
  umount /tmp/varlibkubeletcp
  echo "copied /var/lib/kubelet to data device $TARGET_DEVICE_NAME"
  mount LABEL=$LABEL /var/lib/kubelet -o defaults,prjquota,errors=remount-ro
  echo "mounted /var/lib/kubelet on data device $TARGET_DEVICE_NAME"
fi
`))
					return script, err
				}))
			})

			It("should start gardener-node-agent and stop gardener-node-init", func() {
				Expect(Bootstrap(ctx, log, fakeFS, fakeDBus, bootstrapConfig)).To(Succeed())
				assertions()
			})
		})
	})
})

func assertFileOnDisk(fakeFS afero.Afero, path, expectedContent string, fileMode uint32) {
	description := "file path " + path

	content, err := fakeFS.ReadFile(path)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), description)
	ExpectWithOffset(1, string(content)).To(Equal(expectedContent), description)

	fileInfo, err := fakeFS.Stat(path)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), description)
	ExpectWithOffset(1, fileInfo.Mode()).To(Equal(fs.FileMode(fileMode)), description)
}

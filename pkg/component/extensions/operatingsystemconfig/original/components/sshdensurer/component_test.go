// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package sshdensurer_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components"
	. "github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/sshdensurer"
	"github.com/gardener/gardener/pkg/utils"
)

var _ = Describe("Component", func() {
	Describe("#Config", func() {
		var (
			component components.Component
			ctx       components.Context
		)

		BeforeEach(func() {
			component = New()
		})

		It("should return the expected units and files when SSHAccessEnabled is set to true", func() {
			ctx = components.Context{SSHAccessEnabled: true}
			units, files, err := component.Config(ctx)

			Expect(err).NotTo(HaveOccurred())

			sshdEnsurerUnit := extensionsv1alpha1.Unit{

				Name:    "sshd-ensurer.service",
				Command: ptr.To(extensionsv1alpha1.CommandStart),
				Content: ptr.To(`[Unit]
Description=Ensure SSHD service is enabled or disabled
DefaultDependencies=no
[Service]
Type=simple
Restart=always
RestartSec=15
ExecStart=/var/lib/sshd-ensurer/run.sh
[Install]
WantedBy=multi-user.target`),
				FilePaths: []string{"/var/lib/sshd-ensurer/run.sh"},
			}

			sshdEnsurerFile := extensionsv1alpha1.File{
				Path:        "/var/lib/sshd-ensurer/run.sh",
				Permissions: ptr.To[uint32](0755),
				Content: extensionsv1alpha1.FileContent{
					Inline: &extensionsv1alpha1.FileContentInline{
						Encoding: "b64",
						Data:     utils.EncodeBase64([]byte(enableScript)),
					},
				},
			}

			Expect(units).To(ConsistOf(sshdEnsurerUnit))
			Expect(files).To(ConsistOf(sshdEnsurerFile))
		})

		It("should return the expected units and files when SSHAccessEnabled is set to false", func() {
			ctx = components.Context{SSHAccessEnabled: false}
			units, files, err := component.Config(ctx)

			Expect(err).NotTo(HaveOccurred())

			sshdEnsurerUnit := extensionsv1alpha1.Unit{
				Name:    "sshd-ensurer.service",
				Command: ptr.To(extensionsv1alpha1.CommandStart),
				Content: ptr.To(`[Unit]
Description=Ensure SSHD service is enabled or disabled
DefaultDependencies=no
[Service]
Type=simple
Restart=always
RestartSec=15
ExecStart=/var/lib/sshd-ensurer/run.sh
[Install]
WantedBy=multi-user.target`),
				FilePaths: []string{"/var/lib/sshd-ensurer/run.sh"},
			}

			sshdEnsurerFile := extensionsv1alpha1.File{
				Path:        "/var/lib/sshd-ensurer/run.sh",
				Permissions: ptr.To[uint32](0755),
				Content: extensionsv1alpha1.FileContent{
					Inline: &extensionsv1alpha1.FileContentInline{
						Encoding: "b64",
						Data:     utils.EncodeBase64([]byte(disableScript)),
					},
				},
			}

			Expect(units).To(ConsistOf(sshdEnsurerUnit))
			Expect(files).To(ConsistOf(sshdEnsurerFile))
		})
	})
})

const (
	enableScript = `#!/bin/bash -eu
set -e

# Unmask and enable sshd service if not enabled
if ! systemctl is-enabled --quiet sshd.service ; then
    systemctl unmask sshd.service
    # When sshd.service is disabled on gardenlinux the service is deleted
    # On gardenlinux sshd.service is enabled by enabling ssh.service
    if ! systemctl enable sshd.service ; then
        systemctl enable ssh.service
    fi
fi

# Start sshd service if not active
if ! systemctl is-active --quiet sshd.service ; then
    systemctl start sshd.service
fi
`
	disableScript = `#!/bin/bash -eu
set -e

# Disable and mask sshd service if not masked
if [ "$(systemctl is-enabled sshd.service)" != "masked" ] ; then
    systemctl disable sshd.service || true
    # Using the --now flag stops the selected service
    systemctl mask --now sshd.service
fi

# Stopping the sshd service with the --now flag does
# not terminate already established connections
# Kill all currently established orphaned ssh connections on the host
pkill -P 1 -x sshd || true
`
)

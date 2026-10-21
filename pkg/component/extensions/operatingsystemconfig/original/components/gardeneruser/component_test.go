// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package gardeneruser_test

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components"
	. "github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/gardeneruser"
	"github.com/gardener/gardener/pkg/utils"
)

var _ = Describe("Component", func() {
	Describe("#Config", func() {
		var (
			component components.Component
			ctx       components.Context

			sshPublicKeys = []string{
				"some-non-base64-encoded-public-key",
				"another-not-encoded-key",
				"the-last-key-i-promise",
			}
		)

		BeforeEach(func() {
			component = New()
			ctx = components.Context{SSHPublicKeys: sshPublicKeys}
		})

		It("should return the expected units and files", func() {
			units, files, err := component.Config(ctx)

			Expect(err).NotTo(HaveOccurred())
			Expect(units).To(ConsistOf(
				extensionsv1alpha1.Unit{
					Name:   "gardener-user.service",
					Enable: new(true),
					Content: new(`[Unit]
Description=Configure gardener user
After=sshd.service
[Service]
Restart=on-failure
EnvironmentFile=/etc/environment
ExecStart=/var/lib/gardener-user/run.sh
[Install]
WantedBy=multi-user.target
`),
				},
				extensionsv1alpha1.Unit{
					Name:   "gardener-user.path",
					Enable: new(true),
					Content: new(`[Path]
PathChanged=/var/lib/gardener-user-authorized-keys
[Install]
WantedBy=multi-user.target
`),
				},
			))
			Expect(files).To(ConsistOf(
				extensionsv1alpha1.File{
					Path:        "/var/lib/gardener-user-authorized-keys",
					Permissions: new(int32(0644)),
					Content: extensionsv1alpha1.FileContent{
						Inline: &extensionsv1alpha1.FileContentInline{
							Encoding: "b64",
							Data:     utils.EncodeBase64([]byte(strings.Join(sshPublicKeys, "\n"))),
						},
					},
				},
				extensionsv1alpha1.File{
					Path:        "/var/lib/gardener-user/run.sh",
					Permissions: new(int32(0755)),
					Content: extensionsv1alpha1.FileContent{
						Inline: &extensionsv1alpha1.FileContentInline{
							Encoding: "b64",
							Data:     utils.EncodeBase64([]byte(script)),
						},
					},
				},
			))
		})
	})
})

const script = `#!/bin/bash -eu

DIR_SSH="/home/gardener/.ssh"
PATH_AUTHORIZED_KEYS="$DIR_SSH/authorized_keys"
PATH_SUDOERS="/etc/sudoers.d/99-gardener-user"
USERNAME="gardener"
HOME_DIR="/home/$USERNAME"

# create user if missing
if ! id "$USERNAME" >/dev/null 2>&1; then
  if [ -d "$HOME_DIR" ]; then
    # The home directory survived an /etc wipe (e.g. GardenLinux wipe-based in-place OS upgrade) while the
    # passwd/group entries did not. Recreate the user/group with the UID/GID that still own the home directory
    # so its files (and anything else owned by the gardener user) keep their owner instead of being orphaned.
    GARDENER_UID="$(stat -c %u "$HOME_DIR")"
    GARDENER_GID="$(stat -c %g "$HOME_DIR")"
    getent group "$GARDENER_GID" >/dev/null 2>&1 || groupadd -g "$GARDENER_GID" "$USERNAME"
    useradd "$USERNAME" -m -u "$GARDENER_UID" -g "$GARDENER_GID"
  else
    useradd "$USERNAME" -mU
  fi
fi

# copy authorized_keys file
mkdir -p $DIR_SSH
cp -f "/var/lib/gardener-user-authorized-keys" $PATH_AUTHORIZED_KEYS
chown $USERNAME:$USERNAME $PATH_AUTHORIZED_KEYS

# remove unused legacy file
if [ -f "/var/lib/gardener-user-ssh.key" ]; then
  rm -f "/var/lib/gardener-user-ssh.key"
fi

# allow sudo for gardener user
if [ ! -f "$PATH_SUDOERS" ]; then
  echo "$USERNAME ALL=(ALL) NOPASSWD:ALL" > $PATH_SUDOERS
fi
`

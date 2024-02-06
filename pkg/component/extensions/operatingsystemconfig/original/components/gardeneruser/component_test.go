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

package gardeneruser_test

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"

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
					Enable: ptr.To(true),
					Content: ptr.To(`[Unit]
Description=Configure gardener user
After=sshd.service
[Service]
Restart=on-failure
EnvironmentFile=/etc/environment
ExecStart=/var/lib/gardener-user/run.sh
`),
				},
			))
			Expect(files).To(ConsistOf(
				extensionsv1alpha1.File{
					Path:        "/var/lib/gardener-user-authorized-keys",
					Permissions: ptr.To(int32(0644)),
					Content: extensionsv1alpha1.FileContent{
						Inline: &extensionsv1alpha1.FileContentInline{
							Encoding: "b64",
							Data:     utils.EncodeBase64([]byte(strings.Join(sshPublicKeys, "\n"))),
						},
					},
				},
				extensionsv1alpha1.File{
					Path:        "/var/lib/gardener-user/run.sh",
					Permissions: ptr.To(int32(0755)),
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

# create user if missing
id $USERNAME || useradd $USERNAME -mU

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

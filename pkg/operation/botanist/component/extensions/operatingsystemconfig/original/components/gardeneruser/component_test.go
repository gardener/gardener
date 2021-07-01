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

package gardeneruser_test

import (
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/operatingsystemconfig/original/components"
	. "github.com/gardener/gardener/pkg/operation/botanist/component/extensions/operatingsystemconfig/original/components/gardeneruser"
	"github.com/gardener/gardener/pkg/utils"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"
)

var _ = Describe("Component", func() {
	Describe("#Config", func() {
		var (
			component components.Component
			ctx       components.Context

			sshPublicKey       = "some-non-base64-encoded-public-key"
			sshPublicKeyBase64 = utils.EncodeBase64([]byte(sshPublicKey))
		)

		BeforeEach(func() {
			component = New()
			ctx = components.Context{SSHPublicKey: sshPublicKey}
		})

		It("should return the expected units and files", func() {
			units, files, err := component.Config(ctx)

			Expect(err).NotTo(HaveOccurred())
			Expect(units).To(ConsistOf(
				extensionsv1alpha1.Unit{
					Name:   "gardener-user.service",
					Enable: pointer.Bool(true),
					Content: pointer.String(`[Unit]
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
					Path:        "/var/lib/gardener-user-ssh.key",
					Permissions: pointer.Int32(0644),
					Content: extensionsv1alpha1.FileContent{
						Inline: &extensionsv1alpha1.FileContentInline{
							Encoding: "b64",
							Data:     sshPublicKeyBase64,
						},
					},
				},
				extensionsv1alpha1.File{
					Path:        "/var/lib/gardener-user/run.sh",
					Permissions: pointer.Int32(0755),
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

id $USERNAME || useradd $USERNAME -mU
if [ ! -f "$PATH_AUTHORIZED_KEYS" ]; then
  mkdir -p $DIR_SSH
  cp -f /var/lib/gardener-user-ssh.key $PATH_AUTHORIZED_KEYS
  chown $USERNAME:$USERNAME $PATH_AUTHORIZED_KEYS
fi
if [ ! -f "$PATH_SUDOERS" ]; then
  echo "$USERNAME ALL=(ALL) NOPASSWD:ALL" > $PATH_SUDOERS
fi
`

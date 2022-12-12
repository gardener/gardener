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

package sshddisabler_test

import (
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/operatingsystemconfig/original/components"
	. "github.com/gardener/gardener/pkg/operation/botanist/component/extensions/operatingsystemconfig/original/components/sshddisabler"
	"github.com/gardener/gardener/pkg/utils"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/pointer"
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

		It("should return the expected units and files when EnsureSSHAccessDisabled is set to true", func() {
			ctx = components.Context{EnsureSSHAccessDisabled: true}
			units, files, err := component.Config(ctx)

			Expect(err).NotTo(HaveOccurred())
			Expect(units).To(ConsistOf(
				[]extensionsv1alpha1.Unit{
					{
						Name:    "sshddisabler.service",
						Command: pointer.String("start"),
						Content: pointer.String(`[Unit]
Description=Disable ssh access and kill any currently established ssh connections
DefaultDependencies=no
[Service]
Type=simple
Restart=always
RestartSec=15
ExecStart=/var/lib/sshd-disabler/run.sh
[Install]
WantedBy=multi-user.target`),
					},
				},
			))
			Expect(files).To(ConsistOf(
				[]extensionsv1alpha1.File{
					{
						Path:        "/var/lib/sshd-disabler/run.sh",
						Permissions: pointer.Int32(0755),
						Content: extensionsv1alpha1.FileContent{
							Inline: &extensionsv1alpha1.FileContentInline{
								Encoding: "b64",
								Data:     utils.EncodeBase64([]byte(script)),
							},
						},
					},
				},
			))
		})

		It("should return the expected units and files when EnsureSSHAccessDisabled is set to false", func() {
			ctx = components.Context{EnsureSSHAccessDisabled: false}
			units, files, err := component.Config(ctx)

			Expect(err).NotTo(HaveOccurred())
			Expect(units).To(ConsistOf(
				[]extensionsv1alpha1.Unit{
					{
						Name:    "sshddisabler.service",
						Command: pointer.String("start"),
						Content: pointer.String(`[Unit]
Description=Disable ssh access and kill any currently established ssh connections
DefaultDependencies=no
[Service]
Type=simple
ExecStart= echo service sshddisabler is disabled in workers settings.
[Install]
WantedBy=multi-user.target`),
					},
				},
			))
			Expect(files).To(BeNil())
		})
	})
})

const script = `#!/bin/bash -eu
set -e

# Stop sshd service if active
if systemctl is-active --quiet sshd.service ; then
    systemctl stop sshd.service
fi

# Disable sshd service if enabled
if systemctl is-enabled --quiet sshd.service ; then
    systemctl disable sshd.service
fi

# Disabling the sshd service does not terminate already established connections
# Kill all currently established ssh connections
pids=$(pidof sshd || true)
if [ -n "$pids" ]; then
    kill $pids
fi
`

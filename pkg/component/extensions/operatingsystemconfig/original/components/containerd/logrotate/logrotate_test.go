// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package logrotate_test

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/containerd"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/containerd/logrotate"
)

var _ = Describe("Logrotate", func() {
	Describe("#Config", func() {
		Context("containerd container runtime", func() {
			It("should return the expected units and files in", func() {
				var (
					prefix       = containerd.ContainerRuntime
					pathConfig   = "/bar/baz"
					pathLogFiles = strings.Builder{}
				)

				pathLogFiles.WriteString("/var/log/")
				pathLogFiles.WriteString(containerd.ContainerRuntime)
				pathLogFiles.WriteString("/*.log")

				units, files := logrotate.Config(pathConfig, pathLogFiles.String(), prefix)

				serviceUnit := extensionsv1alpha1.Unit{
					Name:   prefix + "-logrotate.service",
					Enable: ptr.To(true),
					Content: ptr.To(`[Unit]
Description=Rotate and Compress System Logs
[Service]
ExecStart=/usr/sbin/logrotate -s /var/lib/` + prefix + `-logrotate.status ` + pathConfig + `
[Install]
WantedBy=multi-user.target`),
					FilePaths: []string{pathConfig},
				}

				timerUnit := extensionsv1alpha1.Unit{
					Name:    prefix + "-logrotate.timer",
					Command: ptr.To(extensionsv1alpha1.CommandStart),
					Enable:  ptr.To(true),
					Content: ptr.To(`[Unit]
Description=Log Rotation at each 10 minutes
[Timer]
OnCalendar=*:0/10
AccuracySec=1min
Persistent=true
[Install]
WantedBy=multi-user.target`),
				}

				serviceConfigFile := extensionsv1alpha1.File{
					Path:        pathConfig,
					Permissions: ptr.To[uint32](0644),
					Content: extensionsv1alpha1.FileContent{
						Inline: &extensionsv1alpha1.FileContentInline{
							Data: pathLogFiles.String() + ` {
    rotate 14
    copytruncate
    missingok
    notifempty
    compress
    daily
    dateext
    dateformat -%Y%m%d-%s
    create 0644 root root
}
`,
						},
					},
				}

				Expect(units).To(ConsistOf(serviceUnit, timerUnit))
				Expect(files).To(ConsistOf(serviceConfigFile))
			})
		})
	})
})

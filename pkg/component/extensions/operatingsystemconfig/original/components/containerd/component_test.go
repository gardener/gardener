// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package containerd_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components"
	. "github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/containerd"
)

var _ = Describe("Component", func() {
	Describe("#Config", func() {
		var component components.Component

		BeforeEach(func() {
			component = New()
		})

		It("should return the expected units and files", func() {
			units, files, err := component.Config(components.Context{})

			Expect(err).NotTo(HaveOccurred())

			containerdUnit := extensionsv1alpha1.Unit{
				Name:      "containerd.service",
				FilePaths: []string{"/var/lib/ca-certificates-local/ROOTcerts.crt"},
			}

			logrotateUnit := extensionsv1alpha1.Unit{
				Name:   "containerd-logrotate.service",
				Enable: ptr.To(true),
				Content: ptr.To(`[Unit]
Description=Rotate and Compress System Logs
[Service]
ExecStart=/usr/sbin/logrotate -s /var/lib/containerd-logrotate.status /etc/systemd/containerd.conf
[Install]
WantedBy=multi-user.target`),
				FilePaths: []string{"/etc/systemd/containerd.conf"},
			}

			logrotateTimerUnit := extensionsv1alpha1.Unit{
				Name:    "containerd-logrotate.timer",
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

			logrotateConfigFile := extensionsv1alpha1.File{
				Path:        "/etc/systemd/containerd.conf",
				Permissions: ptr.To[uint32](0644),
				Content: extensionsv1alpha1.FileContent{
					Inline: &extensionsv1alpha1.FileContentInline{
						Data: logRotateData,
					},
				},
			}

			Expect(units).To(ConsistOf(containerdUnit, logrotateUnit, logrotateTimerUnit))
			Expect(files).To(ConsistOf(logrotateConfigFile))
		})
	})
})

const logRotateData = `/var/log/pods/*/*/*.log {
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
`

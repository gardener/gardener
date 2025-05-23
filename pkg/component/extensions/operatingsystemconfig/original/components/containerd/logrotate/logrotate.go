// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package logrotate

import (
	"k8s.io/utils/ptr"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

// Config returns the content for logrotate units and files.
// Whenever logrotate is ran, this config will:
//   - keep only 14 old (rotated) logs, and will discard older logs.
//
// Prefix carries the target container runtime (such as  containerd, docker).
// When containerd is used the log rotation based on size is performed by kubelet.
func Config(pathConfig, pathLogFiles, prefix string) ([]extensionsv1alpha1.Unit, []extensionsv1alpha1.File) {
	serviceFile := extensionsv1alpha1.File{
		Path:        pathConfig,
		Permissions: ptr.To[uint32](0644),
		Content: extensionsv1alpha1.FileContent{
			Inline: &extensionsv1alpha1.FileContentInline{
				Data: pathLogFiles + ` {
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

	serviceUnit := extensionsv1alpha1.Unit{
		Name:   prefix + "-logrotate.service",
		Enable: ptr.To(true),
		Content: ptr.To(`[Unit]
Description=Rotate and Compress System Logs
[Service]
ExecStart=/usr/sbin/logrotate -s /var/lib/` + prefix + `-logrotate.status ` + pathConfig + `
[Install]
WantedBy=multi-user.target`),
		FilePaths: []string{serviceFile.Path},
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

	return []extensionsv1alpha1.Unit{serviceUnit, timerUnit}, []extensionsv1alpha1.File{serviceFile}
}

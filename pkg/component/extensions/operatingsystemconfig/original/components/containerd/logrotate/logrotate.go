// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package logrotate

import (
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

// Config returns the content for logrotate units and files.
// Whenever logrotate is ran, this config will:
//   - keep only 14 old (rotated) logs, and will discard older logs.
//
// Prefix carries the target container runtime (such as  containerd, docker).
// When containerd is used the log rotation based on size is performed by kubelet.
//
// A security requirement mandates that no log entry older than 14 days is present on the host.
// kubelet and containerd only support size-based log rotation, not time-based retention, so
// logrotate is used to enforce daily rotation and a 14-day retention window on top of the
// container runtime's own size-based rotation.
// See: https://github.com/gardener/gardener/issues/653 for the historical context.
//
// The timer is configured with jitter to avoid all nodes rotating logs at the same time
// which can cause spikes on the storage backend.
// See: https://github.com/gardener/gardener/issues/15149 for more context.
func Config(pathConfig, pathLogFiles, prefix string) ([]extensionsv1alpha1.Unit, []extensionsv1alpha1.File) {
	serviceFile := extensionsv1alpha1.File{
		Path:        pathConfig,
		Permissions: new(uint32(0644)),
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
		Enable: new(true),
		Content: new(`[Unit]
Description=Rotate and Compress System Logs
[Service]
ExecStart=/usr/sbin/logrotate -s /var/lib/` + prefix + `-logrotate.status ` + pathConfig + `
[Install]
WantedBy=multi-user.target`),
		FilePaths: []string{serviceFile.Path},
	}

	timerUnit := extensionsv1alpha1.Unit{
		Name:    prefix + "-logrotate.timer",
		Command: new(extensionsv1alpha1.CommandStart),
		Enable:  new(true),
		Content: new(`[Unit]
Description=Log Rotation once a day
[Timer]
OnCalendar=daily
AccuracySec=1min
RandomizedDelaySec=4h
Persistent=true
[Install]
WantedBy=multi-user.target`),
	}

	return []extensionsv1alpha1.Unit{serviceUnit, timerUnit}, []extensionsv1alpha1.File{serviceFile}
}

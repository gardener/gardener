// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package logrotate

import (
	"strings"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

// Config returns the content for logrotate units and files.
// Prefix carries the target container runtime (such as containerd, docker).
//
// A security requirement mandates that no log entry older than 14 days is present on the host.
// kubelet only supports size-based log rotation, not time-based retention, so two mechanisms
// are combined to enforce the 14-day retention window:
//   - logrotate handles daily rotation and pruning of active log files (*.log).
//   - a find command deletes kubelet's size-rotated files (*.log.*) older than 14 days,
//     since logrotate can only prune files it rotated itself.
//
// For the historical context, see https://github.com/gardener/gardener/issues/653.
//
// The timer is configured with jitter to avoid all nodes rotating logs at the same time
// which can cause spikes on the storage backend.
// For more context, see https://github.com/gardener/gardener/issues/15149.
func Config(pathConfig, pathLogFiles, prefix string) ([]extensionsv1alpha1.Unit, []extensionsv1alpha1.File) {
	// Derive the pod log directory by stripping the glob suffix from pathLogFiles.
	// e.g. "/var/log/pods/*/*/*.log" -> "/var/log/pods"
	podLogDir := strings.TrimRight(strings.SplitN(pathLogFiles, "*", 2)[0], "/")

	serviceFile := extensionsv1alpha1.File{
		Path:        pathConfig,
		Permissions: new(uint32(0644)),
		Content: extensionsv1alpha1.FileContent{
			Inline: &extensionsv1alpha1.FileContentInline{
				Data: pathLogFiles + ` {
    rotate 14
    copytruncate
    missingok
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
StartLimitBurst=5
StartLimitIntervalSec=30
[Service]
ExecStart=/usr/sbin/logrotate -s /var/lib/` + prefix + `-logrotate.status ` + pathConfig + `
ExecStartPost=/bin/sh -c 'find ` + podLogDir + ` -name "*.log.*" -mtime +14 -delete 2>&1 || [ ! -d ` + podLogDir + ` ]'
Restart=on-failure
RestartSec=5
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

// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package logrotate

import (
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"k8s.io/utils/pointer"
)

// Config returns the content for logrotate units and files.
// Whenever logrotate is ran, this config will:
//   - keep only 14 old (rotated) logs, and will discard older logs.
//
// Prefix carries the target container runtime (such as  containerd, docker).
// When containerd is used the log rotation based on size is performed by kubelet.
func Config(pathConfig, pathLogFiles, prefix string) ([]extensionsv1alpha1.Unit, []extensionsv1alpha1.File) {
	var (
		extUnit []extensionsv1alpha1.Unit
		extFile []extensionsv1alpha1.File
	)

	extUnit = []extensionsv1alpha1.Unit{
		{
			Name:   prefix + "-logrotate.service",
			Enable: pointer.Bool(true),
			Content: pointer.String(`[Unit]
Description=Rotate and Compress System Logs
[Service]
ExecStart=/usr/sbin/logrotate -s /var/lib/` + prefix + `-logrotate.status ` + pathConfig + `
[Install]
WantedBy=multi-user.target`),
		},
		{
			Name:    prefix + "-logrotate.timer",
			Command: pointer.String("start"),
			Enable:  pointer.Bool(true),
			Content: pointer.String(`[Unit]
Description=Log Rotation at each 10 minutes
[Timer]
OnCalendar=*:0/10
AccuracySec=1min
Persistent=true
[Install]
WantedBy=multi-user.target`),
		},
	}

	extFile = []extensionsv1alpha1.File{
		{
			Path:        pathConfig,
			Permissions: pointer.Int32(0644),
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
		},
	}

	return extUnit, extFile
}

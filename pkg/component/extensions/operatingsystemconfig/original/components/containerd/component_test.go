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
	"github.com/gardener/gardener/pkg/utils"
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

			monitorUnit := extensionsv1alpha1.Unit{
				Name:    "containerd-monitor.service",
				Command: ptr.To(extensionsv1alpha1.CommandStart),
				Enable:  ptr.To(true),
				Content: ptr.To(`[Unit]
Description=Containerd-monitor daemon
After=containerd.service
[Install]
WantedBy=multi-user.target
[Service]
Restart=always
EnvironmentFile=/etc/environment
ExecStart=/opt/bin/health-monitor-containerd`),
				FilePaths: []string{"/opt/bin/health-monitor-containerd"},
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

			monitorFile := extensionsv1alpha1.File{
				Path:        "/opt/bin/health-monitor-containerd",
				Permissions: ptr.To[uint32](0755),
				Content: extensionsv1alpha1.FileContent{
					Inline: &extensionsv1alpha1.FileContentInline{
						Encoding: "b64",
						Data:     utils.EncodeBase64([]byte(healthMonitorScript)),
					},
				},
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

			Expect(units).To(ConsistOf(containerdUnit, monitorUnit, logrotateUnit, logrotateTimerUnit))
			Expect(files).To(ConsistOf(monitorFile, logrotateConfigFile))
		})
	})
})

const (
	healthMonitorScript = `#!/bin/bash
set -o nounset
set -o pipefail

function containerd_monitoring {
  echo "containerd monitor has started !"
  while [ 1 ]; do
    start_timestamp=$(date +%s)
    until ctr c list > /dev/null; do
      CONTAINERD_PID=$(systemctl show --property MainPID containerd --value)

      if [ $CONTAINERD_PID -eq 0 ]; then
          echo "Connection to containerd socket failed (process not started), retrying in $SLEEP_SECONDS seconds..."
          break
      fi

      now=$(date +%s)
      time_elapsed="$(($now-$start_timestamp))"

      if [ $time_elapsed -gt 60 ]; then
        echo "containerd daemon unreachable for more than 60s. Sending SIGTERM to PID $CONTAINERD_PID"
        kill -n 15 $CONTAINERD_PID
        sleep 20
        break 2
      fi
      echo "Connection to containerd socket failed, retrying in $SLEEP_SECONDS seconds..."
      sleep $SLEEP_SECONDS
    done
    sleep $SLEEP_SECONDS
  done
}

SLEEP_SECONDS=10
echo "Start health monitoring for containerd"
containerd_monitoring
`

	logRotateData = `/var/log/pods/*/*/*.log {
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
)

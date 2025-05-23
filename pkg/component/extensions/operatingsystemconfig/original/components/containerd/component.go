// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package containerd

import (
	"bytes"
	_ "embed"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"k8s.io/utils/ptr"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/containerd/logrotate"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/rootcertificates"
	"github.com/gardener/gardener/pkg/utils"
)

var (
	tplNameHealthMonitor = "health-monitor"
	//go:embed templates/scripts/health-monitor.tpl.sh
	tplContentHealthMonitor string
	tplHealthMonitor        *template.Template
)

func init() {
	var err error
	tplHealthMonitor, err = template.
		New(tplNameHealthMonitor).
		Funcs(sprig.TxtFuncMap()).
		Parse(tplContentHealthMonitor)
	if err != nil {
		panic(err)
	}
}

const (
	// UnitName is the name of the containerd service unit.
	UnitName = v1beta1constants.OperatingSystemConfigUnitNameContainerDService
	// UnitNameMonitor is the name of the containerd monitor service unit.
	UnitNameMonitor = "containerd-monitor.service"
	// PathSocketEndpoint is the path to the containerd unix domain socket.
	PathSocketEndpoint = "unix:///run/containerd/containerd.sock"
	// CgroupPath is the cgroup path the containerd container runtime is isolated in.
	CgroupPath = "/system.slice/containerd.service"
	// ContainerRuntime designates the runtime type
	ContainerRuntime = "containerd"
)

type containerd struct{}

// New returns a new containerd component.
func New() *containerd {
	return &containerd{}
}

func (containerd) Name() string {
	return ContainerRuntime
}

func (containerd) Config(_ components.Context) ([]extensionsv1alpha1.Unit, []extensionsv1alpha1.File, error) {
	const (
		pathHealthMonitor   = v1beta1constants.OperatingSystemConfigFilePathBinaries + "/health-monitor-containerd"
		pathLogRotateConfig = "/etc/systemd/containerd.conf"
	)

	var healthMonitorScript bytes.Buffer
	if err := tplHealthMonitor.Execute(&healthMonitorScript, nil); err != nil {
		return nil, nil, err
	}

	logRotateUnits, logRotateFiles := logrotate.Config(pathLogRotateConfig, "/var/log/pods/*/*/*.log", ContainerRuntime)

	monitorFile := extensionsv1alpha1.File{
		Path:        pathHealthMonitor,
		Permissions: ptr.To[uint32](0755),
		Content: extensionsv1alpha1.FileContent{
			Inline: &extensionsv1alpha1.FileContentInline{
				Encoding: "b64",
				Data:     utils.EncodeBase64(healthMonitorScript.Bytes()),
			},
		},
	}

	// Unit without content to trigger restart of containerd.service when CAs change.
	containerdUnit := extensionsv1alpha1.Unit{
		Name:      UnitName,
		FilePaths: []string{rootcertificates.PathLocalSSLRootCerts},
	}

	monitorUnit := extensionsv1alpha1.Unit{
		Name:    UnitNameMonitor,
		Command: ptr.To(extensionsv1alpha1.CommandStart),
		Enable:  ptr.To(true),
		Content: ptr.To(`[Unit]
Description=Containerd-monitor daemon
After=` + UnitName + `
[Install]
WantedBy=multi-user.target
[Service]
Restart=always
EnvironmentFile=/etc/environment
ExecStart=` + pathHealthMonitor),
		FilePaths: []string{monitorFile.Path},
	}

	return append(logRotateUnits, containerdUnit, monitorUnit), append(logRotateFiles, monitorFile), nil
}

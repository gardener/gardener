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

package docker

import (
	"bytes"
	_ "embed"
	"text/template"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/operatingsystemconfig/original/components"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/operatingsystemconfig/original/components/docker/logrotate"
	"github.com/gardener/gardener/pkg/utils"

	"github.com/Masterminds/sprig"
	"k8s.io/utils/pointer"
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
	// UnitName is the name of the Docker service unit.
	UnitName = v1beta1constants.OperatingSystemConfigUnitNameDockerService
	// UnitNameMonitor is the name of the Docker monitor service unit.
	UnitNameMonitor = "docker-monitor.service"
	// PathBinary is the path to the docker binary.
	PathBinary = "/usr/bin/docker"
	// ContainerRuntime designates the runtime type.
	ContainerRuntime = "docker"
)

type component struct{}

// New returns a new docker component.
func New() *component {
	return &component{}
}

func (component) Name() string {
	return ContainerRuntime
}

func (component) Config(_ components.Context) ([]extensionsv1alpha1.Unit, []extensionsv1alpha1.File, error) {
	const (
		pathHealthMonitor   = v1beta1constants.OperatingSystemConfigFilePathBinaries + "/health-monitor-docker"
		pathLogRotateConfig = "/etc/systemd/docker.conf"
	)

	var healthMonitorScript bytes.Buffer
	if err := tplHealthMonitor.Execute(&healthMonitorScript, nil); err != nil {
		return nil, nil, err
	}

	logRotateUnits, logRotateFiles := logrotate.Config(pathLogRotateConfig, "/var/lib/docker/containers/*/*.log", ContainerRuntime)

	return append([]extensionsv1alpha1.Unit{
			{
				Name:    UnitNameMonitor,
				Command: pointer.String("start"),
				Enable:  pointer.Bool(true),
				Content: pointer.String(`[Unit]
Description=Docker-monitor daemon
After=` + UnitName + `
[Install]
WantedBy=multi-user.target
[Service]
Restart=always
EnvironmentFile=/etc/environment
ExecStart=` + pathHealthMonitor),
			},
		}, logRotateUnits...),
		append([]extensionsv1alpha1.File{
			{
				Path:        pathHealthMonitor,
				Permissions: pointer.Int32(0755),
				Content: extensionsv1alpha1.FileContent{
					Inline: &extensionsv1alpha1.FileContentInline{
						Encoding: "b64",
						Data:     utils.EncodeBase64(healthMonitorScript.Bytes()),
					},
				},
			},
		}, logRotateFiles...),
		nil
}

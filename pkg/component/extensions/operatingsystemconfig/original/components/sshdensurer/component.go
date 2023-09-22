// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package sshdensurer

import (
	"bytes"
	_ "embed"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/pointer"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components"
	"github.com/gardener/gardener/pkg/utils"
)

var (
	tplEnableSSHName = "sshd-enable"
	//go:embed templates/scripts/enable-sshd.tpl.sh
	tplEnableSSHScript string
	tplEnableSSH       *template.Template
	tplDisableSSHName  = "sshd-disable"
	//go:embed templates/scripts/disable-sshd.tpl.sh
	tplDisableSSHScript string
	tplDisableSSH       *template.Template
)

func init() {
	var err error
	tplEnableSSH, err = template.
		New(tplEnableSSHName).
		Funcs(sprig.TxtFuncMap()).
		Parse(tplEnableSSHScript)
	utilruntime.Must(err)

	tplDisableSSH, err = template.
		New(tplDisableSSHName).
		Funcs(sprig.TxtFuncMap()).
		Parse(tplDisableSSHScript)
	utilruntime.Must(err)
}

const (
	pathScript = "/var/lib/sshd-ensurer/run.sh"
)

type component struct{}

// New returns a new sshd-ensurer component.
func New() *component {
	return &component{}
}

func (component) Name() string {
	return "sshd-ensurer"
}

func (component) Config(ctx components.Context) ([]extensionsv1alpha1.Unit, []extensionsv1alpha1.File, error) {
	var script bytes.Buffer

	if ctx.SSHAccessEnabled {
		if err := tplEnableSSH.Execute(&script, nil); err != nil {
			return nil, nil, err
		}
	} else {
		if err := tplDisableSSH.Execute(&script, nil); err != nil {
			return nil, nil, err
		}
	}

	return []extensionsv1alpha1.Unit{
			{
				Name:    "sshd-ensurer.service",
				Command: pointer.String("start"),
				Content: pointer.String(`[Unit]
Description=Ensure SSHD service is enabled or disabled
DefaultDependencies=no
[Service]
Type=simple
Restart=always
RestartSec=15
ExecStart=` + pathScript + `
[Install]
WantedBy=multi-user.target`),
			},
		}, []extensionsv1alpha1.File{
			{
				Path:        pathScript,
				Permissions: pointer.Int32(0755),
				Content: extensionsv1alpha1.FileContent{
					Inline: &extensionsv1alpha1.FileContentInline{
						Encoding: "b64",
						Data:     utils.EncodeBase64(script.Bytes()),
					},
				},
			},
		}, nil
}

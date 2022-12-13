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

package sshddisabler

import (
	"bytes"
	_ "embed"
	"html/template"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/operatingsystemconfig/original/components"
	"github.com/gardener/gardener/pkg/utils"

	"github.com/Masterminds/sprig"
	"k8s.io/utils/pointer"
)

var (
	tplName = "disable-sshd"
	//go:embed templates/scripts/disable-sshd.tpl.sh
	tplContent string
	tpl        *template.Template
)

func init() {
	var err error
	tpl, err = template.
		New(tplName).
		Funcs(sprig.TxtFuncMap()).
		Parse(tplContent)
	if err != nil {
		panic(err)
	}
}

const (
	pathScript = "/var/lib/sshd-disabler/run.sh"
)

type component struct{}

// New returns a new sshddisabler component.
func New() *component {
	return &component{}
}

func (component) Name() string {
	return "sshddisabler"
}

func (component) Config(ctx components.Context) ([]extensionsv1alpha1.Unit, []extensionsv1alpha1.File, error) {
	var script bytes.Buffer
	if err := tpl.Execute(&script, map[string]interface{}{}); err != nil {
		return nil, nil, err
	}

	if ctx.EnsureSSHAccessDisabled {
		return []extensionsv1alpha1.Unit{
				{
					Name:    "sshddisabler.service",
					Command: pointer.String("start"),
					Content: pointer.String(`[Unit]
Description=Disable ssh access and kill any currently established ssh connections
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

	return []extensionsv1alpha1.Unit{
		{
			Name:    "sshddisabler.service",
			Command: pointer.String("start"),
			Content: pointer.String(`[Unit]
Description=Disable ssh access and kill any currently established ssh connections
DefaultDependencies=no
[Service]
Type=simple
ExecStart=/bin/echo "service sshddisabler is disabled in workers settings."
[Install]
WantedBy=multi-user.target`),
		},
	}, nil, nil
}

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

package containerd

import (
	"bytes"
	_ "embed"
	"text/template"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/operatingsystemconfig/original/components"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/images"

	"github.com/Masterminds/sprig"
	"k8s.io/utils/pointer"
)

var (
	tplNameInitializer = "init"
	//go:embed templates/scripts/init.tpl.sh
	tplContentInitializer string
	tplInitializer        *template.Template
)

func init() {
	var err error
	tplInitializer, err = template.
		New(tplNameInitializer).
		Funcs(sprig.TxtFuncMap()).
		Parse(tplContentInitializer)
	if err != nil {
		panic(err)
	}
}

type initializer struct{}

// NewInitializer returns a new containerd initializer component.
func NewInitializer() *initializer {
	return &initializer{}
}

func (initializer) Name() string {
	return "containerd-initializer"
}

func (initializer) Config(ctx components.Context) ([]extensionsv1alpha1.Unit, []extensionsv1alpha1.File, error) {
	const (
		pathScript          = v1beta1constants.OperatingSystemConfigFilePathBinaries + "/init-containerd"
		unitNameInitializer = "containerd-initializer.service"
	)

	var script bytes.Buffer
	if err := tplInitializer.Execute(&script, map[string]interface{}{
		"binaryPath":          extensionsv1alpha1.ContainerDRuntimeContainersBinFolder,
		"pauseContainerImage": ctx.Images[images.ImageNamePauseContainer],
	}); err != nil {
		return nil, nil, err
	}

	return []extensionsv1alpha1.Unit{
			{
				Name:    unitNameInitializer,
				Command: pointer.String("start"),
				Enable:  pointer.Bool(true),
				Content: pointer.String(`[Unit]
Description=Containerd initializer
[Install]
WantedBy=multi-user.target
[Service]
Type=oneshot
RemainAfterExit=yes
ExecStart=` + pathScript),
			},
		},
		[]extensionsv1alpha1.File{
			{
				Path:        pathScript,
				Permissions: pointer.Int32(744),
				Content: extensionsv1alpha1.FileContent{
					Inline: &extensionsv1alpha1.FileContentInline{
						Encoding: "b64",
						Data:     utils.EncodeBase64(script.Bytes()),
					},
				},
			},
			{
				Path:        "/etc/systemd/system/containerd.service.d/10-require-containerd-initializer.conf",
				Permissions: pointer.Int32(0644),
				Content: extensionsv1alpha1.FileContent{
					Inline: &extensionsv1alpha1.FileContentInline{
						Data: `[Unit]
After=` + unitNameInitializer + `
Requires=` + unitNameInitializer,
					},
				},
			},
		},
		nil
}

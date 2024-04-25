// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package containerd

import (
	"bytes"
	_ "embed"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/imagevector"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components"
	"github.com/gardener/gardener/pkg/utils"
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
		"pauseContainerImage": ctx.Images[imagevector.ImageNamePauseContainer],
	}); err != nil {
		return nil, nil, err
	}

	return []extensionsv1alpha1.Unit{
			{
				Name:    unitNameInitializer,
				Command: ptr.To(extensionsv1alpha1.CommandStart),
				Enable:  ptr.To(true),
				Content: ptr.To(`[Unit]
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
				Permissions: ptr.To[int32](744),
				Content: extensionsv1alpha1.FileContent{
					Inline: &extensionsv1alpha1.FileContentInline{
						Encoding: "b64",
						Data:     utils.EncodeBase64(script.Bytes()),
					},
				},
			},
			{
				Path:        "/etc/systemd/system/containerd.service.d/10-require-containerd-initializer.conf",
				Permissions: ptr.To[int32](0644),
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

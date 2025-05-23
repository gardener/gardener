// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package sshdensurer

import (
	"bytes"
	_ "embed"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/ptr"

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

	sshdEnsurerFile := extensionsv1alpha1.File{
		Path:        pathScript,
		Permissions: ptr.To[uint32](0755),
		Content: extensionsv1alpha1.FileContent{
			Inline: &extensionsv1alpha1.FileContentInline{
				Encoding: "b64",
				Data:     utils.EncodeBase64(script.Bytes()),
			},
		},
	}

	sshdEnsurerUnit := extensionsv1alpha1.Unit{
		Name:    "sshd-ensurer.service",
		Command: ptr.To(extensionsv1alpha1.CommandStart),
		Content: ptr.To(`[Unit]
Description=Ensure SSHD service is enabled or disabled
DefaultDependencies=no
[Service]
Type=simple
Restart=always
RestartSec=15
ExecStart=` + pathScript + `
[Install]
WantedBy=multi-user.target`),
		FilePaths: []string{sshdEnsurerFile.Path},
	}

	return []extensionsv1alpha1.Unit{sshdEnsurerUnit}, []extensionsv1alpha1.File{sshdEnsurerFile}, nil
}

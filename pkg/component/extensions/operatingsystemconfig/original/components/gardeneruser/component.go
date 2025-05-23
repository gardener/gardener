// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardeneruser

import (
	"bytes"
	_ "embed"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"k8s.io/utils/ptr"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components"
	"github.com/gardener/gardener/pkg/utils"
)

var (
	tplName = "reconcile"
	//go:embed templates/scripts/reconcile.tpl.sh
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
	pathScript = "/var/lib/gardener-user/run.sh"

	// pathPublicSSHKey is the old file that contained just a single SSH public key.
	// If this file is found on a node, it will be deleted.
	pathPublicSSHKey = "/var/lib/gardener-user-ssh.key"

	// pathAuthorizedSSHKeys is the new file that can contain multiple SSH public keys.
	pathAuthorizedSSHKeys = "/var/lib/gardener-user-authorized-keys"
)

type component struct{}

// New returns a new Gardener user component.
func New() *component {
	return &component{}
}

func (component) Name() string {
	return "gardener-user"
}

func (component) Config(ctx components.Context) ([]extensionsv1alpha1.Unit, []extensionsv1alpha1.File, error) {
	var script bytes.Buffer
	if err := tpl.Execute(&script, map[string]any{
		"pathPublicSSHKey":      pathPublicSSHKey,
		"pathAuthorizedSSHKeys": pathAuthorizedSSHKeys,
	}); err != nil {
		return nil, nil, err
	}

	authorizedKeys := strings.Join(ctx.SSHPublicKeys, "\n")

	return []extensionsv1alpha1.Unit{
			{
				Name:   "gardener-user.service",
				Enable: ptr.To(true),
				Content: ptr.To(`[Unit]
Description=Configure gardener user
After=sshd.service
[Service]
Restart=on-failure
EnvironmentFile=/etc/environment
ExecStart=` + pathScript + `
`),
			},
			{
				Name:   "gardener-user.path",
				Enable: ptr.To(true),
				Content: ptr.To(`[Path]
PathChanged=` + pathAuthorizedSSHKeys + `
[Install]
WantedBy=multi-user.target
`),
			},
		},
		[]extensionsv1alpha1.File{
			{
				Path:        pathAuthorizedSSHKeys,
				Permissions: ptr.To[uint32](0644),
				Content: extensionsv1alpha1.FileContent{
					Inline: &extensionsv1alpha1.FileContentInline{
						Encoding: "b64",
						Data:     utils.EncodeBase64([]byte(authorizedKeys)),
					},
				},
			},
			{
				Path:        pathScript,
				Permissions: ptr.To[uint32](0755),
				Content: extensionsv1alpha1.FileContent{
					Inline: &extensionsv1alpha1.FileContentInline{
						Encoding: "b64",
						Data:     utils.EncodeBase64(script.Bytes()),
					},
				},
			},
		},
		nil
}

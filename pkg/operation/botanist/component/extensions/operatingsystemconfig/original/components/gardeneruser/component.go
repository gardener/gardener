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

package gardeneruser

import (
	"bytes"
	_ "embed"
	"strings"
	"text/template"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/operatingsystemconfig/original/components"
	"github.com/gardener/gardener/pkg/utils"

	"github.com/Masterminds/sprig"
	"k8s.io/utils/pointer"
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
	if err := tpl.Execute(&script, map[string]interface{}{
		"pathPublicSSHKey":      pathPublicSSHKey,
		"pathAuthorizedSSHKeys": pathAuthorizedSSHKeys,
	}); err != nil {
		return nil, nil, err
	}

	authorizedKeys := strings.Join(ctx.SSHPublicKeys, "\n")

	return []extensionsv1alpha1.Unit{
			{
				Name:   "gardener-user.service",
				Enable: pointer.Bool(true),
				Content: pointer.String(`[Unit]
Description=Configure gardener user
After=sshd.service
[Service]
Restart=on-failure
EnvironmentFile=/etc/environment
ExecStart=` + pathScript + `
`),
			},
		},
		[]extensionsv1alpha1.File{
			{
				Path:        pathAuthorizedSSHKeys,
				Permissions: pointer.Int32(0644),
				Content: extensionsv1alpha1.FileContent{
					Inline: &extensionsv1alpha1.FileContentInline{
						Encoding: "b64",
						Data:     utils.EncodeBase64([]byte(authorizedKeys)),
					},
				},
			},
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
		},
		nil
}

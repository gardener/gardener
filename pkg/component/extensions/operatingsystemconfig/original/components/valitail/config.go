// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package valitail

import (
	"bytes"
	_ "embed"
	"errors"
	"net/url"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"k8s.io/utils/ptr"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components"
	"github.com/gardener/gardener/pkg/utils"
)

var (
	tplNameValitail = "config"
	//go:embed templates/valitail-config.tpl.yaml
	tplContentValitail string
	tplValitail        *template.Template
)

func init() {
	tplValitail = template.Must(template.
		New(tplNameValitail).
		Funcs(sprig.TxtFuncMap()).
		Parse(tplContentValitail))
}

func getValitailConfigurationFile(ctx components.Context) (extensionsv1alpha1.File, error) {
	if ctx.ValiIngress == "" {
		return extensionsv1alpha1.File{}, errors.New("vali ingress url is missing")
	}

	apiServerURL, err := url.Parse(ctx.APIServerURL)
	if err != nil {
		return extensionsv1alpha1.File{}, err
	}

	var config bytes.Buffer
	if err := tplValitail.Execute(&config, map[string]interface{}{
		"clientURL":         "https://" + ctx.ValiIngress + "/vali/api/v1/push",
		"pathCACert":        PathCACert,
		"valiIngress":       ctx.ValiIngress,
		"pathAuthToken":     PathAuthToken,
		"APIServerURL":      ctx.APIServerURL,
		"APIServerHostname": apiServerURL.Hostname(),
	}); err != nil {
		return extensionsv1alpha1.File{}, err
	}

	return extensionsv1alpha1.File{
		Path:        PathConfig,
		Permissions: ptr.To[int32](0644),
		Content: extensionsv1alpha1.FileContent{
			Inline: &extensionsv1alpha1.FileContentInline{
				Encoding: "b64",
				Data:     utils.EncodeBase64(config.Bytes()),
			},
		},
	}, nil
}

func getValitailCAFile(ctx components.Context) extensionsv1alpha1.File {
	var cABundle []byte
	if ctx.CABundle != nil {
		cABundle = []byte(*ctx.CABundle)
	}

	return extensionsv1alpha1.File{
		Path:        PathCACert,
		Permissions: ptr.To[int32](0644),
		Content: extensionsv1alpha1.FileContent{
			Inline: &extensionsv1alpha1.FileContentInline{
				Encoding: "b64",
				Data:     utils.EncodeBase64(cABundle),
			},
		},
	}
}

func getValitailUnit() extensionsv1alpha1.Unit {
	return extensionsv1alpha1.Unit{
		Name:    UnitName,
		Command: ptr.To(extensionsv1alpha1.CommandStart),
		Enable:  ptr.To(true),
		Content: ptr.To(`[Unit]
Description=valitail daemon
Documentation=https://github.com/credativ/plutono
[Install]
WantedBy=multi-user.target
[Service]
CPUAccounting=yes
MemoryAccounting=yes
CPUQuota=3%
CPUQuotaPeriodSec=1000ms
MemoryMin=29M
MemoryHigh=400M
MemoryMax=800M
MemorySwapMax=0
Restart=always
RestartSec=5
EnvironmentFile=/etc/environment
ExecStartPre=/bin/sh -c "systemctl set-environment HOSTNAME=$(hostname | tr [:upper:] [:lower:])"
ExecStart=` + v1beta1constants.OperatingSystemConfigFilePathBinaries + `/valitail -config.file=` + PathConfig),
		FilePaths: []string{PathConfig, PathCACert, valitailBinaryPath},
	}
}

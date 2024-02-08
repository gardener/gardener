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
	"fmt"
	"net/url"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/imagevector"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/downloader"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components"
	"github.com/gardener/gardener/pkg/component/logging/vali"
	"github.com/gardener/gardener/pkg/features"
	nodeagentv1alpha1 "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
)

var (
	// TODO(rfranzke): Remove the fetch-token script when the UseGardenerNodeAgent feature gate gets removed.
	tplNameFetchToken = "fetch-token"
	//go:embed templates/scripts/fetch-token.tpl.sh
	tplContentFetchToken string
	tplFetchToken        *template.Template

	tplNameValitail = "fetch-token"
	//go:embed templates/valitail-config.tpl.yaml
	tplContentValitail string
	tplValitail        *template.Template
)

func init() {
	var err error
	tplFetchToken, err = template.
		New(tplNameFetchToken).
		Funcs(sprig.TxtFuncMap()).
		Parse(tplContentFetchToken)
	if err != nil {
		panic(err)
	}

	tplValitail, err = template.
		New(tplNameValitail).
		Funcs(sprig.TxtFuncMap()).
		Parse(tplContentValitail)
	if err != nil {
		panic(err)
	}
}

func getValitailConfigurationFile(ctx components.Context) (extensionsv1alpha1.File, error) {
	var config bytes.Buffer

	if ctx.ValiIngress == "" {
		return extensionsv1alpha1.File{}, fmt.Errorf("vali ingress url is missing")
	}

	apiServerURL, err := url.Parse(ctx.APIServerURL)
	if err != nil {
		return extensionsv1alpha1.File{}, err
	}

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
		Permissions: ptr.To(int32(0644)),
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
		Permissions: ptr.To(int32(0644)),
		Content: extensionsv1alpha1.FileContent{
			Inline: &extensionsv1alpha1.FileContentInline{
				Encoding: "b64",
				Data:     utils.EncodeBase64(cABundle),
			},
		},
	}
}

func getValitailUnit(ctx components.Context) extensionsv1alpha1.Unit {
	var (
		execStartPre string
		execStart    = v1beta1constants.OperatingSystemConfigFilePathBinaries + `/valitail -config.file=` + PathConfig
	)

	if !features.DefaultFeatureGate.Enabled(features.UseGardenerNodeAgent) {
		if ctx.ValitailEnabled {
			execStartPre = execStartPreCopyBinaryFromContainer("valitail", ctx.Images[imagevector.ImageNameValitail])
		}
	}

	// TODO(rfranzke): Drop this 'disablement' handling once UseGardenerNodeAgent feature gate gets removed.
	//  gardener-node-agent takes care of disabling and removing the systemd service when it's no longer present in the
	//  operating system configuration.
	if !ctx.ValitailEnabled {
		execStartPre = "/bin/systemctl disable " + UnitName
		execStart = fmt.Sprintf(`/bin/sh -c "echo service %s is removed!; while true; do sleep 86400; done"`, UnitName)
	}

	unitContent := `[Unit]
Description=valitail daemon
Documentation=https://github.com/credativ/plutono`

	if !features.DefaultFeatureGate.Enabled(features.UseGardenerNodeAgent) {
		unitContent += `
After=` + unitNameFetchToken
	}

	unitContent += `
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
ExecStartPre=/bin/sh -c "systemctl set-environment HOSTNAME=$(hostname | tr [:upper:] [:lower:])"`

	if execStartPre != "" {
		unitContent += `
ExecStartPre=` + execStartPre
	}

	unitContent += `
ExecStart=` + execStart

	unit := extensionsv1alpha1.Unit{
		Name:    UnitName,
		Command: ptr.To(extensionsv1alpha1.CommandStart),
		Enable:  ptr.To(true),
		Content: &unitContent,
	}

	if ctx.ValitailEnabled {
		unit.FilePaths = []string{PathConfig, PathCACert}

		if features.DefaultFeatureGate.Enabled(features.UseGardenerNodeAgent) {
			unit.FilePaths = append(unit.FilePaths, valitailBinaryPath)
		}
	}

	return unit
}

func getFetchTokenScriptFile() (extensionsv1alpha1.File, error) {
	var script bytes.Buffer
	if err := tplFetchToken.Execute(&script, map[string]interface{}{
		"pathCredentialsToken":  downloader.PathCredentialsToken,
		"pathCredentialsServer": downloader.PathCredentialsServer,
		"pathCredentialsCACert": downloader.PathCredentialsCACert,
		"pathAuthToken":         PathAuthToken,
		"dataKeyToken":          resourcesv1alpha1.DataKeyToken,
		"secretName":            vali.ValitailTokenSecretName,
	}); err != nil {
		return extensionsv1alpha1.File{}, err
	}

	return extensionsv1alpha1.File{
		Path:        PathFetchTokenScript,
		Permissions: ptr.To(int32(0744)),
		Content: extensionsv1alpha1.FileContent{
			Inline: &extensionsv1alpha1.FileContentInline{
				Encoding: "b64",
				Data:     utils.EncodeBase64(script.Bytes()),
			},
		},
	}, nil
}

func getFetchTokenScriptUnit(ctx components.Context) extensionsv1alpha1.Unit {
	var (
		execStartPre string
		execStart    = PathFetchTokenScript
	)

	if !ctx.ValitailEnabled {
		execStartPre = "/bin/systemctl disable " + unitNameFetchToken
		execStart = fmt.Sprintf(`/bin/sh -c "rm -f `+PathAuthToken+`; echo service %s is removed!; while true; do sleep 86400; done"`, unitNameFetchToken)
	}

	afterUnit := downloader.UnitName
	if features.DefaultFeatureGate.Enabled(features.UseGardenerNodeAgent) {
		afterUnit = nodeagentv1alpha1.UnitName
	}

	unitContent := `[Unit]
Description=valitail token fetcher
After=` + afterUnit + `
[Install]
WantedBy=multi-user.target
[Service]
Restart=always
RestartSec=300
RuntimeMaxSec=120
EnvironmentFile=/etc/environment`

	if execStartPre != "" {
		unitContent += `
ExecStartPre=` + execStartPre
	}

	unitContent += `
ExecStart=` + execStart

	unit := extensionsv1alpha1.Unit{
		Name:    unitNameFetchToken,
		Command: ptr.To(extensionsv1alpha1.CommandStart),
		Enable:  ptr.To(true),
		Content: &unitContent,
	}

	if ctx.ValitailEnabled {
		unit.FilePaths = []string{PathFetchTokenScript}
	}

	return unit
}

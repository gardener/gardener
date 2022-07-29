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

package promtail

import (
	"bytes"
	_ "embed"
	"fmt"
	"net/url"
	"text/template"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/operatingsystemconfig/downloader"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/operatingsystemconfig/original/components"
	"github.com/gardener/gardener/pkg/operation/botanist/component/logging/kuberbacproxy"

	"github.com/Masterminds/sprig"
	"github.com/gardener/gardener/pkg/utils"
	"k8s.io/utils/pointer"
)

var (
	tplNameFetchToken = "fetch-token"
	//go:embed templates/scripts/fetch-token.tpl.sh
	tplContentFetchToken string
	tplFetchToken        *template.Template

	tplNamePromtail = "fetch-token"
	//go:embed templates/promtail-config.tpl.yaml
	tplContentPromtail string
	tplPromtail        *template.Template
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

	tplPromtail, err = template.
		New(tplNamePromtail).
		Funcs(sprig.TxtFuncMap()).
		Parse(tplContentPromtail)
	if err != nil {
		panic(err)
	}
}

const setActiveJournalFileScript = `#!/bin/bash
PERSISTANT_JOURNAL_FILE=/var/log/journal
TEMP_JOURNAL_FILE=/run/log/journal
if [ ! -d "$PERSISTANT_JOURNAL_FILE" ] && [ -d "$TEMP_JOURNAL_FILE" ]; then
	sed -i -e "s|$PERSISTANT_JOURNAL_FILE|$TEMP_JOURNAL_FILE|g" ` + PathConfig + `
elif  [ ! -d "$TEMP_JOURNAL_FILE" ] && [ -d "$PERSISTANT_JOURNAL_FILE" ]; then
	sed -i -e "s|$TEMP_JOURNAL_FILE$|$PERSISTANT_JOURNAL_FILE|g" ` + PathConfig + `
fi`

func getPromtailConfigurationFile(ctx components.Context) (extensionsv1alpha1.File, error) {
	var config bytes.Buffer

	if ctx.LokiIngress == "" {
		return extensionsv1alpha1.File{}, fmt.Errorf("loki ingress url is missing")
	}

	apiServerURL, err := url.Parse(ctx.APIServerURL)
	if err != nil {
		return extensionsv1alpha1.File{}, err
	}

	if err := tplPromtail.Execute(&config, map[string]interface{}{
		"clientURL":         "https://" + ctx.LokiIngress + "/loki/api/v1/push",
		"pathCACert":        PathCACert,
		"lokiIngress":       ctx.LokiIngress,
		"pathAuthToken":     PathAuthToken,
		"APIServerURL":      ctx.APIServerURL,
		"APIServerHostname": apiServerURL.Hostname(),
	}); err != nil {
		return extensionsv1alpha1.File{}, err
	}

	return extensionsv1alpha1.File{
		Path:        PathConfig,
		Permissions: pointer.Int32(0644),
		Content: extensionsv1alpha1.FileContent{
			Inline: &extensionsv1alpha1.FileContentInline{
				Encoding: "b64",
				Data:     utils.EncodeBase64(config.Bytes()),
			},
		},
	}, nil
}

func getPromtailCAFile(ctx components.Context) extensionsv1alpha1.File {
	var cABundle []byte
	if ctx.CABundle != nil {
		cABundle = []byte(*ctx.CABundle)
	}
	return extensionsv1alpha1.File{
		Path:        PathCACert,
		Permissions: pointer.Int32(0644),
		Content: extensionsv1alpha1.FileContent{
			Inline: &extensionsv1alpha1.FileContentInline{
				Encoding: "b64",
				Data:     utils.EncodeBase64(cABundle),
			},
		},
	}
}

func setActiveJournalFile() extensionsv1alpha1.File {
	return extensionsv1alpha1.File{
		Path:        PathSetActiveJournalFileScript,
		Permissions: pointer.Int32(0644),
		Content: extensionsv1alpha1.FileContent{
			Inline: &extensionsv1alpha1.FileContentInline{
				Encoding: "b64",
				Data:     utils.EncodeBase64([]byte(setActiveJournalFileScript)),
			},
		},
	}
}

func getPromtailUnit(execStartPre, execStartPreConfig, execStart string) extensionsv1alpha1.Unit {
	return extensionsv1alpha1.Unit{
		Name:    UnitName,
		Command: pointer.String("start"),
		Enable:  pointer.Bool(true),
		Content: pointer.String(`[Unit]
Description=promtail daemon
Documentation=https://grafana.com/docs/loki/latest/clients/promtail/
After=` + unitNameFetchToken + `
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
ExecStartPre=/bin/sh -c "systemctl set-environment HOSTNAME=$(hostname)"
ExecStartPre=` + execStartPre + `
ExecStartPre=` + execStartPreConfig + `
ExecStart=` + execStart),
	}
}

func getFetchTokenScriptFile() (extensionsv1alpha1.File, error) {
	var script bytes.Buffer
	if err := tplFetchToken.Execute(&script, map[string]interface{}{
		"pathCredentialsToken":  downloader.PathCredentialsToken,
		"pathCredentialsServer": downloader.PathCredentialsServer,
		"pathCredentialsCACert": downloader.PathCredentialsCACert,
		"pathAuthToken":         PathAuthToken,
		"dataKeyToken":          resourcesv1alpha1.DataKeyToken,
		"secretName":            kuberbacproxy.PromtailTokenSecretName,
	}); err != nil {
		return extensionsv1alpha1.File{}, err
	}

	return extensionsv1alpha1.File{
		Path:        PathFetchTokenScript,
		Permissions: pointer.Int32(0744),
		Content: extensionsv1alpha1.FileContent{
			Inline: &extensionsv1alpha1.FileContentInline{
				Encoding: "b64",
				Data:     utils.EncodeBase64(script.Bytes()),
			},
		},
	}, nil
}

func getFetchTokenScriptUnit(execStartPre, execStart string) extensionsv1alpha1.Unit {
	unitContent := `[Unit]
Description=promtail token fetcher
After=` + downloader.UnitName + `
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

	return extensionsv1alpha1.Unit{
		Name:    unitNameFetchToken,
		Command: pointer.String("start"),
		Enable:  pointer.Bool(true),
		Content: &unitContent,
	}
}

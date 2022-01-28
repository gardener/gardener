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
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/operatingsystemconfig/original/components"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/images"
	"github.com/gardener/gardener/pkg/utils/imagevector"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"
	"k8s.io/utils/pointer"
)

var _ = Describe("Promtail", func() {
	Describe("#Config", func() {
		var (
			cABundle           = "malskjdvbfnasufbaus"
			clusterDomain      = "testClusterDomain.com"
			promtailImageName  = "Promtail"
			promtailRepository = "github.com/promtail"
			promtailImageTag   = "v0.1.0"
			promtailImage      = &imagevector.Image{
				Name:       promtailImageName,
				Repository: promtailRepository,
				Tag:        &promtailImageTag,
			}
			lokiIngress = "ingress.loki.testClusterDomain"
		)

		It("should return the expected units and files when shoot logging is enabled", func() {
			ctx := components.Context{
				CABundle:      &cABundle,
				ClusterDomain: clusterDomain,
				Images: map[string]*imagevector.Image{
					images.ImageNamePromtail: promtailImage,
				},
				LokiIngress:     lokiIngress,
				PromtailEnabled: true,
			}

			conf := defaultConfig
			conf.Client.Url = "https://" + lokiIngress + "/loki/api/v1/push"
			conf.Client.TLSConfig.ServerName = lokiIngress
			configYaml, err := yaml.Marshal(&conf)
			Expect(err).To(BeNil())

			units, files, err := New().Config(ctx)
			Expect(err).To(BeNil())

			Expect(units).To(ConsistOf(
				extensionsv1alpha1.Unit{
					Name:    UnitName,
					Command: pointer.String("start"),
					Enable:  pointer.Bool(true),
					Content: pointer.String(`[Unit]
Description=promtail daemon
Documentation=https://grafana.com/docs/loki/latest/clients/promtail/
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
ExecStartPre=/usr/bin/docker run --rm -v /opt/bin:/opt/bin:rw --entrypoint /bin/sh ` + promtailRepository + ":" + promtailImageTag + " -c " + "\"cp /usr/bin/promtail /opt/bin\"" + `
ExecStartPre=/bin/sh ` + PathSetActiveJournalFileScript + `
ExecStart=/opt/bin/promtail -config.file=` + PathConfig),
				},
				extensionsv1alpha1.Unit{
					Name:    "promtail-fetch-token.service",
					Command: pointer.String("start"),
					Enable:  pointer.Bool(true),
					Content: pointer.String(`[Unit]
Description=promtail token fetcher
After=cloud-config-downloader.service
[Install]
WantedBy=multi-user.target
[Service]
Restart=always
RestartSec=300
RuntimeMaxSec=120
EnvironmentFile=/etc/environment
ExecStart=/var/lib/promtail/scripts/fetch-token.sh`),
				},
			))

			Expect(files).To(ConsistOf(
				extensionsv1alpha1.File{
					Path:        "/var/lib/promtail/config/config",
					Permissions: pointer.Int32(0644),
					Content: extensionsv1alpha1.FileContent{
						Inline: &extensionsv1alpha1.FileContentInline{
							Encoding: "b64",
							Data:     utils.EncodeBase64(configYaml),
						},
					},
				},
				extensionsv1alpha1.File{
					Path:        "/var/lib/promtail/scripts/fetch-token.sh",
					Permissions: pointer.Int32(0744),
					Content: extensionsv1alpha1.FileContent{
						Inline: &extensionsv1alpha1.FileContentInline{
							Encoding: "b64",
							Data: utils.EncodeBase64([]byte(`#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

{
if ! SECRET="$(wget \
  -qO- \
  --header         "Accept: application/yaml" \
  --header         "Authorization: Bearer $(cat "/var/lib/cloud-config-downloader/credentials/token")" \
  --ca-certificate "/var/lib/cloud-config-downloader/credentials/ca.crt" \
  "$(cat "/var/lib/cloud-config-downloader/credentials/server")/api/v1/namespaces/kube-system/secrets/gardener-promtail")"; then

  echo "Could not retrieve the promtail token secret"
  exit 1
fi

echo "$SECRET" | sed -rn "s/  token: (.*)/\1/p" | base64 -d > "/var/lib/promtail/auth-token"

exit $?
}
`)),
						},
					},
				},
				extensionsv1alpha1.File{
					Path:        "/var/lib/promtail/ca.crt",
					Permissions: pointer.Int32(0644),
					Content: extensionsv1alpha1.FileContent{
						Inline: &extensionsv1alpha1.FileContentInline{
							Encoding: "b64",
							Data:     utils.EncodeBase64([]byte(cABundle)),
						},
					},
				},
				extensionsv1alpha1.File{
					Path:        "/var/lib/promtail/scripts/set_active_journal_file.sh",
					Permissions: pointer.Int32(0644),
					Content: extensionsv1alpha1.FileContent{
						Inline: &extensionsv1alpha1.FileContentInline{
							Encoding: "b64",
							Data:     utils.EncodeBase64([]byte(setActiveJournalFileScript)),
						},
					},
				},
			))
		})

		It("should return the expected units and files when shoot logging is not enabled", func() {
			ctx := components.Context{
				CABundle:      &cABundle,
				ClusterDomain: clusterDomain,
				Images: map[string]*imagevector.Image{
					images.ImageNamePromtail: promtailImage,
				},
				LokiIngress:     lokiIngress,
				PromtailEnabled: false,
			}

			units, files, err := New().Config(ctx)
			Expect(err).To(BeNil())

			Expect(units).To(ConsistOf(
				extensionsv1alpha1.Unit{
					Name:    "promtail.service",
					Command: pointer.String("start"),
					Enable:  pointer.Bool(true),
					Content: pointer.String(`[Unit]
Description=promtail daemon
Documentation=https://grafana.com/docs/loki/latest/clients/promtail/
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
ExecStartPre=/bin/systemctl disable promtail.service
ExecStartPre=/bin/sh -c "echo 'service does not have configuration'"
ExecStart=/bin/sh -c "echo service promtail.service is removed!; while true; do sleep 86400; done"`),
				},
				extensionsv1alpha1.Unit{
					Name:    "promtail-fetch-token.service",
					Command: pointer.String("start"),
					Enable:  pointer.Bool(true),
					Content: pointer.String(`[Unit]
Description=promtail token fetcher
After=cloud-config-downloader.service
[Install]
WantedBy=multi-user.target
[Service]
Restart=always
RestartSec=300
RuntimeMaxSec=120
EnvironmentFile=/etc/environment
ExecStartPre=/bin/systemctl disable promtail-fetch-token.service
ExecStart=/bin/sh -c "rm -f /var/lib/promtail/auth-token; echo service promtail-fetch-token.service is removed!; while true; do sleep 86400; done"`),
				},
			))

			Expect(files).To(BeNil())
		})

		It("should return error when loki ingress is not specified", func() {
			ctx := components.Context{
				CABundle:      &cABundle,
				ClusterDomain: clusterDomain,
				Images: map[string]*imagevector.Image{
					images.ImageNamePromtail: promtailImage,
				},
				PromtailEnabled: true,
				LokiIngress:     "",
			}

			units, files, err := New().Config(ctx)
			Expect(err).To(MatchError(ContainSubstring("loki ingress url is missing")))
			Expect(units).To(BeNil())
			Expect(files).To(BeNil())
		})
	})
})

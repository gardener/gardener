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
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	"github.com/gardener/gardener/pkg/utils/test"
)

var _ = Describe("Valitail", func() {
	Describe("#Config", func() {
		var (
			cABundle           = "malskjdvbfnasufbaus"
			clusterDomain      = "testClusterDomain.com"
			apiServerURL       = "https://api.test-cluster.com"
			valitailImageName  = "Valitail"
			valitailRepository = "github.com/valitail"
			valitailImageTag   = "v0.1.0"
			valitailImage      = &imagevector.Image{
				Name:       valitailImageName,
				Repository: valitailRepository,
				Tag:        &valitailImageTag,
			}
			valiIngress = "ingress.vali.testClusterDomain"
		)

		testConfig := func(useGardenerNodeAgentEnabled bool) {
			Context(fmt.Sprintf("UseGardenerNodeAgent: %v", useGardenerNodeAgentEnabled), func() {
				It("should return the expected units and files when shoot logging is enabled", func() {
					defer test.WithFeatureGate(features.DefaultFeatureGate, features.UseGardenerNodeAgent, useGardenerNodeAgentEnabled)()
					ctx := components.Context{
						CABundle:      &cABundle,
						ClusterDomain: clusterDomain,
						Images: map[string]*imagevector.Image{
							"valitail": valitailImage,
						},
						ValiIngress:     valiIngress,
						ValitailEnabled: true,
						APIServerURL:    apiServerURL,
					}

					units, files, err := New().Config(ctx)
					Expect(err).NotTo(HaveOccurred())

					var (
						afterUnit              = "gardener-node-agent.service"
						valitailDaemonStartPre string
					)

					if !useGardenerNodeAgentEnabled {
						afterUnit = "cloud-config-downloader.service"
						valitailDaemonStartPre = `
ExecStartPre=/usr/bin/docker run --rm -v /opt/bin:/opt/bin:rw --entrypoint /bin/sh ` + valitailRepository + ":" + valitailImageTag + " -c " + "\"cp /usr/bin/valitail /opt/bin\""
					}

					unitContent := `[Unit]
Description=valitail daemon
Documentation=https://github.com/credativ/plutono`

					if !useGardenerNodeAgentEnabled {
						unitContent += `
After=valitail-fetch-token.service`
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
ExecStartPre=/bin/sh -c "systemctl set-environment HOSTNAME=$(hostname | tr [:upper:] [:lower:])"` + valitailDaemonStartPre + `
ExecStart=/opt/bin/valitail -config.file=` + PathConfig

					valitailDaemonUnit := extensionsv1alpha1.Unit{
						Name:    UnitName,
						Command: ptr.To(extensionsv1alpha1.CommandStart),
						Enable:  ptr.To(true),
						Content: ptr.To(unitContent),
					}

					valitailTokenFetchUnit := extensionsv1alpha1.Unit{
						Name:    "valitail-fetch-token.service",
						Command: ptr.To(extensionsv1alpha1.CommandStart),
						Enable:  ptr.To(true),
						Content: ptr.To(`[Unit]
Description=valitail token fetcher
After=` + afterUnit + `
[Install]
WantedBy=multi-user.target
[Service]
Restart=always
RestartSec=300
RuntimeMaxSec=120
EnvironmentFile=/etc/environment
ExecStart=/var/lib/valitail/scripts/fetch-token.sh`),
						FilePaths: []string{"/var/lib/valitail/scripts/fetch-token.sh"},
					}

					valitailConfigFile := extensionsv1alpha1.File{
						Path:        "/var/lib/valitail/config/config",
						Permissions: ptr.To(int32(0644)),
						Content: extensionsv1alpha1.FileContent{
							Inline: &extensionsv1alpha1.FileContentInline{
								Encoding: "b64",
								Data: utils.EncodeBase64([]byte(`server:
  disable: true
  log_level: info
  http_listen_port: 3001
client:
  url: https://ingress.vali.testClusterDomain/vali/api/v1/push
  batchwait: 10s
  batchsize: 1536000
  bearer_token_file: /var/lib/valitail/auth-token
  tls_config:
    ca_file: /var/lib/valitail/ca.crt
    server_name: ingress.vali.testClusterDomain
positions:
  filename: /var/log/positions.yaml
scrape_configs:
- job_name: journal
  journal:
    json: false
    labels:
      job: systemd-journal
      origin: systemd-journal
    max_age: 12h
  relabel_configs:
  - action: drop
    regex: ^localhost$
    source_labels: ['__journal__hostname']
  - action: replace
    regex: '(.+)'
    replacement: $1
    source_labels: ['__journal__systemd_unit']
    target_label: '__journal_syslog_identifier'
  - action: keep
    regex: ^kernel|kubelet\.service|docker\.service|containerd\.service|gardener-node-agent\.service$
    source_labels: ['__journal_syslog_identifier']
  - source_labels: ['__journal_syslog_identifier']
    target_label: unit
  - source_labels: ['__journal__hostname']
    target_label: nodename
- job_name: combine-journal
  journal:
    json: false
    labels:
      job: systemd-combine-journal
      origin: systemd-journal
    max_age: 12h
  relabel_configs:
  - action: drop
    regex: ^localhost$
    source_labels: ['__journal__hostname']
  - action: replace
    regex: '(.+)'
    replacement: $1
    source_labels: ['__journal__systemd_unit']
    target_label: '__journal_syslog_identifier'
  - action: drop
    regex: ^kernel|kubelet\.service|docker\.service|containerd\.service|gardener-node-agent\.service$
    source_labels: ['__journal_syslog_identifier']
  - source_labels: ['__journal_syslog_identifier']
    target_label: unit
  - source_labels: ['__journal__hostname']
    target_label: nodename
  pipeline_stages:
  - pack:
     labels:
     - unit
     ingest_timestamp: true
- job_name: kubernetes-pods-name
  pipeline_stages:
  - cri: {}
  - labeldrop:
    - filename
    - stream
    - pod_uid
  kubernetes_sd_configs:
  - role: pod
    api_server: https://api.test-cluster.com
    tls_config:
      server_name: api.test-cluster.com
      ca_file: /var/lib/valitail/ca.crt
    bearer_token_file: /var/lib/valitail/auth-token
    namespaces:
      names: ['kube-system']
  relabel_configs:
  - action: drop
    regex: ''
    separator: ''
    source_labels:
    - __meta_kubernetes_pod_label_gardener_cloud_role
    - __meta_kubernetes_pod_label_origin
    - __meta_kubernetes_pod_label_resources_gardener_cloud_managed_by
  - action: replace
    regex: '.+'
    replacement: "gardener"
    source_labels: ['__meta_kubernetes_pod_label_gardener_cloud_role']
    target_label: __meta_kubernetes_pod_label_origin
  - action: replace
    regex: 'gardener'
    replacement: "gardener"
    source_labels: ['__meta_kubernetes_pod_label_resources_gardener_cloud_managed_by']
    target_label: __meta_kubernetes_pod_label_origin
  - action: keep
    regex: 'gardener'
    source_labels: ['__meta_kubernetes_pod_label_origin']
  - action: replace
    regex: ''
    replacement: 'default'
    source_labels: ['__meta_kubernetes_pod_label_gardener_cloud_role']
    target_label: __meta_kubernetes_pod_label_gardener_cloud_role
  - source_labels: ['__meta_kubernetes_pod_node_name']
    target_label: '__host__'
  - source_labels: ['__meta_kubernetes_pod_node_name']
    target_label: 'nodename'
  - action: replace
    source_labels: ['__meta_kubernetes_namespace']
    target_label: namespace_name
  - action: replace
    source_labels: ['__meta_kubernetes_pod_name']
    target_label: pod_name
  - action: replace
    source_labels: ['__meta_kubernetes_pod_uid']
    target_label: pod_uid
  - action: replace
    source_labels: ['__meta_kubernetes_pod_container_name']
    target_label: container_name
  - replacement: /var/log/pods/*$1/*.log
    separator: /
    source_labels:
    - __meta_kubernetes_pod_uid
    - __meta_kubernetes_pod_container_name
    target_label: __path__
  - source_labels: ['__meta_kubernetes_pod_label_gardener_cloud_role']
    target_label: gardener_cloud_role
  - source_labels: ['__meta_kubernetes_pod_label_origin']
    replacement: 'shoot_system'
    target_label: origin
`)),
							},
						},
					}

					valitailBinaryFile := extensionsv1alpha1.File{
						Path:        "/opt/bin/valitail",
						Permissions: ptr.To(int32(0755)),
						Content: extensionsv1alpha1.FileContent{
							ImageRef: &extensionsv1alpha1.FileContentImageRef{
								Image:           ctx.Images["valitail"].String(),
								FilePathInImage: "/usr/bin/valitail",
							},
						},
					}

					valitailFetchTokenScriptFile := extensionsv1alpha1.File{
						Path:        "/var/lib/valitail/scripts/fetch-token.sh",
						Permissions: ptr.To(int32(0744)),
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
  "$(cat "/var/lib/cloud-config-downloader/credentials/server")/api/v1/namespaces/kube-system/secrets/gardener-valitail")"; then

  echo "Could not retrieve the valitail token secret"
  exit 1
fi

echo "$SECRET" | sed -rn "s/  token: (.*)/\1/p" | base64 -d > "/var/lib/valitail/auth-token"

exit $?
}
`)),
							},
						},
					}

					caBundleFile := extensionsv1alpha1.File{
						Path:        "/var/lib/valitail/ca.crt",
						Permissions: ptr.To(int32(0644)),
						Content: extensionsv1alpha1.FileContent{
							Inline: &extensionsv1alpha1.FileContentInline{
								Encoding: "b64",
								Data:     utils.EncodeBase64([]byte(cABundle)),
							},
						},
					}

					expectedFiles := []extensionsv1alpha1.File{valitailConfigFile, caBundleFile}
					if !useGardenerNodeAgentEnabled {
						expectedFiles = append(expectedFiles, valitailFetchTokenScriptFile)
						valitailTokenFetchUnit.FilePaths = []string{"/var/lib/valitail/scripts/fetch-token.sh"}
					}

					valitailDaemonUnit.FilePaths = []string{
						"/var/lib/valitail/config/config",
						"/var/lib/valitail/ca.crt",
					}

					if useGardenerNodeAgentEnabled {
						expectedFiles = append(expectedFiles, valitailBinaryFile)
						valitailDaemonUnit.FilePaths = append(valitailDaemonUnit.FilePaths, "/opt/bin/valitail")
					}

					expectedUnits := []extensionsv1alpha1.Unit{valitailDaemonUnit}
					if !useGardenerNodeAgentEnabled {
						expectedUnits = append(expectedUnits, valitailTokenFetchUnit)
					}

					Expect(units).To(ConsistOf(expectedUnits))
					Expect(files).To(ConsistOf(expectedFiles))
				})

				It("should return the expected units and files when shoot logging is not enabled", func() {
					defer test.WithFeatureGate(features.DefaultFeatureGate, features.UseGardenerNodeAgent, useGardenerNodeAgentEnabled)()
					ctx := components.Context{
						CABundle:      &cABundle,
						ClusterDomain: clusterDomain,
						Images: map[string]*imagevector.Image{
							"valitail": valitailImage,
						},
						ValiIngress:     valiIngress,
						ValitailEnabled: false,
					}

					units, files, err := New().Config(ctx)
					Expect(err).NotTo(HaveOccurred())

					if useGardenerNodeAgentEnabled {
						Expect(units).To(BeEmpty())
						Expect(files).To(BeEmpty())
						return
					}

					afterUnit := "cloud-config-downloader.service"
					if useGardenerNodeAgentEnabled {
						afterUnit = "gardener-node-agent.service"
					}

					unitContent := `[Unit]
Description=valitail daemon
Documentation=https://github.com/credativ/plutono`

					if !useGardenerNodeAgentEnabled {
						unitContent += `
After=valitail-fetch-token.service`
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
ExecStartPre=/bin/sh -c "systemctl set-environment HOSTNAME=$(hostname | tr [:upper:] [:lower:])"
ExecStartPre=/bin/systemctl disable valitail.service
ExecStart=/bin/sh -c "echo service valitail.service is removed!; while true; do sleep 86400; done"`

					Expect(units).To(ConsistOf([]extensionsv1alpha1.Unit{
						{
							Name:    "valitail.service",
							Command: ptr.To(extensionsv1alpha1.CommandStart),
							Enable:  ptr.To(true),
							Content: ptr.To(unitContent),
						},
						{
							Name:    "valitail-fetch-token.service",
							Command: ptr.To(extensionsv1alpha1.CommandStart),
							Enable:  ptr.To(true),
							Content: ptr.To(`[Unit]
Description=valitail token fetcher
After=` + afterUnit + `
[Install]
WantedBy=multi-user.target
[Service]
Restart=always
RestartSec=300
RuntimeMaxSec=120
EnvironmentFile=/etc/environment
ExecStartPre=/bin/systemctl disable valitail-fetch-token.service
ExecStart=/bin/sh -c "rm -f /var/lib/valitail/auth-token; echo service valitail-fetch-token.service is removed!; while true; do sleep 86400; done"`),
						},
					}))
					Expect(files).To(BeNil())
				})

				It("should return error when vali ingress is not specified", func() {
					defer test.WithFeatureGate(features.DefaultFeatureGate, features.UseGardenerNodeAgent, useGardenerNodeAgentEnabled)()
					ctx := components.Context{
						CABundle:      &cABundle,
						ClusterDomain: clusterDomain,
						Images: map[string]*imagevector.Image{
							"valitail": valitailImage,
						},
						ValitailEnabled: true,
						ValiIngress:     "",
					}

					units, files, err := New().Config(ctx)
					Expect(err).To(MatchError(ContainSubstring("vali ingress url is missing")))
					Expect(units).To(BeNil())
					Expect(files).To(BeNil())
				})
			})
		}
		// Testing with feature gate UseGardenerNodeAgent: false
		testConfig(false)
		// Testing with feature gate UseGardenerNodeAgent: true
		testConfig(true)
	})
})

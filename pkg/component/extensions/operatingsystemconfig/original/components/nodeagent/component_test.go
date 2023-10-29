// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package nodeagent_test

import (
	"github.com/Masterminds/semver/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
	"k8s.io/utils/pointer"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components"
	. "github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/nodeagent"
	nodeagentv1alpha1 "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
	imagevectorutils "github.com/gardener/gardener/pkg/utils/imagevector"
)

var _ = Describe("Component", func() {
	var (
		oscSecretName     = "osc-secret-name"
		kubernetesVersion = semver.MustParse("1.2.3")
		apiServerURL      = "https://localhost"
		caBundle          = "ca-bundle"
	)

	Describe("#Config", func() {
		var component components.Component

		BeforeEach(func() {
			component = New()
		})

		It("should return the expected units and files", func() {
			key := "key"

			unitFiles, err := Files(ComponentConfig(key, kubernetesVersion), apiServerURL, caBundle, "/var/lib/gardener-node-agent/credentials/token")
			Expect(err).NotTo(HaveOccurred())

			units, files, err := component.Config(components.Context{
				Key:               key,
				KubernetesVersion: kubernetesVersion,
				APIServerURL:      apiServerURL,
				CABundle:          &caBundle,
				Images:            map[string]*imagevectorutils.Image{"gardener-node-agent": {Repository: "gardener-node-agent", Tag: pointer.String("v1")}},
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(units).To(ConsistOf(
				extensionsv1alpha1.Unit{
					Name:   "gardener-node-agent.service",
					Enable: pointer.Bool(true),
					Content: pointer.String(`[Unit]
Description=Gardener Node Agent
After=network.target

[Service]
LimitMEMLOCK=infinity
ExecStart=/opt/bin/gardener-node-agent --config=/var/lib/gardener-node-agent/config.yaml
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target`),
					Files: append(unitFiles, extensionsv1alpha1.File{
						Path: "/opt/bin/gardener-node-agent",
						Content: extensionsv1alpha1.FileContent{
							ImageRef: &extensionsv1alpha1.FileContentImageRef{
								Image:           "gardener-node-agent:v1",
								FilePathInImage: "/gardener-node-agent",
							},
						},
					}),
				},
			))
			Expect(files).To(BeEmpty())
		})
	})

	Describe("#UnitContent", func() {
		It("should return the expected result", func() {
			Expect(UnitContent()).To(Equal(`[Unit]
Description=Gardener Node Agent
After=network.target

[Service]
LimitMEMLOCK=infinity
ExecStart=/opt/bin/gardener-node-agent --config=/var/lib/gardener-node-agent/config.yaml
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target`))
		})
	})

	Describe("#ComponentConfig", func() {
		It("should return the expected result", func() {
			Expect(ComponentConfig(oscSecretName, kubernetesVersion)).To(Equal(&nodeagentv1alpha1.NodeAgentConfiguration{
				ClientConnection: componentbaseconfigv1alpha1.ClientConnectionConfiguration{
					Kubeconfig: "/var/lib/gardener-node-agent/credentials/kubeconfig",
				},
				Controllers: nodeagentv1alpha1.ControllerConfiguration{
					OperatingSystemConfig: nodeagentv1alpha1.OperatingSystemConfigControllerConfig{
						SecretName:        oscSecretName,
						KubernetesVersion: kubernetesVersion,
					},
					Token: nodeagentv1alpha1.TokenControllerConfig{
						SecretName: "gardener-node-agent",
					},
				},
			}))
		})
	})

	Describe("#Files", func() {
		It("should return the expected files", func() {
			var (
				config    = ComponentConfig(oscSecretName, nil)
				tokenFile = "token-file"
			)

			Expect(Files(config, apiServerURL, caBundle, tokenFile)).To(ConsistOf(
				extensionsv1alpha1.File{
					Path:        "/var/lib/gardener-node-agent/config.yaml",
					Permissions: pointer.Int32(0644),
					Content: extensionsv1alpha1.FileContent{Inline: &extensionsv1alpha1.FileContentInline{Encoding: "b64", Data: utils.EncodeBase64([]byte(`apiVersion: nodeagent.config.gardener.cloud/v1alpha1
clientConnection:
  acceptContentTypes: ""
  burst: 0
  contentType: ""
  kubeconfig: /var/lib/gardener-node-agent/credentials/kubeconfig
  qps: 0
controllers:
  operatingSystemConfig:
    kubernetesVersion: null
    secretName: ` + oscSecretName + `
  token:
    secretName: gardener-node-agent
kind: NodeAgentConfiguration
logFormat: ""
logLevel: ""
server: {}
`))}},
				},
				extensionsv1alpha1.File{
					Path:        "/var/lib/gardener-node-agent/credentials/kubeconfig",
					Permissions: pointer.Int32(0644),
					Content: extensionsv1alpha1.FileContent{Inline: &extensionsv1alpha1.FileContentInline{Encoding: "b64", Data: utils.EncodeBase64([]byte(`apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: ` + utils.EncodeBase64([]byte(caBundle)) + `
    server: ` + apiServerURL + `
  name: gardener-node-agent
contexts:
- context:
    cluster: gardener-node-agent
    user: gardener-node-agent
  name: gardener-node-agent
current-context: gardener-node-agent
kind: Config
preferences: {}
users:
- name: gardener-node-agent
  user:
    tokenFile: ` + tokenFile + `
`))}},
				},
			))
		})
	})
})

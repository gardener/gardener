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
	"time"

	"github.com/Masterminds/semver/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"k8s.io/utils/ptr"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components"
	. "github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/original/components/nodeagent"
	nodeagentv1alpha1 "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
	imagevectorutils "github.com/gardener/gardener/pkg/utils/imagevector"
)

var _ = Describe("Component", func() {
	var (
		oscSecretName              = "osc-secret-name"
		kubernetesVersion          = semver.MustParse("1.2.3")
		apiServerURL               = "https://localhost"
		caBundle                   = []byte("ca-bundle")
		syncJitterPeriod           = &metav1.Duration{Duration: time.Second}
		additionalTokenSyncConfigs = []nodeagentv1alpha1.TokenSecretSyncConfig{{
			SecretName: "gardener-valitail",
			Path:       "/var/lib/valitail/auth-token",
		}}
	)

	Describe("#Config", func() {
		var component components.Component

		BeforeEach(func() {
			component = New()
		})

		It("should return the expected units and files", func() {
			key := "key"

			expectedFiles, err := Files(ComponentConfig(key, kubernetesVersion, apiServerURL, caBundle, syncJitterPeriod, nil))
			Expect(err).NotTo(HaveOccurred())

			units, files, err := component.Config(components.Context{
				Key:                 key,
				KubernetesVersion:   kubernetesVersion,
				APIServerURL:        apiServerURL,
				CABundle:            pointer.String(string(caBundle)),
				Images:              map[string]*imagevectorutils.Image{"gardener-node-agent": {Repository: "gardener-node-agent", Tag: pointer.String("v1")}},
				OSCSyncJitterPeriod: syncJitterPeriod,
			})

			expectedFiles = append(expectedFiles, extensionsv1alpha1.File{
				Path:        "/opt/bin/gardener-node-agent",
				Permissions: pointer.Int32(0755),
				Content: extensionsv1alpha1.FileContent{
					ImageRef: &extensionsv1alpha1.FileContentImageRef{
						Image:           "gardener-node-agent:v1",
						FilePathInImage: "/gardener-node-agent",
					},
				},
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(units).To(ConsistOf(
				extensionsv1alpha1.Unit{
					Name:   "gardener-node-agent.service",
					Enable: ptr.To(true),
					Content: pointer.String(`[Unit]
Description=Gardener Node Agent
After=network-online.target

[Service]
LimitMEMLOCK=infinity
ExecStart=/opt/bin/gardener-node-agent --config=/var/lib/gardener-node-agent/config.yaml
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target`),
					FilePaths: []string{"/var/lib/gardener-node-agent/config.yaml", "/opt/bin/gardener-node-agent"},
				},
			))
			Expect(files).To(ConsistOf(expectedFiles))
		})
	})

	Describe("#UnitContent", func() {
		It("should return the expected result", func() {
			Expect(UnitContent()).To(Equal(`[Unit]
Description=Gardener Node Agent
After=network-online.target

[Service]
LimitMEMLOCK=infinity
ExecStart=/opt/bin/gardener-node-agent --config=/var/lib/gardener-node-agent/config.yaml
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target`))
		})
	})

	Describe("#ComponentConfig", func() {
		It("should return the expected result", func() {
			Expect(ComponentConfig(oscSecretName, kubernetesVersion, apiServerURL, caBundle, syncJitterPeriod, additionalTokenSyncConfigs)).To(Equal(&nodeagentv1alpha1.NodeAgentConfiguration{
				APIServer: nodeagentv1alpha1.APIServer{
					Server:   apiServerURL,
					CABundle: caBundle,
				},
				Controllers: nodeagentv1alpha1.ControllerConfiguration{
					OperatingSystemConfig: nodeagentv1alpha1.OperatingSystemConfigControllerConfig{
						SecretName:        oscSecretName,
						KubernetesVersion: kubernetesVersion,
						SyncJitterPeriod:  syncJitterPeriod,
					},
					Token: nodeagentv1alpha1.TokenControllerConfig{
						SyncConfigs: []nodeagentv1alpha1.TokenSecretSyncConfig{
							{
								SecretName: "gardener-node-agent",
								Path:       "/var/lib/gardener-node-agent/credentials/token",
							},
							{
								SecretName: "gardener-valitail",
								Path:       "/var/lib/valitail/auth-token",
							},
						},
						SyncPeriod: &metav1.Duration{Duration: 12 * time.Hour},
					},
				},
			}))
		})
	})

	Describe("#Files", func() {
		It("should return the expected files", func() {
			config := ComponentConfig(oscSecretName, nil, apiServerURL, caBundle, syncJitterPeriod, additionalTokenSyncConfigs)

			Expect(Files(config)).To(ConsistOf(extensionsv1alpha1.File{
				Path:        "/var/lib/gardener-node-agent/config.yaml",
				Permissions: pointer.Int32(0600),
				Content: extensionsv1alpha1.FileContent{Inline: &extensionsv1alpha1.FileContentInline{Encoding: "b64", Data: utils.EncodeBase64([]byte(`apiServer:
  caBundle: ` + utils.EncodeBase64(caBundle) + `
  server: ` + apiServerURL + `
apiVersion: nodeagent.config.gardener.cloud/v1alpha1
clientConnection:
  acceptContentTypes: ""
  burst: 0
  contentType: ""
  kubeconfig: ""
  qps: 0
controllers:
  operatingSystemConfig:
    kubernetesVersion: null
    secretName: ` + oscSecretName + `
    syncJitterPeriod: ` + syncJitterPeriod.Duration.String() + `
  token:
    syncConfigs:
    - path: /var/lib/gardener-node-agent/credentials/token
      secretName: gardener-node-agent
    - path: /var/lib/valitail/auth-token
      secretName: gardener-valitail
    syncPeriod: 12h0m0s
kind: NodeAgentConfiguration
logFormat: ""
logLevel: ""
server: {}
`))}},
			}))
		})
	})
})

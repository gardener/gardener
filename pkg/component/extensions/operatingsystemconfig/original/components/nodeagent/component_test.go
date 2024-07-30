// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package nodeagent_test

import (
	"time"

	"github.com/Masterminds/semver/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

			expectedFiles, err := Files(ComponentConfig(key, kubernetesVersion, apiServerURL, caBundle, nil))
			Expect(err).NotTo(HaveOccurred())

			units, files, err := component.Config(components.Context{
				Key:               key,
				KubernetesVersion: kubernetesVersion,
				APIServerURL:      apiServerURL,
				CABundle:          ptr.To(string(caBundle)),
				Images:            map[string]*imagevectorutils.Image{"gardener-node-agent": {Repository: ptr.To("gardener-node-agent"), Tag: ptr.To("v1")}},
			})

			expectedFiles = append(expectedFiles, extensionsv1alpha1.File{
				Path:        "/opt/bin/gardener-node-agent",
				Permissions: ptr.To[int32](0755),
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
					Content: ptr.To(`[Unit]
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
			Expect(ComponentConfig(oscSecretName, kubernetesVersion, apiServerURL, caBundle, additionalTokenSyncConfigs)).To(Equal(&nodeagentv1alpha1.NodeAgentConfiguration{
				APIServer: nodeagentv1alpha1.APIServer{
					Server:   apiServerURL,
					CABundle: caBundle,
				},
				Controllers: nodeagentv1alpha1.ControllerConfiguration{
					OperatingSystemConfig: nodeagentv1alpha1.OperatingSystemConfigControllerConfig{
						SecretName:        oscSecretName,
						KubernetesVersion: kubernetesVersion,
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
			config := ComponentConfig(oscSecretName, nil, apiServerURL, caBundle, additionalTokenSyncConfigs)

			Expect(Files(config)).To(ConsistOf(extensionsv1alpha1.File{
				Path:        "/var/lib/gardener-node-agent/config.yaml",
				Permissions: ptr.To[int32](0600),
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

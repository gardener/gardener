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
	"github.com/gardener/gardener/pkg/features"
	nodeagentconfigv1alpha1 "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
	imagevectorutils "github.com/gardener/gardener/pkg/utils/imagevector"
	"github.com/gardener/gardener/pkg/utils/test"
)

var _ = Describe("Component", func() {
	var (
		oscSecretName              = "osc-secret-name"
		kubernetesVersion          = semver.MustParse("1.2.3")
		apiServerURL               = "https://localhost"
		caBundle                   = []byte("ca-bundle")
		additionalTokenSyncConfigs = []nodeagentconfigv1alpha1.TokenSecretSyncConfig{{
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
				CABundle:          string(caBundle),
				Images:            map[string]*imagevectorutils.Image{"gardener-node-agent": {Repository: ptr.To("gardener-node-agent"), Tag: ptr.To("v1")}},
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
			Expect(files).To(ConsistOf(append(expectedFiles, extensionsv1alpha1.File{
				Path:        "/opt/bin/gardener-node-agent",
				Permissions: ptr.To[uint32](0755),
				Content: extensionsv1alpha1.FileContent{
					ImageRef: &extensionsv1alpha1.FileContentImageRef{
						Image:           "gardener-node-agent:v1",
						FilePathInImage: "/gardener-node-agent",
					},
				},
			})))
		})

		It("should return the expected binary prefix when image is from local registry", func() {
			_, files, err := component.Config(components.Context{
				Images: map[string]*imagevectorutils.Image{"gardener-node-agent": {Repository: ptr.To("garden.local.gardener.cloud:5001/gardener-node-agent"), Tag: ptr.To("v1")}},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(files).To(ContainElement(extensionsv1alpha1.File{
				Path:        "/opt/bin/gardener-node-agent",
				Permissions: ptr.To[uint32](0755),
				Content: extensionsv1alpha1.FileContent{
					ImageRef: &extensionsv1alpha1.FileContentImageRef{
						Image:           "garden.local.gardener.cloud:5001/gardener-node-agent:v1",
						FilePathInImage: "/ko-app/gardener-node-agent",
					},
				},
			}))
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
		It("should return the expected result when NodeAgentAuthorizer feature gate is disabled", func() {
			DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.NodeAgentAuthorizer, false))
			Expect(ComponentConfig(oscSecretName, kubernetesVersion, apiServerURL, caBundle, additionalTokenSyncConfigs)).To(Equal(&nodeagentconfigv1alpha1.NodeAgentConfiguration{
				APIServer: nodeagentconfigv1alpha1.APIServer{
					Server:   apiServerURL,
					CABundle: caBundle,
				},
				Controllers: nodeagentconfigv1alpha1.ControllerConfiguration{
					OperatingSystemConfig: nodeagentconfigv1alpha1.OperatingSystemConfigControllerConfig{
						SecretName:        oscSecretName,
						KubernetesVersion: kubernetesVersion,
					},
					Token: nodeagentconfigv1alpha1.TokenControllerConfig{
						SyncConfigs: []nodeagentconfigv1alpha1.TokenSecretSyncConfig{
							{
								SecretName: "gardener-valitail",
								Path:       "/var/lib/valitail/auth-token",
							},
							{
								SecretName: "gardener-node-agent",
								Path:       "/var/lib/gardener-node-agent/credentials/token",
							},
						},
						SyncPeriod: &metav1.Duration{Duration: 12 * time.Hour},
					},
				},
				FeatureGates: map[string]bool{string(features.NodeAgentAuthorizer): false},
			}))
		})

		It("should return the expected result", func() {
			Expect(ComponentConfig(oscSecretName, kubernetesVersion, apiServerURL, caBundle, additionalTokenSyncConfigs)).To(Equal(&nodeagentconfigv1alpha1.NodeAgentConfiguration{
				APIServer: nodeagentconfigv1alpha1.APIServer{
					Server:   apiServerURL,
					CABundle: caBundle,
				},
				Controllers: nodeagentconfigv1alpha1.ControllerConfiguration{
					OperatingSystemConfig: nodeagentconfigv1alpha1.OperatingSystemConfigControllerConfig{
						SecretName:        oscSecretName,
						KubernetesVersion: kubernetesVersion,
					},
					Token: nodeagentconfigv1alpha1.TokenControllerConfig{
						SyncConfigs: []nodeagentconfigv1alpha1.TokenSecretSyncConfig{
							{
								SecretName: "gardener-valitail",
								Path:       "/var/lib/valitail/auth-token",
							},
						},
						SyncPeriod: &metav1.Duration{Duration: 12 * time.Hour},
					},
				},
				FeatureGates: map[string]bool{string(features.NodeAgentAuthorizer): true},
			}))
		})
	})

	Describe("#Files", func() {
		It("should return the expected files when NodeAgentAuthorizer feature gate is disabled ", func() {
			DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.NodeAgentAuthorizer, false))
			config := ComponentConfig(oscSecretName, nil, apiServerURL, caBundle, additionalTokenSyncConfigs)

			Expect(Files(config)).To(ConsistOf(extensionsv1alpha1.File{
				Path:        "/var/lib/gardener-node-agent/config.yaml",
				Permissions: ptr.To[uint32](0600),
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
    - path: /var/lib/valitail/auth-token
      secretName: gardener-valitail
    - path: /var/lib/gardener-node-agent/credentials/token
      secretName: gardener-node-agent
    syncPeriod: 12h0m0s
featureGates:
  NodeAgentAuthorizer: false
kind: NodeAgentConfiguration
logFormat: ""
logLevel: ""
server: {}
`))}},
			}))
		})

		It("should return the expected files", func() {
			config := ComponentConfig(oscSecretName, nil, apiServerURL, caBundle, additionalTokenSyncConfigs)

			Expect(Files(config)).To(ConsistOf(extensionsv1alpha1.File{
				Path:        "/var/lib/gardener-node-agent/config.yaml",
				Permissions: ptr.To[uint32](0600),
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
    - path: /var/lib/valitail/auth-token
      secretName: gardener-valitail
    syncPeriod: 12h0m0s
featureGates:
  NodeAgentAuthorizer: true
kind: NodeAgentConfiguration
logFormat: ""
logLevel: ""
server: {}
`))}},
			}))
		})
	})

	Describe("#FilePathInImage", func() {
		It("should return the path when registry is not local", func() {
			Expect(FilePathInImage("some-registry/gna:v1.0")).To(Equal("/gardener-node-agent"))
		})

		It("should return the path when registry is local", func() {
			Expect(FilePathInImage("garden.local.gardener.cloud:5001/gna:v1.0")).To(Equal("/ko-app/gardener-node-agent"))
		})
	})
})

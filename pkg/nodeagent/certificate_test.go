// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package nodeagent_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/afero"

	. "github.com/gardener/gardener/pkg/nodeagent"
	nodeagentconfigv1alpha1 "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
)

var _ = Describe("Certificate", func() {
	Describe("#GetAPIServerConfig", func() {
		var (
			fs                  afero.Afero
			nodeAgentConfig     *nodeagentconfigv1alpha1.NodeAgentConfiguration
			nodeAgentConfigFile []byte
		)

		BeforeEach(func() {
			fs = afero.Afero{Fs: afero.NewMemMapFs()}
			nodeAgentConfig = &nodeagentconfigv1alpha1.NodeAgentConfiguration{
				APIServer: nodeagentconfigv1alpha1.APIServer{
					CABundle: []byte("ca-bundle"),
					Server:   "https://test-server",
				},
			}

			nodeAgentConfigFile = []byte(`apiServer:
  caBundle: ` + utils.EncodeBase64(nodeAgentConfig.APIServer.CABundle) + `
  server: ` + nodeAgentConfig.APIServer.Server + `
apiVersion: nodeagent.config.gardener.cloud/v1alpha1
kind: NodeAgentConfiguration
`)
		})

		It("should return an error if the kubeconfig file does not exist", func() {
			config, err := GetAPIServerConfig(fs)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("error reading gardener-node-agent config"))
			Expect(config).To(BeNil())
		})

		It("should return an error if the kubeconfig file is invalid", func() {
			invalidConfigFile := []byte(`apiVersion: nodeagent.config.gardener.cloud/v1alpha1
			kind: Invalid
			`)
			Expect(fs.WriteFile(nodeagentconfigv1alpha1.ConfigFilePath, invalidConfigFile, 0600)).To(Succeed())

			config, err := GetAPIServerConfig(fs)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("error decoding gardener-node-agent config"))
			Expect(config).To(BeNil())
		})

		It("should return the APIServer config", func() {
			Expect(fs.WriteFile(nodeagentconfigv1alpha1.ConfigFilePath, nodeAgentConfigFile, 0600)).To(Succeed())

			config, err := GetAPIServerConfig(fs)
			Expect(err).NotTo(HaveOccurred())
			Expect(config).NotTo(BeNil())
			Expect(config.CABundle).To(Equal(nodeAgentConfig.APIServer.CABundle))
			Expect(config.Server).To(Equal(nodeAgentConfig.APIServer.Server))
		})
	})
})

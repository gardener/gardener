// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package nodeagent_test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/afero"
	"k8s.io/component-base/version"

	nodeagentconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/nodeagent/v1alpha1"
	. "github.com/gardener/gardener/pkg/nodeagent"
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
					CAFile: nodeagentconfigv1alpha1.ClusterCAFilePath,
					Server: "https://test-server",
				},
			}

			nodeAgentConfigFile = []byte(`apiServer:
  caFile: ` + nodeAgentConfig.APIServer.CAFile + `
  server: ` + nodeAgentConfig.APIServer.Server + `
apiVersion: nodeagent.config.gardener.cloud/v1alpha1
kind: NodeAgentConfiguration
`)
		})

		It("should return an error if the kubeconfig file does not exist", func() {
			config, err := GetAPIServerConfig(fs, "/var/lib/gardener-node-agent")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("error reading gardener-node-agent config"))
			Expect(config).To(BeNil())
		})

		It("should return an error if the kubeconfig file is invalid", func() {
			invalidConfigFile := []byte(`apiVersion: nodeagent.config.gardener.cloud/v1alpha1
			kind: Invalid
			`)
			Expect(fs.WriteFile(fmt.Sprintf("/var/lib/gardener-node-agent/config-%s.yaml", version.Get().GitVersion), invalidConfigFile, 0600)).To(Succeed())

			config, err := GetAPIServerConfig(fs, "/var/lib/gardener-node-agent")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("error decoding gardener-node-agent config"))
			Expect(config).To(BeNil())
		})

		It("should return the APIServer config", func() {
			Expect(fs.WriteFile(fmt.Sprintf("/var/lib/gardener-node-agent/config-%s.yaml", version.Get().GitVersion), nodeAgentConfigFile, 0600)).To(Succeed())

			config, err := GetAPIServerConfig(fs, "/var/lib/gardener-node-agent")
			Expect(err).NotTo(HaveOccurred())
			Expect(config).NotTo(BeNil())
			Expect(config.CAFile).To(Equal(nodeAgentConfig.APIServer.CAFile))
			Expect(config.Server).To(Equal(nodeAgentConfig.APIServer.Server))
		})
	})
})

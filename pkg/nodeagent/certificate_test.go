// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package nodeagent_test

import (
	"fmt"
	"os"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/afero"
	"k8s.io/client-go/tools/clientcmd"
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

	Describe("#EnsureServerInKubeconfig", func() {
		var (
			fs         afero.Afero
			kubeconfig []byte
		)

		BeforeEach(func() {
			fs = afero.Afero{Fs: afero.NewMemMapFs()}
			kubeconfig = []byte(`apiVersion: v1
clusters:
- cluster:
    certificate-authority: /var/lib/gardener-node-agent/cluster-ca.crt
    server: https://localhost
  name: node-agent
contexts:
- context:
    cluster: node-agent
    user: node-agent
  name: node-agent
current-context: node-agent
kind: Config
users:
- name: node-agent
  user:
    client-certificate-data: Y2VydA==
    client-key-data: a2V5
`)
		})

		It("should return an error if the kubeconfig file does not exist", func() {
			Expect(EnsureServerInKubeconfig(logr.Discard(), fs, "https://api.internal.foo.bar")).To(MatchError(ContainSubstring("failed reading kubeconfig file")))
		})

		It("should return an error if the kubeconfig file is invalid", func() {
			Expect(fs.WriteFile(nodeagentconfigv1alpha1.KubeconfigFilePath, []byte("invalid"), 0600)).To(Succeed())

			Expect(EnsureServerInKubeconfig(logr.Discard(), fs, "https://api.internal.foo.bar")).To(MatchError(ContainSubstring("failed loading kubeconfig file")))
		})

		It("should update the API server address and keep the credentials", func() {
			Expect(fs.WriteFile(nodeagentconfigv1alpha1.KubeconfigFilePath, kubeconfig, 0600)).To(Succeed())

			Expect(EnsureServerInKubeconfig(logr.Discard(), fs, "https://api.internal.foo.bar")).To(Succeed())

			kubeconfigRaw, err := fs.ReadFile(nodeagentconfigv1alpha1.KubeconfigFilePath)
			Expect(err).NotTo(HaveOccurred())
			updatedKubeconfig, err := clientcmd.Load(kubeconfigRaw)
			Expect(err).NotTo(HaveOccurred())
			Expect(updatedKubeconfig.Clusters).To(HaveKeyWithValue("node-agent", And(
				HaveField("Server", "https://api.internal.foo.bar"),
				HaveField("CertificateAuthority", "/var/lib/gardener-node-agent/cluster-ca.crt"),
			)))
			Expect(updatedKubeconfig.AuthInfos).To(HaveKeyWithValue("node-agent", And(
				HaveField("ClientCertificateData", []byte("cert")),
				HaveField("ClientKeyData", []byte("key")),
			)))
			Expect(updatedKubeconfig.CurrentContext).To(Equal("node-agent"))

			fileInfo, err := fs.Stat(nodeagentconfigv1alpha1.KubeconfigFilePath)
			Expect(err).NotTo(HaveOccurred())
			Expect(fileInfo.Mode().Perm()).To(Equal(os.FileMode(0600)))
		})

		It("should not rewrite the kubeconfig if the API server address is up-to-date", func() {
			Expect(fs.WriteFile(nodeagentconfigv1alpha1.KubeconfigFilePath, kubeconfig, 0600)).To(Succeed())

			Expect(EnsureServerInKubeconfig(logr.Discard(), fs, "https://localhost")).To(Succeed())

			Expect(fs.ReadFile(nodeagentconfigv1alpha1.KubeconfigFilePath)).To(Equal(kubeconfig))
		})
	})
})

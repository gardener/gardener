// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package botanist

import (
	"github.com/Masterminds/semver/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/tools/clientcmd"

	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
)

var _ = Describe("ControlPlane", func() {
	var b *GardenadmBotanist

	BeforeEach(func() {
		b = &GardenadmBotanist{}
	})

	Describe("#kubeconfigWithServer", func() {
		It("should replace the server of all clusters", func() {
			kubeconfig := []byte(`apiVersion: v1
kind: Config
clusters:
- name: foo
  cluster:
    server: https://localhost
    certificate-authority-data: Y2EtZGF0YQ==
- name: bar
  cluster:
    server: https://localhost
contexts:
- name: foo
  context:
    cluster: foo
    user: foo
current-context: foo
users:
- name: foo
  user:
    token: some-token
`)

			result, err := kubeconfigWithServer(kubeconfig, "https://api.foo.bar.com")
			Expect(err).NotTo(HaveOccurred())

			config, err := clientcmd.Load(result)
			Expect(err).NotTo(HaveOccurred())
			Expect(config.Clusters).To(HaveLen(2))
			Expect(config.Clusters["foo"].Server).To(Equal("https://api.foo.bar.com"))
			Expect(config.Clusters["foo"].CertificateAuthorityData).To(Equal([]byte("ca-data")))
			Expect(config.Clusters["bar"].Server).To(Equal("https://api.foo.bar.com"))
			Expect(config.AuthInfos["foo"].Token).To(Equal("some-token"))
			Expect(config.CurrentContext).To(Equal("foo"))
		})

		It("should fail for an invalid kubeconfig", func() {
			_, err := kubeconfigWithServer([]byte("{"), "https://api.foo.bar.com")
			Expect(err).To(MatchError(ContainSubstring("failed loading kubeconfig")))
		})
	})

	Describe("#DiscoverKubernetesVersion", func() {
		It("should succeed discovering the version", func() {
			version, err := b.DiscoverKubernetesVersion(fakekubernetes.NewClientSetBuilder().WithVersion("1.33.0").Build())
			Expect(err).NotTo(HaveOccurred())
			Expect(version).To(Equal(semver.MustParse("1.33.0")))
		})

		It("should fail parsing the kubernetes version", func() {
			version, err := b.DiscoverKubernetesVersion(fakekubernetes.NewClientSetBuilder().WithVersion("cannot-parse").Build())
			Expect(err).To(MatchError(ContainSubstring("failed parsing semver version")))
			Expect(version).To(BeNil())
		})
	})
})

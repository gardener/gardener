// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bootstrappers_test

import (
	"context"
	"path/filepath"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/afero"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	nodeagentconfigv1alpha1 "github.com/gardener/gardener/pkg/nodeagent/apis/config/v1alpha1"
	. "github.com/gardener/gardener/pkg/nodeagent/bootstrappers"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/test"
)

var _ = Describe("KubeletBootstrapKubeconfig", func() {
	var (
		ctx = context.TODO()
		log = logr.Discard()

		fakeFS          afero.Afero
		apiServerConfig = nodeagentconfigv1alpha1.APIServer{
			Server:   "server",
			CABundle: []byte("ca-bundle"),
		}
		bootstrapToken = "bootstrap-token"
		runnable       manager.Runnable
	)

	BeforeEach(func() {
		fakeFS = afero.Afero{Fs: afero.NewMemMapFs()}
		runnable = &KubeletBootstrapKubeconfig{
			Log:             log,
			FS:              fakeFS,
			APIServerConfig: apiServerConfig,
		}
	})

	Describe("#Start", func() {
		var (
			pathBootstrapTokenFile              = filepath.Join("/", "var", "lib", "gardener-node-agent", "credentials", "bootstrap-token")
			pathKubeletDirectory                = filepath.Join("/", "var", "lib", "kubelet")
			pathKubeletBootstrapKubeconfigFile  = filepath.Join(pathKubeletDirectory, "kubeconfig-bootstrap")
			pathKubeletClientCertKubeconfigFile = filepath.Join(pathKubeletDirectory, "kubeconfig-real")
			pathKubeletClientCertFile           = filepath.Join(pathKubeletDirectory, "pki", "kubelet-client-current.pem")
		)

		When("bootstrap token file does not exist", func() {
			It("should do nothing when bootstrap token file does not exist", func() {
				Expect(runnable.Start(ctx)).To(Succeed())

				test.AssertNoDirectoryOnDisk(fakeFS, pathKubeletDirectory)
				test.AssertNoFileOnDisk(fakeFS, pathKubeletBootstrapKubeconfigFile)
			})
		})

		When("bootstrap token file exists", func() {
			BeforeEach(func() {
				Expect(fakeFS.WriteFile(pathBootstrapTokenFile, []byte(bootstrapToken), 0600)).To(Succeed())
			})

			It("should do nothing when kubelet kubeconfig with client certificate already exists", func() {
				_, err := fakeFS.Create(pathKubeletClientCertKubeconfigFile)
				Expect(err).NotTo(HaveOccurred())

				Expect(runnable.Start(ctx)).To(Succeed())

				test.AssertDirectoryOnDisk(fakeFS, pathKubeletDirectory)
				test.AssertNoFileOnDisk(fakeFS, pathKubeletBootstrapKubeconfigFile)
			})

			It("should do nothing when kubelet client certificate file already exists", func() {
				_, err := fakeFS.Create(pathKubeletClientCertFile)
				Expect(err).NotTo(HaveOccurred())

				Expect(runnable.Start(ctx)).To(Succeed())

				test.AssertDirectoryOnDisk(fakeFS, pathKubeletDirectory)
				test.AssertNoFileOnDisk(fakeFS, pathKubeletBootstrapKubeconfigFile)
			})

			It("should create the bootstrap kubeconfig file", func() {
				expectedBootstrapKubeconfig := `apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: ` + utils.EncodeBase64(apiServerConfig.CABundle) + `
    server: https://` + apiServerConfig.Server + `
  name: kubelet-bootstrap
contexts:
- context:
    cluster: kubelet-bootstrap
    user: kubelet-bootstrap
  name: kubelet-bootstrap
current-context: kubelet-bootstrap
kind: Config
preferences: {}
users:
- name: kubelet-bootstrap
  user:
    token: ` + bootstrapToken + `
`

				Expect(runnable.Start(ctx)).To(Succeed())

				test.AssertDirectoryOnDisk(fakeFS, pathKubeletDirectory)
				test.AssertFileOnDisk(fakeFS, pathKubeletBootstrapKubeconfigFile, expectedBootstrapKubeconfig, 0600)
			})
		})
	})
})

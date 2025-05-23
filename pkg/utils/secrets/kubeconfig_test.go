// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package secrets_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"
	clientcmdlatest "k8s.io/client-go/tools/clientcmd/api/latest"
	clientcmdv1 "k8s.io/client-go/tools/clientcmd/api/v1"

	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	. "github.com/gardener/gardener/pkg/utils/secrets"
)

var _ = Describe("Kubeconfig Secrets", func() {
	var (
		name        = "bundle"
		contextName = "shoot--foo--bar"
		cluster     = clientcmdv1.Cluster{Server: "server", CertificateAuthority: "/some/path"}
		authInfo    = clientcmdv1.AuthInfo{Token: "token"}
		namespace   = "default"
	)

	Describe("Configuration", func() {
		var config *KubeconfigSecretConfig

		BeforeEach(func() {
			config = &KubeconfigSecretConfig{
				Name:        name,
				ContextName: contextName,
				Cluster:     cluster,
				AuthInfo:    authInfo,
				Namespace:   namespace,
			}
		})

		Describe("#GetName", func() {
			It("should return the name", func() {
				Expect(config.GetName()).To(Equal(name))
			})
		})

		Describe("#Generate", func() {
			It("should generate the kubeconfig", func() {
				obj, err := config.Generate()
				Expect(err).NotTo(HaveOccurred())

				kubeconfig, ok := obj.(*Kubeconfig)
				Expect(ok).To(BeTrue())

				expected := kubernetesutils.NewKubeconfig(contextName, cluster, authInfo)
				expected.Contexts[0].Context.Namespace = namespace

				Expect(kubeconfig.Name).To(Equal(name))
				Expect(kubeconfig.Kubeconfig).To(Equal(expected))
			})
		})

		Describe("#SecretData", func() {
			It("should return the correct data map", func() {
				obj, err := config.Generate()
				Expect(err).NotTo(HaveOccurred())

				kubeconfig, ok := obj.(*Kubeconfig)
				Expect(ok).To(BeTrue())

				raw, err := runtime.Encode(clientcmdlatest.Codec, kubeconfig.Kubeconfig)
				Expect(err).NotTo(HaveOccurred())

				Expect(kubeconfig.SecretData()).To(Equal(map[string][]byte{"kubeconfig": raw}))
			})
		})
	})
})

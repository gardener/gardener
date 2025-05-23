// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package access_test

import (
	"bytes"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/afero"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	clientcmdlatest "k8s.io/client-go/tools/clientcmd/api/latest"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

var _ = Describe("Access controller tests", func() {
	var config *clientcmdapi.Config

	BeforeEach(func() {
		config = clientcmdapi.NewConfig()
		config.Kind = "Config"
		config.APIVersion = "v1"
		config.Clusters["garden"] = &clientcmdapi.Cluster{
			Server: "https://api.gardener.test",
		}
		config.Contexts["garden"] = &clientcmdapi.Context{
			Cluster:  "garden",
			AuthInfo: "garden",
		}
		config.CurrentContext = "garden"
	})

	Context("when unrelated secret is created", func() {
		BeforeEach(func() {
			addTokenToConfig(config, "Z2FyZGVuZXIK")
			kubeConfigData, err := marshalConfig(config)
			Expect(err).NotTo(HaveOccurred())

			secret := testSecret.DeepCopy()
			secret.Name = testSecret.Name + "-unrelated"
			secret.Data = map[string][]byte{
				"kubeconfig": kubeConfigData,
			}
			metav1.SetMetaDataAnnotation(&secret.ObjectMeta, "serviceaccount.resources.gardener.cloud/token-renew-timestamp", "")
			Expect(mgrClient.Create(ctx, secret)).To(Succeed())

			DeferCleanup(func() {
				Expect(mgrClient.Delete(ctx, secret)).To(Succeed())
			})
		})

		It("should not create a token file when unrelated secret is created", func() {
			Consistently(func(g Gomega) {
				exists, err := afero.Exists(fs, tokenFilePath)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(exists).To(BeFalse())
			}).Should(Succeed())
		})
	})

	Context("when secret is created", func() {
		BeforeEach(func() {
			Expect(mgrClient.Create(ctx, testSecret)).To(Succeed())

			DeferCleanup(func() {
				Expect(mgrClient.Delete(ctx, testSecret)).To(Succeed())
			})
		})

		It("should eventually create a token file when secret is created", func() {
			Consistently(func(g Gomega) {
				exists, err := afero.Exists(fs, tokenFilePath)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(exists).To(BeFalse())
			}).Should(Succeed())

			By("Add generic kubeconfig")
			kubeConfigData, err := marshalConfig(config)
			Expect(err).NotTo(HaveOccurred())
			testSecret.Data = map[string][]byte{
				"kubeconfig": kubeConfigData,
			}
			Expect(mgrClient.Update(ctx, testSecret)).To(Succeed())

			Consistently(func(g Gomega) {
				exists, err := afero.Exists(fs, tokenFilePath)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(exists).To(BeFalse())
			}).Should(Succeed())

			By("Add token renew annotation")
			metav1.SetMetaDataAnnotation(&testSecret.ObjectMeta, "serviceaccount.resources.gardener.cloud/token-renew-timestamp", "")
			Expect(mgrClient.Update(ctx, testSecret)).To(Succeed())

			Consistently(func(g Gomega) {
				exists, err := afero.Exists(fs, tokenFilePath)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(exists).To(BeFalse())
			}).Should(Succeed())

			By("Fill kubeconfig with token")
			addTokenToConfig(config, "Z2FyZGVuZXIK")
			kubeConfigData, err = marshalConfig(config)
			Expect(err).NotTo(HaveOccurred())
			testSecret.Data = map[string][]byte{
				"kubeconfig": kubeConfigData,
			}
			Expect(mgrClient.Update(ctx, testSecret)).To(Succeed())

			var event event.TypedGenericEvent[*rest.Config]
			Eventually(func(g Gomega) {
				token, err := afero.ReadFile(fs, tokenFilePath)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(string(token)).To(Equal("Z2FyZGVuZXIK"))
				g.Expect(channel).To(Receive(&event))
			}).Should(Succeed())
			Expect(event.Object).To(Equal(restConfigWithTokenPath(kubeConfigData, tokenFilePath)))

			By("Update token in kubeconfig")
			addTokenToConfig(config, "Ym90YW5pc3QK")
			kubeConfigData, err = marshalConfig(config)
			Expect(err).NotTo(HaveOccurred())
			testSecret.Data["kubeconfig"] = kubeConfigData
			Expect(mgrClient.Update(ctx, testSecret)).To(Succeed())

			Eventually(func(g Gomega) {
				token, err := afero.ReadFile(fs, tokenFilePath)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(string(token)).To(Equal("Ym90YW5pc3QK"))
				g.Expect(channel).To(Receive(&event))
			}).Should(Succeed())
			Expect(event.Object).To(Equal(restConfigWithTokenPath(kubeConfigData, tokenFilePath)))
		})
	})
})

func addTokenToConfig(config *clientcmdapi.Config, token string) {
	config.AuthInfos["garden"] = &clientcmdapi.AuthInfo{Token: token}
}

func marshalConfig(config *clientcmdapi.Config) ([]byte, error) {
	buf := &bytes.Buffer{}
	err := clientcmdlatest.Codec.Encode(config, buf)
	return buf.Bytes(), err
}

func restConfigWithTokenPath(kubeConfigRaw []byte, tokenPath string) *rest.Config {
	restConfig, err := clientcmd.RESTConfigFromKubeConfig(kubeConfigRaw)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	restConfig.BearerToken = ""
	restConfig.BearerTokenFile = tokenPath
	return restConfig
}

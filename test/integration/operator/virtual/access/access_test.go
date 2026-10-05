// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package access_test

import (
	"bytes"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/afero"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	clientcmdlatest "k8s.io/client-go/tools/clientcmd/api/latest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
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
		addTokenToConfig(config, "")
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
		var (
			ev     event.TypedGenericEvent[*rest.Config]
			secret *corev1.Secret
		)

		BeforeEach(func() {
			ev = event.TypedGenericEvent[*rest.Config]{}
			secret = testSecret.DeepCopy()
			Expect(mgrClient.Create(ctx, secret)).To(Succeed())

			DeferCleanup(func() {
				Expect(mgrClient.Delete(ctx, secret)).To(Succeed())
			})
		})

		It("should not create a token file before the kubeconfig is present", func() {
			Consistently(func(g Gomega) {
				exists, err := afero.Exists(fs, tokenFilePath)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(exists).To(BeFalse())
			}).Should(Succeed())
		})

		It("should run the token-requestor to bootstrap the token when the kubeconfig has none", func() {
			By("Add generic kubeconfig without a token")
			kubeConfigData, err := marshalConfig(config)
			Expect(err).NotTo(HaveOccurred())
			secret.Data = map[string][]byte{"kubeconfig": kubeConfigData}
			Expect(mgrClient.Update(ctx, secret)).To(Succeed())

			By("Wait for the token-requestor to populate the kubeconfig and the controller to forward it")
			Eventually(func(g Gomega) {
				token, err := afero.ReadFile(fs, tokenFilePath)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(string(token)).To(Equal(issuedToken))
				g.Expect(channel).To(Receive(&ev))
			}).Should(Succeed())
			Expect(ev.Object).To(Equal(restConfigWithTokenPath(kubeconfigFromSecret(secret), tokenFilePath)))
		})

		It("should forward an already populated token and update it on changes", func() {
			By("Fill kubeconfig with a token and a renew timestamp in the future")
			addTokenToConfig(config, "Z2FyZGVuZXIK")
			kubeConfigData, err := marshalConfig(config)
			Expect(err).NotTo(HaveOccurred())
			secret.Data = map[string][]byte{"kubeconfig": kubeConfigData}
			metav1.SetMetaDataAnnotation(&secret.ObjectMeta, resourcesv1alpha1.ServiceAccountTokenRenewTimestamp, time.Now().Add(time.Hour).UTC().Format(time.RFC3339))
			Expect(mgrClient.Update(ctx, secret)).To(Succeed())

			Eventually(func(g Gomega) {
				token, err := afero.ReadFile(fs, tokenFilePath)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(string(token)).To(Equal("Z2FyZGVuZXIK"))
				g.Expect(channel).To(Receive(&ev))
			}).Should(Succeed())
			Expect(ev.Object).To(Equal(restConfigWithTokenPath(kubeConfigData, tokenFilePath)))

			By("Update token in kubeconfig")
			addTokenToConfig(config, "Ym90YW5pc3QK")
			kubeConfigData, err = marshalConfig(config)
			Expect(err).NotTo(HaveOccurred())
			secret.Data["kubeconfig"] = kubeConfigData
			Expect(mgrClient.Update(ctx, secret)).To(Succeed())

			Eventually(func(g Gomega) {
				token, err := afero.ReadFile(fs, tokenFilePath)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(string(token)).To(Equal("Ym90YW5pc3QK"))
				g.Expect(channel).To(Receive(&ev))
			}).Should(Succeed())
			Expect(ev.Object).To(Equal(restConfigWithTokenPath(kubeConfigData, tokenFilePath)))
		})

		It("should renew the token via the token-requestor once the renew timestamp has been reached", func() {
			By("Fill kubeconfig with a token whose renew timestamp lies in the past")
			addTokenToConfig(config, "ZXhwaXJlZAo=")
			kubeConfigData, err := marshalConfig(config)
			Expect(err).NotTo(HaveOccurred())
			secret.Data = map[string][]byte{"kubeconfig": kubeConfigData}
			metav1.SetMetaDataAnnotation(&secret.ObjectMeta, resourcesv1alpha1.ServiceAccountTokenRenewTimestamp, time.Now().Add(-time.Hour).UTC().Format(time.RFC3339))
			Expect(mgrClient.Update(ctx, secret)).To(Succeed())

			By("Wait for the token-requestor to replace the expired token and the controller to forward it")
			Eventually(func(g Gomega) {
				token, err := afero.ReadFile(fs, tokenFilePath)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(string(token)).To(Equal(issuedToken))
				g.Expect(channel).To(Receive(&ev))
			}).Should(Succeed())
			Expect(ev.Object).To(Equal(restConfigWithTokenPath(kubeconfigFromSecret(secret), tokenFilePath)))
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

// kubeconfigFromSecret reads back the (token-requestor-mutated) kubeconfig from the gardener-internal secret, so the
// expected REST config is built from the exact bytes the controller observed.
func kubeconfigFromSecret(secret *corev1.Secret) []byte {
	updated := secret.DeepCopy()
	ExpectWithOffset(1, mgrClient.Get(ctx, client.ObjectKeyFromObject(secret), updated)).To(Succeed())
	return updated.Data["kubeconfig"]
}

func restConfigWithTokenPath(kubeConfigRaw []byte, tokenPath string) *rest.Config {
	restConfig, err := clientcmd.RESTConfigFromKubeConfig(kubeConfigRaw)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	restConfig.BearerToken = ""
	restConfig.BearerTokenFile = tokenPath
	return restConfig
}

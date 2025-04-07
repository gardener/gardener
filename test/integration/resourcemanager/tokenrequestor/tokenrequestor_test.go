// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package tokenrequestor_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/rest"
	clientcmdlatest "k8s.io/client-go/tools/clientcmd/api/latest"
	clientcmdv1 "k8s.io/client-go/tools/clientcmd/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("TokenRequestor tests", func() {
	var (
		resourceName string

		secret         *corev1.Secret
		serviceAccount *corev1.ServiceAccount
	)

	BeforeEach(func() {
		resourceName = "test-" + utils.ComputeSHA256Hex([]byte(uuid.NewUUID()))[:8]

		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      resourceName,
				Namespace: testNamespace.Name,
				Annotations: map[string]string{
					"serviceaccount.resources.gardener.cloud/name":      resourceName,
					"serviceaccount.resources.gardener.cloud/namespace": testNamespace.Name,
					"serviceaccount.resources.gardener.cloud/labels":    `{"foo":"bar"}`,
				},
				Labels: map[string]string{
					"resources.gardener.cloud/purpose": "token-requestor",
				},
			},
		}
		serviceAccount = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      resourceName,
				Namespace: testNamespace.Name,
			},
		}

		fakeClock.SetTime(time.Now().Round(time.Second))
	})

	It("should behave correctly when: create w/o label, update w/ label, delete w/ label", func() {
		secret.Labels = nil
		Expect(testClient.Create(ctx, secret)).To(Succeed())

		Consistently(func() error {
			return testClient.Get(ctx, client.ObjectKeyFromObject(serviceAccount), serviceAccount)
		}).Should(BeNotFoundError())

		secret.Labels = map[string]string{"resources.gardener.cloud/purpose": "token-requestor"}
		Expect(testClient.Update(ctx, secret)).To(Succeed())

		Eventually(func() error {
			return testClient.Get(ctx, client.ObjectKeyFromObject(serviceAccount), serviceAccount)
		}).Should(Succeed())
		Expect(serviceAccount.Labels).To(Equal(map[string]string{"foo": "bar"}))

		Expect(testClient.Delete(ctx, secret)).To(Succeed())

		Consistently(func() error {
			return testClient.Get(ctx, client.ObjectKeyFromObject(serviceAccount), serviceAccount)
		}).Should(Succeed())
	})

	It("should behave correctly when: create w/ label, update w/o label, delete w/o label", func() {
		Expect(testClient.Create(ctx, secret)).To(Succeed())

		Eventually(func() error {
			return testClient.Get(ctx, client.ObjectKeyFromObject(serviceAccount), serviceAccount)
		}).Should(Succeed())
		Expect(serviceAccount.Labels).To(Equal(map[string]string{"foo": "bar"}))

		patch := client.MergeFrom(secret.DeepCopy())
		secret.Labels = nil
		Expect(testClient.Patch(ctx, secret, patch)).To(Succeed())

		Consistently(func() error {
			return testClient.Get(ctx, client.ObjectKeyFromObject(serviceAccount), serviceAccount)
		}).Should(Succeed())

		Expect(testClient.Delete(ctx, secret)).To(Succeed())

		Consistently(func() error {
			return testClient.Get(ctx, client.ObjectKeyFromObject(serviceAccount), serviceAccount)
		}).Should(Succeed())
	})

	Context("it should be able to authenticate", func() {
		var newRestConfig *rest.Config

		AfterEach(func() {
			newClient, err := client.New(newRestConfig, client.Options{Mapper: testClient.RESTMapper()})
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() error {
				return newClient.Get(ctx, client.ObjectKeyFromObject(serviceAccount), serviceAccount)
			}).Should(BeForbiddenError())
		})

		It("should be able to authenticate with the created token", func() {
			Expect(testClient.Create(ctx, secret)).To(Succeed())

			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(serviceAccount), serviceAccount)).To(Succeed())

				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())
				g.Expect(secret.Annotations).To(HaveKey("serviceaccount.resources.gardener.cloud/token-renew-timestamp"))
			}).Should(Succeed())

			newRestConfig = &rest.Config{
				Host:            restConfig.Host,
				BearerToken:     string(secret.Data["token"]),
				TLSClientConfig: rest.TLSClientConfig{CAData: restConfig.CAData},
			}
		})

		It("should be able to authenticate with the created token and CABundle", func() {
			secret.Annotations["serviceaccount.resources.gardener.cloud/inject-ca-bundle"] = "true"
			Expect(testClient.Create(ctx, secret)).To(Succeed())

			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(serviceAccount), serviceAccount)).To(Succeed())

				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())
				g.Expect(secret.Annotations).To(HaveKey("serviceaccount.resources.gardener.cloud/token-renew-timestamp"))
			}).Should(Succeed())

			newRestConfig = &rest.Config{
				Host:            restConfig.Host,
				BearerToken:     string(secret.Data["token"]),
				TLSClientConfig: rest.TLSClientConfig{CAData: secret.Data["bundle.crt"]},
			}
		})

		It("should be able to authenticate with the updated kubeconfig", func() {
			kubeconfig := &clientcmdv1.Config{
				CurrentContext: "config",
				Clusters: []clientcmdv1.NamedCluster{{
					Name: "config",
					Cluster: clientcmdv1.Cluster{
						Server:                   restConfig.Host,
						CertificateAuthorityData: restConfig.CAData,
					},
				}},
				AuthInfos: []clientcmdv1.NamedAuthInfo{{
					Name: "config",
				}},
				Contexts: []clientcmdv1.NamedContext{{
					Name: "config",
					Context: clientcmdv1.Context{
						Cluster:  "config",
						AuthInfo: "config",
					},
				}},
			}
			kubeconfigRaw, err := runtime.Encode(clientcmdlatest.Codec, kubeconfig)
			Expect(err).NotTo(HaveOccurred())

			secret.Data = map[string][]byte{"kubeconfig": kubeconfigRaw}
			secret.Labels = map[string]string{"resources.gardener.cloud/purpose": "token-requestor"}
			Expect(testClient.Create(ctx, secret)).To(Succeed())

			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(serviceAccount), serviceAccount)).Should(Succeed())

				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())
				g.Expect(secret.Annotations).To(HaveKey("serviceaccount.resources.gardener.cloud/token-renew-timestamp"))
			}).Should(Succeed())

			newRestConfig, err = kubernetes.RESTConfigFromClientConnectionConfiguration(nil, secret.Data["kubeconfig"])
			Expect(err).NotTo(HaveOccurred())
		})

		It("should be able to authenticate with the updated kubeconfig (including CABundle)", func() {
			kubeconfig := &clientcmdv1.Config{
				CurrentContext: "config",
				Clusters: []clientcmdv1.NamedCluster{{
					Name: "config",
					Cluster: clientcmdv1.Cluster{
						Server: restConfig.Host,
					},
				}},
				AuthInfos: []clientcmdv1.NamedAuthInfo{{
					Name: "config",
				}},
				Contexts: []clientcmdv1.NamedContext{{
					Name: "config",
					Context: clientcmdv1.Context{
						Cluster:  "config",
						AuthInfo: "config",
					},
				}},
			}
			kubeconfigRaw, err := runtime.Encode(clientcmdlatest.Codec, kubeconfig)
			Expect(err).NotTo(HaveOccurred())

			secret.Data = map[string][]byte{"kubeconfig": kubeconfigRaw}
			secret.Labels = map[string]string{"resources.gardener.cloud/purpose": "token-requestor"}
			secret.Annotations["serviceaccount.resources.gardener.cloud/inject-ca-bundle"] = "true"
			Expect(testClient.Create(ctx, secret)).To(Succeed())

			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(serviceAccount), serviceAccount)).Should(Succeed())

				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())
				g.Expect(secret.Annotations).To(HaveKey("serviceaccount.resources.gardener.cloud/token-renew-timestamp"))
			}).Should(Succeed())

			newRestConfig, err = kubernetes.RESTConfigFromClientConnectionConfiguration(nil, secret.Data["kubeconfig"])
			Expect(err).NotTo(HaveOccurred())
		})
	})

	AfterEach(func() {
		Expect(testClient.Delete(ctx, secret)).To(Or(Succeed(), BeNotFoundError()))
		Eventually(func() error {
			return testClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)
		}).Should(BeNotFoundError())

		Expect(testClient.Delete(ctx, serviceAccount)).To(Or(Succeed(), BeNotFoundError()))
		Eventually(func() error {
			return testClient.Get(ctx, client.ObjectKeyFromObject(serviceAccount), serviceAccount)
		}).Should(BeNotFoundError())
	})
})

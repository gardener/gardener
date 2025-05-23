// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package tokenrequest_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	. "github.com/gardener/gardener/pkg/utils/gardener/tokenrequest"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
)

var _ = Describe("Secrets", func() {
	var (
		ctx = context.Background()

		namespace          = "foo-bar"
		c                  client.Client
		fakeSecretsManager secretsmanager.Interface
	)

	BeforeEach(func() {
		c = fakeclient.NewClientBuilder().Build()
		fakeSecretsManager = fakesecretsmanager.New(c, namespace)

		var err error
		_, err = fakeSecretsManager.Generate(
			ctx,
			&secretsutils.CertificateSecretConfig{Name: v1beta1constants.SecretNameCACluster, CommonName: "kubernetes", CertType: secretsutils.CACert},
		)
		Expect(err).ShouldNot(HaveOccurred())
	})

	Describe("GenerateGenericTokenKubeconfig", func() {
		It("should generate the generic token kubeconfig", func() {
			secret, err := GenerateGenericTokenKubeconfig(ctx, fakeSecretsManager, namespace, "kube-apiserver")
			Expect(err).ShouldNot(HaveOccurred())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())
		})

		It("should generate a new generic token kubeconfig while keeping the old one", func() {
			By("create kubeconfig with existing CA")
			secretBefore, err := GenerateGenericTokenKubeconfig(ctx, fakeSecretsManager, namespace, "kube-apiserver")
			Expect(err).ShouldNot(HaveOccurred())
			Expect(err).NotTo(HaveOccurred())

			By("update CA with new bundle.crt data")
			ca, found := fakeSecretsManager.Get(v1beta1constants.SecretNameCACluster)
			Expect(found).To(BeTrue())
			ca.Data["bundle.crt"] = []byte("====rotatedCA")
			Expect(c.Update(ctx, ca)).To(Succeed())

			By("create kubeconfig with new CA data")
			secretAfter, err := GenerateGenericTokenKubeconfig(ctx, fakeSecretsManager, namespace, "kube-apiserver")
			Expect(err).ShouldNot(HaveOccurred())

			By("check results")
			Expect(secretBefore.Name).NotTo(Equal(secretAfter.Name))
			Expect(c.Get(ctx, client.ObjectKeyFromObject(secretBefore), secretBefore)).To(Succeed())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(secretAfter), secretAfter)).To(Succeed())
		})
	})

	Describe("#RenewAccessSecrets", func() {
		It("should remove the renew-timestamp annotation from all relevant access secrets", func() {
			var (
				relevantSecrets = []*corev1.Secret{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:        "secret1",
							Namespace:   namespace,
							Annotations: map[string]string{"serviceaccount.resources.gardener.cloud/token-renew-timestamp": "foo"},
							Labels: map[string]string{
								"resources.gardener.cloud/purpose": "token-requestor",
								"resources.gardener.cloud/class":   "shoot",
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:        "secret2",
							Namespace:   namespace,
							Annotations: map[string]string{"serviceaccount.resources.gardener.cloud/token-renew-timestamp": "foo"},
							Labels: map[string]string{
								"resources.gardener.cloud/purpose": "token-requestor",
								"resources.gardener.cloud/class":   "shoot",
							},
						},
					},
				}
				irrelevantSecrets = []*corev1.Secret{
					{
						// doesn't have the token-requestor label
						ObjectMeta: metav1.ObjectMeta{
							Name:        "secret3",
							Namespace:   namespace,
							Annotations: map[string]string{"serviceaccount.resources.gardener.cloud/token-renew-timestamp": "foo"},
							Labels:      map[string]string{"resources.gardener.cloud/class": "shoot"},
						},
					},
					{
						// in another namespace
						ObjectMeta: metav1.ObjectMeta{
							Name:        "secret4",
							Namespace:   namespace + "-other",
							Annotations: map[string]string{"serviceaccount.resources.gardener.cloud/token-renew-timestamp": "foo"},
							Labels: map[string]string{
								"resources.gardener.cloud/purpose": "token-requestor",
								"resources.gardener.cloud/class":   "shoot",
							},
						},
					},
					{
						// different class
						ObjectMeta: metav1.ObjectMeta{
							Name:        "secret5",
							Namespace:   namespace,
							Annotations: map[string]string{"serviceaccount.resources.gardener.cloud/token-renew-timestamp": "foo"},
							Labels: map[string]string{
								"resources.gardener.cloud/purpose": "token-requestor",
								"resources.gardener.cloud/class":   "garden",
							},
						},
					},
				}
			)

			for _, secret := range append(relevantSecrets, irrelevantSecrets...) {
				Expect(c.Create(ctx, secret)).To(Succeed(), "should be able to create secret %s", client.ObjectKeyFromObject(secret))
			}

			Expect(RenewAccessSecrets(ctx, c, client.InNamespace(namespace), client.MatchingLabels{"resources.gardener.cloud/class": "shoot"})).To(Succeed())

			for _, secret := range relevantSecrets {
				key := client.ObjectKeyFromObject(secret)
				Expect(c.Get(ctx, key, secret)).To(Succeed(), "should be able to get secret %s", key)
				Expect(secret.Annotations).NotTo(HaveKey("serviceaccount.resources.gardener.cloud/token-renew-timestamp"),
					"should have removed renew timestamp from relevant secret %s", key,
				)
			}

			for _, secret := range irrelevantSecrets {
				key := client.ObjectKeyFromObject(secret)
				Expect(c.Get(ctx, key, secret)).To(Succeed(), "should be able to get secret %s", key)
				Expect(secret.Annotations).To(HaveKey("serviceaccount.resources.gardener.cloud/token-renew-timestamp"),
					"should not have removed renew timestamp from irrelevant secret %s", key,
				)
			}
		})
	})

	Describe("#RenewWorkloadIdentityTokens", func() {
		It("should remove the renew-timestamp annotation from all relevant workload identity secrets", func() {
			var (
				relevantSecrets = []*corev1.Secret{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:        "secret1",
							Namespace:   namespace,
							Annotations: map[string]string{"workloadidentity.security.gardener.cloud/token-renew-timestamp": "foo"},
							Labels: map[string]string{
								"security.gardener.cloud/purpose": "workload-identity-token-requestor",
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:        "secret2",
							Namespace:   namespace,
							Annotations: map[string]string{"workloadidentity.security.gardener.cloud/token-renew-timestamp": "foo"},
							Labels: map[string]string{
								"security.gardener.cloud/purpose": "workload-identity-token-requestor",
							},
						},
					},
				}
				irrelevantSecrets = []*corev1.Secret{
					{
						// doesn't have the token-requestor label
						ObjectMeta: metav1.ObjectMeta{
							Name:        "secret3",
							Namespace:   namespace,
							Annotations: map[string]string{"workloadidentity.security.gardener.cloud/token-renew-timestamp": "foo"},
						},
					},
					{
						// in another namespace
						ObjectMeta: metav1.ObjectMeta{
							Name:        "secret4",
							Namespace:   namespace + "-other",
							Annotations: map[string]string{"workloadidentity.security.gardener.cloud/token-renew-timestamp": "foo"},
							Labels: map[string]string{
								"resources.gardener.cloud/purpose": "token-requestor",
								"resources.gardener.cloud/class":   "shoot",
							},
						},
					},
				}
			)

			for _, secret := range append(relevantSecrets, irrelevantSecrets...) {
				Expect(c.Create(ctx, secret)).To(Succeed(), "should be able to create secret %s", client.ObjectKeyFromObject(secret))
			}

			Expect(RenewWorkloadIdentityTokens(ctx, c, client.InNamespace(namespace))).To(Succeed())

			for _, secret := range relevantSecrets {
				key := client.ObjectKeyFromObject(secret)
				Expect(c.Get(ctx, key, secret)).To(Succeed(), "should be able to get secret %s", key)
				Expect(secret.Annotations).NotTo(HaveKey("workloadidentity.security.gardener.cloud/token-renew-timestamp"),
					"should have removed renew timestamp from relevant secret %s", key,
				)
			}

			for _, secret := range irrelevantSecrets {
				key := client.ObjectKeyFromObject(secret)
				Expect(c.Get(ctx, key, secret)).To(Succeed(), "should be able to get secret %s", key)
				Expect(secret.Annotations).To(HaveKey("workloadidentity.security.gardener.cloud/token-renew-timestamp"),
					"should not have removed renew timestamp from irrelevant secret %s", key,
				)
			}
		})
	})

	Describe("#IsTokenPopulated", func() {
		var (
			kubeconfigWithToken = `apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: AAAA
    server: https://foobar
  name: garden
contexts:
- context:
    cluster: garden
    user: garden
  name: garden
current-context: garden
kind: Config
preferences: {}
users:
- name: garden
  user:
    token: bar
`
			kubeconfigWithoutToken = `apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: AAAA
    server: https://foobar
  name: garden
contexts:
- context:
    cluster: garden
    user: garden
  name: garden
current-context: garden
kind: Config
preferences: {}
users:
- name: garden
  user:
    token: ""
`
			kubeconfigWrongContext = `apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: AAAA
    server: https://foobar
  name: garden
contexts:
- context:
    cluster: garden
    user: garden
  name: garden
current-context: foo
kind: Config
preferences: {}
users:
- name: garden
  user:
    token: bar
`
			kubeconfigNoUser = `apiVersion: v1
clusters:
- cluster:
    certificate-authority-data: AAAA
    server: https://foobar
  name: garden
contexts:
- context:
    cluster: garden
    user: garden
  name: garden
- context:
  cluster: garden
  user: foo
name: foo
current-context: foo
kind: Config
preferences: {}
users:
- name: garden
  user:
    token: bar
`
		)
		DescribeTable("#IsTokenPopulated",
			func(kubeconfig string, result bool) {
				secret := corev1.Secret{
					Data: map[string][]byte{"kubeconfig": []byte(kubeconfig)},
				}
				populated, err := IsTokenPopulated(&secret)
				Expect(err).To(Succeed())
				Expect(populated).To(Equal(result))
			},
			Entry("kubeconfig with token should return true", kubeconfigWithToken, true),
			Entry("kubeconfig without token should return false", kubeconfigWithoutToken, false),
			Entry("kubeconfig with wrong context should return false", kubeconfigWrongContext, false),
			Entry("kubeconfig without user should return false", kubeconfigNoUser, false),
		)
	})
})

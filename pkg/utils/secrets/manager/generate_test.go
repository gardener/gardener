// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package manager

import (
	"context"
	"crypto/rsa"
	"io"
	"time"

	secretutils "github.com/gardener/gardener/pkg/utils/secrets"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var (
	oldGenerateRandomString func(int) (string, error)
	oldGenerateKey          func(io.Reader, int) (*rsa.PrivateKey, error)
)

var _ = BeforeSuite(func() {
	oldGenerateRandomString = secretutils.GenerateRandomString
	secretutils.GenerateRandomString = secretutils.FakeGenerateRandomString

	oldGenerateKey = secretutils.GenerateKey
	secretutils.GenerateKey = secretutils.FakeGenerateKey
})

var _ = AfterSuite(func() {
	secretutils.GenerateRandomString = oldGenerateRandomString

	secretutils.GenerateKey = oldGenerateKey
})

var _ = Describe("Generate", func() {
	var (
		ctx       = context.TODO()
		namespace = "shoot--foo--bar"

		m          *manager
		fakeClient client.Client
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).Build()
		m = New(logr.Discard(), fakeClient, namespace, "test", nil).(*manager)
	})

	Describe("#Generate", func() {
		name := "config"

		Context("for non-certificate secrets", func() {
			var config *secretutils.BasicAuthSecretConfig

			BeforeEach(func() {
				config = &secretutils.BasicAuthSecretConfig{
					Name:           name,
					Format:         secretutils.BasicAuthFormatNormal,
					Username:       "foo",
					PasswordLength: 3,
				}
			})

			It("should generate a new secret", func() {
				By("generating new secret")
				secret, err := m.Generate(ctx, config)
				Expect(err).NotTo(HaveOccurred())
				expectSecretWasCreated(ctx, fakeClient, secret)

				By("verifying internal store reflects changes")
				secretInfos, found := m.getFromStore(name)
				Expect(found).To(BeTrue())
				Expect(secretInfos.current.obj).To(Equal(secret))
				Expect(secretInfos.old).To(BeNil())
				Expect(secretInfos.bundle).To(BeNil())
			})

			It("should generate a new secret when the config changes", func() {
				By("generating new secret")
				secret, err := m.Generate(ctx, config)
				Expect(err).NotTo(HaveOccurred())
				expectSecretWasCreated(ctx, fakeClient, secret)

				By("changing secret config and generate again")
				config.PasswordLength = 4
				newSecret, err := m.Generate(ctx, config)
				Expect(err).NotTo(HaveOccurred())
				expectSecretWasCreated(ctx, fakeClient, newSecret)

				By("verifying internal store reflects changes")
				secretInfos, found := m.getFromStore(name)
				Expect(found).To(BeTrue())
				Expect(secretInfos.current.obj).To(Equal(newSecret))
				Expect(secretInfos.old).To(BeNil())
				Expect(secretInfos.bundle).To(BeNil())
			})

			It("should generate a new secret when the last rotation initiation time changes", func() {
				By("generating new secret")
				secret, err := m.Generate(ctx, config)
				Expect(err).NotTo(HaveOccurred())
				expectSecretWasCreated(ctx, fakeClient, secret)

				By("changing last rotation initiation time and generate again")
				m = New(logr.Discard(), fakeClient, namespace, "test", map[string]time.Time{name: time.Now()}).(*manager)

				newSecret, err := m.Generate(ctx, config)
				Expect(err).NotTo(HaveOccurred())
				expectSecretWasCreated(ctx, fakeClient, newSecret)

				By("verifying internal store reflects changes")
				secretInfos, found := m.getFromStore(name)
				Expect(found).To(BeTrue())
				Expect(secretInfos.current.obj).To(Equal(newSecret))
				Expect(secretInfos.old).To(BeNil())
				Expect(secretInfos.bundle).To(BeNil())
			})

			It("should store the old secret if rotation strategy is KeepOld", func() {
				By("generating new secret")
				secret, err := m.Generate(ctx, config)
				Expect(err).NotTo(HaveOccurred())
				expectSecretWasCreated(ctx, fakeClient, secret)

				By("changing secret config and generate again with KeepOld strategy")
				config.PasswordLength = 4
				newSecret, err := m.Generate(ctx, config, Rotate(KeepOld))
				Expect(err).NotTo(HaveOccurred())
				expectSecretWasCreated(ctx, fakeClient, newSecret)

				By("verifying internal store reflects changes")
				secretInfos, found := m.getFromStore(name)
				Expect(found).To(BeTrue())
				Expect(secretInfos.current.obj).To(Equal(newSecret))
				Expect(secretInfos.old.obj).To(Equal(secret))
				Expect(secretInfos.bundle).To(BeNil())
			})

			It("should not store the old secret even if rotation strategy is KeepOld when old secrets shall be ignored", func() {
				By("generating new secret")
				secret, err := m.Generate(ctx, config)
				Expect(err).NotTo(HaveOccurred())
				expectSecretWasCreated(ctx, fakeClient, secret)

				By("changing secret config and generate again with KeepOld strategy and ignore old secrets option")
				config.PasswordLength = 4
				newSecret, err := m.Generate(ctx, config, Rotate(KeepOld), IgnoreOldSecrets())
				Expect(err).NotTo(HaveOccurred())
				expectSecretWasCreated(ctx, fakeClient, newSecret)

				By("verifying internal store reflects changes")
				secretInfos, found := m.getFromStore(name)
				Expect(found).To(BeTrue())
				Expect(secretInfos.current.obj).To(Equal(newSecret))
				Expect(secretInfos.old).To(BeNil())
				Expect(secretInfos.bundle).To(BeNil())
			})

			It("should reconcile the secret", func() {
				By("generating new secret")
				secret, err := m.Generate(ctx, config)
				Expect(err).NotTo(HaveOccurred())
				expectSecretWasCreated(ctx, fakeClient, secret)

				By("marking secret as mutable")
				patch := client.MergeFrom(secret.DeepCopy())
				secret.Immutable = nil
				Expect(fakeClient.Patch(ctx, secret, patch)).To(Succeed())

				By("changing options and generate again")
				secret, err = m.Generate(ctx, config, Persist())
				Expect(err).NotTo(HaveOccurred())

				By("verifying labels got reconciled")
				foundSecret := &corev1.Secret{}
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(secret), foundSecret)).To(Succeed())
				Expect(foundSecret.Labels).To(HaveKeyWithValue("persist", "true"))
				Expect(foundSecret.Immutable).To(PointTo(BeTrue()))
			})
		})

		Context("for CA certificate secrets", func() {
			var config *secretutils.CertificateSecretConfig

			BeforeEach(func() {
				config = &secretutils.CertificateSecretConfig{
					Name:       name,
					CommonName: name,
					CertType:   secretutils.CACert,
				}
			})

			It("should generate a new CA secret and a corresponding bundle", func() {
				By("generating new secret")
				secret, err := m.Generate(ctx, config)
				Expect(err).NotTo(HaveOccurred())
				expectSecretWasCreated(ctx, fakeClient, secret)

				By("finding created bundle secret")
				secretList := &corev1.SecretList{}
				Expect(fakeClient.List(ctx, secretList, client.InNamespace(namespace), client.MatchingLabels{
					"managed-by":       "secrets-manager",
					"manager-identity": "test",
					"bundle-for":       name,
				})).To(Succeed())
				Expect(secretList.Items).To(HaveLen(1))

				By("verifying internal store reflects changes")
				secretInfos, found := m.getFromStore(name)
				Expect(found).To(BeTrue())
				Expect(secretInfos.current.obj).To(Equal(secret))
				Expect(secretInfos.old).To(BeNil())
				Expect(secretInfos.bundle.obj).To(PointTo(Equal(secretList.Items[0])))
			})

			It("should rotate a CA secret and add old and new to the corresponding bundle", func() {
				By("generating new secret")
				secret, err := m.Generate(ctx, config)
				Expect(err).NotTo(HaveOccurred())
				expectSecretWasCreated(ctx, fakeClient, secret)

				By("storing old bundle secret")
				secretInfos, found := m.getFromStore(name)
				Expect(found).To(BeTrue())
				oldBundleSecret := secretInfos.bundle.obj

				By("changing secret config and generate again")
				m = New(logr.Discard(), fakeClient, namespace, "test", map[string]time.Time{name: time.Now()}).(*manager)
				newSecret, err := m.Generate(ctx, config, Rotate(KeepOld))
				Expect(err).NotTo(HaveOccurred())
				expectSecretWasCreated(ctx, fakeClient, newSecret)

				By("finding created bundle secret")
				secretList := &corev1.SecretList{}
				Expect(fakeClient.List(ctx, secretList, client.InNamespace(namespace), client.MatchingLabels{
					"managed-by":       "secrets-manager",
					"manager-identity": "test",
					"bundle-for":       name,
				})).To(Succeed())
				Expect(secretList.Items).To(HaveLen(2))

				By("verifying internal store reflects changes")
				secretInfos, found = m.getFromStore(name)
				Expect(found).To(BeTrue())
				Expect(secretInfos.current.obj).To(Equal(newSecret))
				Expect(secretInfos.old.obj).To(Equal(secret))
				Expect(secretInfos.bundle.obj).NotTo(PointTo(Equal(oldBundleSecret)))
			})
		})

		Context("for certificate secrets", func() {
			var (
				caName, serverName, clientName       = "ca", "server", "client"
				caConfig, serverConfig, clientConfig *secretutils.CertificateSecretConfig
			)

			BeforeEach(func() {
				caConfig = &secretutils.CertificateSecretConfig{
					Name:       caName,
					CommonName: caName,
					CertType:   secretutils.CACert,
				}
				serverConfig = &secretutils.CertificateSecretConfig{
					Name:                        serverName,
					CommonName:                  serverName,
					CertType:                    secretutils.ServerCert,
					SkipPublishingCACertificate: true,
				}
				clientConfig = &secretutils.CertificateSecretConfig{
					Name:                        clientName,
					CommonName:                  clientName,
					CertType:                    secretutils.ClientCert,
					SkipPublishingCACertificate: true,
				}
			})

			It("should keep the same server cert even when the CA rotates", func() {
				By("generating new CA secret")
				caSecret, err := m.Generate(ctx, caConfig)
				Expect(err).NotTo(HaveOccurred())
				expectSecretWasCreated(ctx, fakeClient, caSecret)

				By("generating new server secret")
				serverSecret, err := m.Generate(ctx, serverConfig, SignedByCA(caName))
				Expect(err).NotTo(HaveOccurred())
				expectSecretWasCreated(ctx, fakeClient, serverSecret)

				By("rotating CA")
				m = New(logr.Discard(), fakeClient, namespace, "test", map[string]time.Time{caName: time.Now()}).(*manager)
				newCASecret, err := m.Generate(ctx, caConfig, Rotate(KeepOld))
				Expect(err).NotTo(HaveOccurred())
				expectSecretWasCreated(ctx, fakeClient, newCASecret)

				By("get or generate server secret")
				newServerSecret, err := m.Generate(ctx, serverConfig, SignedByCA(caName))
				Expect(err).NotTo(HaveOccurred())
				expectSecretWasCreated(ctx, fakeClient, newServerSecret)

				By("verifying server secret is still the same")
				serverSecret.TypeMeta = metav1.TypeMeta{APIVersion: "v1", Kind: "Secret"}
				Expect(newServerSecret).To(Equal(serverSecret))
			})

			It("should regenerate the client cert when the CA rotates", func() {
				By("generating new CA secret")
				caSecret, err := m.Generate(ctx, caConfig)
				Expect(err).NotTo(HaveOccurred())
				expectSecretWasCreated(ctx, fakeClient, caSecret)

				By("generating new client secret")
				clientSecret, err := m.Generate(ctx, clientConfig, SignedByCA(caName))
				Expect(err).NotTo(HaveOccurred())
				expectSecretWasCreated(ctx, fakeClient, clientSecret)

				By("rotating CA")
				m = New(logr.Discard(), fakeClient, namespace, "test", map[string]time.Time{caName: time.Now()}).(*manager)
				newCASecret, err := m.Generate(ctx, caConfig, Rotate(KeepOld))
				Expect(err).NotTo(HaveOccurred())
				expectSecretWasCreated(ctx, fakeClient, newCASecret)

				By("get or generate client secret")
				newClientSecret, err := m.Generate(ctx, clientConfig, SignedByCA(caName))
				Expect(err).NotTo(HaveOccurred())
				expectSecretWasCreated(ctx, fakeClient, newClientSecret)

				By("verifying client secret is changed")
				clientSecret.TypeMeta = metav1.TypeMeta{APIVersion: "v1", Kind: "Secret"}
				Expect(newClientSecret).NotTo(Equal(clientSecret))
			})
		})

		Context("backwards compatibility", func() {
			Context("kube-apiserver basic auth", func() {
				var (
					userName    = "admin"
					oldPassword = "old-basic-auth-password"
					config      *secretutils.BasicAuthSecretConfig
				)

				BeforeEach(func() {
					config = &secretutils.BasicAuthSecretConfig{
						Name:           "kube-apiserver-basic-auth",
						Format:         secretutils.BasicAuthFormatCSV,
						Username:       userName,
						PasswordLength: 32,
					}
				})

				It("should generate a new password if old secret does not exist", func() {
					By("generating secret")
					secret, err := m.Generate(ctx, config)
					Expect(err).NotTo(HaveOccurred())

					By("verifying new password was generated")
					basicAuth, err := secretutils.LoadBasicAuthFromCSV("", secret.Data[secretutils.DataKeyCSV])
					Expect(err).NotTo(HaveOccurred())
					Expect(basicAuth.Password).NotTo(Equal(oldPassword))
				})

				It("should keep the existing password if old secret still exists", func() {
					oldBasicAuth := &secretutils.BasicAuth{
						Format:   secretutils.BasicAuthFormatCSV,
						Username: userName,
						Password: oldPassword,
					}

					By("creating existing secret with old password")
					existingSecret := &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "kube-apiserver-basic-auth",
							Namespace: namespace,
						},
						Type: corev1.SecretTypeOpaque,
						Data: oldBasicAuth.SecretData(),
					}
					Expect(fakeClient.Create(ctx, existingSecret)).To(Succeed())

					By("generating secret")
					secret, err := m.Generate(ctx, config)
					Expect(err).NotTo(HaveOccurred())

					By("verifying old password was kept")
					basicAuth, err := secretutils.LoadBasicAuthFromCSV("", secret.Data[secretutils.DataKeyCSV])
					Expect(err).NotTo(HaveOccurred())
					Expect(basicAuth.Password).To(Equal(oldPassword))
				})
			})

			Context("kube-apiserver static token", func() {
				var (
					user1, user2                 = "user1", "user2"
					oldUser1Token, oldUser2Token = "old-static-token-1", "old-static-token-2"
					user1Token                   = secretutils.TokenConfig{
						Username: user1,
						UserID:   user1,
						Groups:   []string{"my-group1"},
					}
					user2Token = secretutils.TokenConfig{
						Username: user2,
						UserID:   user2,
					}

					config *secretutils.StaticTokenSecretConfig
				)

				BeforeEach(func() {
					config = &secretutils.StaticTokenSecretConfig{
						Name: "kube-apiserver-static-token",
						Tokens: map[string]secretutils.TokenConfig{
							user1: user1Token,
							user2: user2Token,
						},
					}
				})

				It("should generate new tokens if old secret does not exist", func() {
					By("generating secret")
					secret, err := m.Generate(ctx, config)
					Expect(err).NotTo(HaveOccurred())

					By("verifying new tokens were generated")
					staticToken, err := secretutils.LoadStaticTokenFromCSV("", secret.Data[secretutils.DataKeyStaticTokenCSV])
					Expect(err).NotTo(HaveOccurred())
					for _, token := range staticToken.Tokens {
						if token.Username == user1 {
							Expect(token.Token).NotTo(Equal(oldUser1Token))
						}
						if token.Username == user2 {
							Expect(token.Token).NotTo(Equal(oldUser2Token))
						}
					}
				})

				It("should generate keep the old tokens if old secret does still exist", func() {
					oldBasicAuth := &secretutils.StaticToken{
						Tokens: []secretutils.Token{
							{
								Username: user1Token.Username,
								UserID:   user1Token.UserID,
								Groups:   user1Token.Groups,
								Token:    oldUser1Token,
							},
							{
								Username: user2Token.Username,
								UserID:   user2Token.UserID,
								Groups:   user2Token.Groups,
								Token:    oldUser2Token,
							},
						},
					}

					By("creating existing secret with old password")
					existingSecret := &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "static-token",
							Namespace: namespace,
						},
						Type: corev1.SecretTypeOpaque,
						Data: oldBasicAuth.SecretData(),
					}
					Expect(fakeClient.Create(ctx, existingSecret)).To(Succeed())

					By("generating secret")
					secret, err := m.Generate(ctx, config)
					Expect(err).NotTo(HaveOccurred())

					By("verifying new tokens were generated")
					staticToken, err := secretutils.LoadStaticTokenFromCSV("", secret.Data[secretutils.DataKeyStaticTokenCSV])
					Expect(err).NotTo(HaveOccurred())
					for _, token := range staticToken.Tokens {
						if token.Username == user1 {
							Expect(token.Token).To(Equal(oldUser1Token))
						}
						if token.Username == user2 {
							Expect(token.Token).To(Equal(oldUser2Token))
						}
					}
				})
			})

			Context("ssh-keypair", func() {
				var (
					oldData = map[string][]byte{
						"id_rsa":     []byte("private-key"),
						"id_rsa.pub": []byte("public key"),
					}
					config *secretutils.RSASecretConfig
				)

				BeforeEach(func() {
					config = &secretutils.RSASecretConfig{
						Name:       "ssh-keypair",
						Bits:       4096,
						UsedForSSH: true,
					}
				})

				It("should generate a new ssh keypair if old secret does not exist", func() {
					By("generating secret")
					secret, err := m.Generate(ctx, config)
					Expect(err).NotTo(HaveOccurred())

					By("verifying new keypair was generated")
					Expect(secret.Data).NotTo(Equal(oldData))
				})

				It("should keep the existing ssh keypair if old secret still exists", func() {
					By("creating existing secret with old password")
					existingSecret := &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "ssh-keypair",
							Namespace: namespace,
						},
						Type: corev1.SecretTypeOpaque,
						Data: oldData,
					}
					Expect(fakeClient.Create(ctx, existingSecret)).To(Succeed())

					By("generating secret")
					secret, err := m.Generate(ctx, config)
					Expect(err).NotTo(HaveOccurred())

					By("verifying old password was kept")
					Expect(secret.Data).To(Equal(oldData))
				})

				It("should make the manager adopt the old ssh keypair if it exists", func() {
					By("creating existing secret with old password")
					existingSecret := &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "ssh-keypair",
							Namespace: namespace,
						},
						Type: corev1.SecretTypeOpaque,
						Data: oldData,
					}
					Expect(fakeClient.Create(ctx, existingSecret)).To(Succeed())

					By("creating existing old secret")
					existingOldSecret := &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "ssh-keypair.old",
							Namespace: namespace,
						},
						Type: corev1.SecretTypeOpaque,
					}
					Expect(fakeClient.Create(ctx, existingOldSecret)).To(Succeed())

					By("generating secret")
					_, err := m.Generate(ctx, config)
					Expect(err).NotTo(HaveOccurred())

					By("verifying old ssh keypair was adopted")
					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(existingOldSecret), existingOldSecret)).To(Succeed())
					Expect(existingOldSecret.Immutable).To(PointTo(BeTrue()))
					Expect(existingOldSecret.Labels).To(Equal(map[string]string{
						"name":                          "ssh-keypair",
						"managed-by":                    "secrets-manager",
						"persist":                       "true",
						"last-rotation-initiation-time": "",
					}))
				})
			})
		})
	})
})

func expectSecretWasCreated(ctx context.Context, fakeClient client.Client, obj *corev1.Secret) {
	secret := obj.DeepCopy()
	secret.TypeMeta = metav1.TypeMeta{APIVersion: "v1", Kind: "Secret"}

	foundSecret := &corev1.Secret{}
	Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(secret), foundSecret)).To(Succeed())

	Expect(foundSecret).To(Equal(secret))
}

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

var _ = Describe("Generate", func() {
	var (
		ctx       = context.TODO()
		namespace = "shoot--foo--bar"

		m          *manager
		fakeClient client.Client
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).Build()
		m = New(logr.Discard(), fakeClient, namespace, nil).(*manager)
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
				m = New(logr.Discard(), fakeClient, namespace, map[string]time.Time{name: time.Now()}).(*manager)

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
					"managed-by": "secrets-manager",
					"bundle-for": name,
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
				m = New(logr.Discard(), fakeClient, namespace, map[string]time.Time{name: time.Now()}).(*manager)
				newSecret, err := m.Generate(ctx, config, Rotate(KeepOld))
				Expect(err).NotTo(HaveOccurred())
				expectSecretWasCreated(ctx, fakeClient, newSecret)

				By("finding created bundle secret")
				secretList := &corev1.SecretList{}
				Expect(fakeClient.List(ctx, secretList, client.InNamespace(namespace), client.MatchingLabels{
					"managed-by": "secrets-manager",
					"bundle-for": name,
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
				m = New(logr.Discard(), fakeClient, namespace, map[string]time.Time{caName: time.Now()}).(*manager)
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
				m = New(logr.Discard(), fakeClient, namespace, map[string]time.Time{caName: time.Now()}).(*manager)
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
	})
})

func expectSecretWasCreated(ctx context.Context, fakeClient client.Client, obj *corev1.Secret) {
	secret := obj.DeepCopy()
	secret.TypeMeta = metav1.TypeMeta{APIVersion: "v1", Kind: "Secret"}

	foundSecret := &corev1.Secret{}
	Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(secret), foundSecret)).To(Succeed())

	Expect(foundSecret).To(Equal(secret))
}

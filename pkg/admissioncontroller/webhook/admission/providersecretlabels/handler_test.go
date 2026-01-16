// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package providersecretlabels_test

import (
	"context"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"

	. "github.com/gardener/gardener/pkg/admissioncontroller/webhook/admission/providersecretlabels"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/logger"
)

var _ = Describe("handler", func() {
	var (
		ctx context.Context
		log logr.Logger

		fakeClient client.Client

		namespace            string
		provider1, provider2 string
	)

	BeforeEach(func() {
		ctx = context.Background()
		log = logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, logzap.WriteTo(GinkgoWriter))

		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()

		namespace = "test"
		provider1, provider2 = "provider1", "provider2"
	})

	Context("for Secrets", func() {
		var (
			handler *SecretHandler

			secret             *corev1.Secret
			secretBinding      *gardencorev1beta1.SecretBinding
			credentialsBinding *securityv1alpha1.CredentialsBinding
		)

		BeforeEach(func() {
			handler = &SecretHandler{Handler: &Handler{
				Logger: log,
				Client: fakeClient,
			}}

			secret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret",
					Namespace: namespace,
				},
			}

			secretBinding = &gardencorev1beta1.SecretBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-secret-binding",
					Namespace: namespace,
				},
				SecretRef: corev1.SecretReference{
					Name:      secret.Name,
					Namespace: secret.Namespace,
				},
				Provider: &gardencorev1beta1.SecretBindingProvider{
					Type: provider1,
				},
			}

			credentialsBinding = &securityv1alpha1.CredentialsBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-credentials-binding",
					Namespace: "another-namespace",
				},
				CredentialsRef: corev1.ObjectReference{
					APIVersion: corev1.SchemeGroupVersion.String(),
					Kind:       "Secret",
					Name:       secret.Name,
					Namespace:  secret.Namespace,
				},
				Provider: securityv1alpha1.CredentialsBindingProvider{
					Type: provider2,
				},
			}
		})

		It("should set the provider label based on the available credential and secret bindings", func() {
			Expect(fakeClient.Create(ctx, secretBinding)).To(Succeed())
			Expect(fakeClient.Create(ctx, credentialsBinding)).To(Succeed())

			Expect(handler.Default(ctx, secret)).To(Succeed())

			Expect(secret.Labels).To(HaveKeyWithValue("provider.shoot.gardener.cloud/provider1", "true"))
			Expect(secret.Labels).To(HaveKeyWithValue("provider.shoot.gardener.cloud/provider2", "true"))
		})

		It("should remove undesired provider type", func() {
			Expect(fakeClient.Create(ctx, secretBinding)).To(Succeed())
			Expect(fakeClient.Create(ctx, credentialsBinding)).To(Succeed())

			secret.Labels = map[string]string{
				"provider.shoot.gardener.cloud/provider1": "true",
				"provider.shoot.gardener.cloud/provider2": "true",
				"provider.shoot.gardener.cloud/provider3": "true",
			}

			Expect(handler.Default(ctx, secret)).To(Succeed())

			Expect(secret.Labels).To(HaveKeyWithValue("provider.shoot.gardener.cloud/provider1", "true"))
			Expect(secret.Labels).To(HaveKeyWithValue("provider.shoot.gardener.cloud/provider2", "true"))
			Expect(secret.Labels).NotTo(HaveKey("provider.shoot.gardener.cloud/provider3"))
		})

		It("should add the missing provider and delete the wrong one", func() {
			Expect(fakeClient.Create(ctx, secretBinding)).To(Succeed())
			Expect(fakeClient.Create(ctx, credentialsBinding)).To(Succeed())

			secret.Labels = map[string]string{
				"provider.shoot.gardener.cloud/provider1": "true",
				"provider.shoot.gardener.cloud/provider3": "true",
			}

			Expect(handler.Default(ctx, secret)).To(Succeed())

			Expect(secret.Labels).To(HaveKeyWithValue("provider.shoot.gardener.cloud/provider1", "true"))
			Expect(secret.Labels).To(HaveKeyWithValue("provider.shoot.gardener.cloud/provider2", "true"))
			Expect(secret.Labels).NotTo(HaveKey("provider.shoot.gardener.cloud/provider3"))
		})

		It("should not add provider labels when secret is unreferenced", func() {
			Expect(handler.Default(ctx, secret)).To(Succeed())
			Expect(secret.Labels).To(BeEmpty())
		})
	})

	Context("for InternalSecrets", func() {
		var (
			handler *InternalSecretHandler

			internalSecret      *gardencorev1beta1.InternalSecret
			credentialsBinding1 *securityv1alpha1.CredentialsBinding
			credentialsBinding2 *securityv1alpha1.CredentialsBinding
		)

		BeforeEach(func() {
			handler = &InternalSecretHandler{Handler: &Handler{
				Logger: log,
				Client: fakeClient,
			}}

			internalSecret = &gardencorev1beta1.InternalSecret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-internal-secret",
					Namespace: namespace,
				},
			}

			credentialsBinding1 = &securityv1alpha1.CredentialsBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-credentials-binding-1",
					Namespace: "another-namespace",
				},
				CredentialsRef: corev1.ObjectReference{
					APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
					Kind:       "InternalSecret",
					Name:       internalSecret.Name,
					Namespace:  internalSecret.Namespace,
				},
				Provider: securityv1alpha1.CredentialsBindingProvider{
					Type: provider1,
				},
			}
			credentialsBinding2 = &securityv1alpha1.CredentialsBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-credentials-binding",
					Namespace: "another-namespace",
				},
				CredentialsRef: corev1.ObjectReference{
					APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
					Kind:       "InternalSecret",
					Name:       internalSecret.Name,
					Namespace:  internalSecret.Namespace,
				},
				Provider: securityv1alpha1.CredentialsBindingProvider{
					Type: provider2,
				},
			}
		})

		It("should set the provider label based on the available credential bindings", func() {
			Expect(fakeClient.Create(ctx, credentialsBinding1)).To(Succeed())
			Expect(fakeClient.Create(ctx, credentialsBinding2)).To(Succeed())

			Expect(handler.Default(ctx, internalSecret)).To(Succeed())

			Expect(internalSecret.Labels).To(HaveKeyWithValue("provider.shoot.gardener.cloud/provider1", "true"))
			Expect(internalSecret.Labels).To(HaveKeyWithValue("provider.shoot.gardener.cloud/provider2", "true"))
		})

		It("should remove undesired provider type", func() {
			Expect(fakeClient.Create(ctx, credentialsBinding1)).To(Succeed())
			Expect(fakeClient.Create(ctx, credentialsBinding2)).To(Succeed())

			internalSecret.Labels = map[string]string{
				"provider.shoot.gardener.cloud/provider1": "true",
				"provider.shoot.gardener.cloud/provider2": "true",
				"provider.shoot.gardener.cloud/provider3": "true",
			}

			Expect(handler.Default(ctx, internalSecret)).To(Succeed())

			Expect(internalSecret.Labels).To(HaveKeyWithValue("provider.shoot.gardener.cloud/provider1", "true"))
			Expect(internalSecret.Labels).To(HaveKeyWithValue("provider.shoot.gardener.cloud/provider2", "true"))
			Expect(internalSecret.Labels).NotTo(HaveKey("provider.shoot.gardener.cloud/provider3"))
		})

		It("should add the missing provider and delete the wrong one", func() {
			Expect(fakeClient.Create(ctx, credentialsBinding1)).To(Succeed())
			Expect(fakeClient.Create(ctx, credentialsBinding2)).To(Succeed())

			internalSecret.Labels = map[string]string{
				"provider.shoot.gardener.cloud/provider1": "true",
				"provider.shoot.gardener.cloud/provider3": "true",
			}

			Expect(handler.Default(ctx, internalSecret)).To(Succeed())

			Expect(internalSecret.Labels).To(HaveKeyWithValue("provider.shoot.gardener.cloud/provider1", "true"))
			Expect(internalSecret.Labels).To(HaveKeyWithValue("provider.shoot.gardener.cloud/provider2", "true"))
			Expect(internalSecret.Labels).NotTo(HaveKey("provider.shoot.gardener.cloud/provider3"))
		})

		It("should not add provider labels when internalSecret is unreferenced", func() {
			Expect(handler.Default(ctx, internalSecret)).To(Succeed())
			Expect(internalSecret.Labels).To(BeEmpty())
		})
	})
})

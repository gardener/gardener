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

		log        logr.Logger
		fakeClient client.Client
		handler    *Handler

		namespace            string
		provider1, provider2 string
		secret               *corev1.Secret
		secretBinding        *gardencorev1beta1.SecretBinding
		credentialsBinding   *securityv1alpha1.CredentialsBinding
	)

	BeforeEach(func() {
		ctx = context.Background()
		log = logger.MustNewZapLogger(logger.DebugLevel, logger.FormatJSON, logzap.WriteTo(GinkgoWriter))

		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()
		handler = &Handler{
			Logger: log,
			Client: fakeClient,
		}

		namespace = "test"
		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-secret",
				Namespace: namespace,
			},
		}

		provider1 = "provider1"
		provider2 = "provider2"

		secretBinding = &gardencorev1beta1.SecretBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-secret-binding",
				Namespace: namespace,
			},
			SecretRef: corev1.SecretReference{
				Name:      "test-secret",
				Namespace: namespace,
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
				Name:       "test-secret",
				Namespace:  namespace,
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

// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubernetes_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/clock"
	testclock "k8s.io/utils/clock/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/secrets"
	"github.com/gardener/gardener/pkg/utils/test"
)

var _ = Describe("ResolveRegistryCABundle", func() {
	var (
		ctx          = context.TODO()
		fakeClient   client.Client
		namespace    = "garden"
		validCertPEM string
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).Build()

		cert, err := (&secrets.CertificateSecretConfig{
			Name:       "test",
			CommonName: "test",
			CertType:   secrets.CACert,
		}).GenerateCertificate()
		Expect(err).NotTo(HaveOccurred())
		validCertPEM = string(cert.CertificatePEM)
	})

	It("should return the inline value directly without a client call", func() {
		inline := "-----BEGIN CERTIFICATE-----\nMIIB...\n-----END CERTIFICATE-----\n"

		result, err := kubernetes.ResolveRegistryCABundle(ctx, fakeClient, nil, &inline)

		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(inline))
	})

	It("should return the bundle.crt value from the referenced secret", func() {
		Expect(fakeClient.Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "my-ca", Namespace: namespace},
			Data:       map[string][]byte{"bundle.crt": []byte(validCertPEM)},
		})).To(Succeed())

		result, err := kubernetes.ResolveRegistryCABundle(ctx, fakeClient, &corev1.SecretReference{Name: "my-ca", Namespace: namespace}, nil)

		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal(validCertPEM))
	})

	It("should return an error when the secret does not exist", func() {
		_, err := kubernetes.ResolveRegistryCABundle(ctx, fakeClient, &corev1.SecretReference{Name: "missing-secret", Namespace: namespace}, nil)

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("failed to get registry CA bundle secret"))
	})

	It("should return an error when bundle.crt key is absent from the secret", func() {
		Expect(fakeClient.Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "no-key-secret", Namespace: namespace},
			Data:       map[string][]byte{"other-key": []byte("value")},
		})).To(Succeed())

		_, err := kubernetes.ResolveRegistryCABundle(ctx, fakeClient, &corev1.SecretReference{Name: "no-key-secret", Namespace: namespace}, nil)

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("bundle.crt"))
	})

	It("should return an error when bundle.crt key is present but empty", func() {
		Expect(fakeClient.Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "empty-key-secret", Namespace: namespace},
			Data:       map[string][]byte{"bundle.crt": {}},
		})).To(Succeed())

		_, err := kubernetes.ResolveRegistryCABundle(ctx, fakeClient, &corev1.SecretReference{Name: "empty-key-secret", Namespace: namespace}, nil)

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("has empty key"))
	})

	It("should return an error when bundle.crt contains an invalid certificate", func() {
		Expect(fakeClient.Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "invalid-cert-secret", Namespace: namespace},
			Data:       map[string][]byte{"bundle.crt": []byte("not-a-cert")},
		})).To(Succeed())

		_, err := kubernetes.ResolveRegistryCABundle(ctx, fakeClient, &corev1.SecretReference{Name: "invalid-cert-secret", Namespace: namespace}, nil)

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("invalid certificate"))
	})

	It("should return an error when bundle.crt certificate expires within 7 days", func() {
		validity := 6 * 24 * time.Hour
		cert, err := (&secrets.CertificateSecretConfig{
			Name:       "test",
			CommonName: "test",
			CertType:   secrets.CACert,
			Validity:   &validity,
		}).GenerateCertificate()
		Expect(err).NotTo(HaveOccurred())

		Expect(fakeClient.Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "expiring-cert-secret", Namespace: namespace},
			Data:       map[string][]byte{"bundle.crt": cert.CertificatePEM},
		})).To(Succeed())

		_, err = kubernetes.ResolveRegistryCABundle(ctx, fakeClient, &corev1.SecretReference{Name: "expiring-cert-secret", Namespace: namespace}, nil)

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("either expired or expires within 7 days"))
	})

	It("should return an error when bundle.crt certificate is already expired", func() {
		DeferCleanup(test.WithVar(&secrets.Clock, testclock.NewFakeClock(time.Now().Add(-8*24*time.Hour))))

		validity := 24 * time.Hour
		cert, err := (&secrets.CertificateSecretConfig{
			Name:       "test",
			CommonName: "test",
			CertType:   secrets.CACert,
			Validity:   &validity,
		}).GenerateCertificate()
		Expect(err).NotTo(HaveOccurred())

		secrets.Clock = clock.RealClock{}

		Expect(fakeClient.Create(ctx, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "expired-cert-secret", Namespace: namespace},
			Data:       map[string][]byte{"bundle.crt": cert.CertificatePEM},
		})).To(Succeed())

		_, err = kubernetes.ResolveRegistryCABundle(ctx, fakeClient, &corev1.SecretReference{Name: "expired-cert-secret", Namespace: namespace}, nil)

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("either expired or expires within 7 days"))
	})
})

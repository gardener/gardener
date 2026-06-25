// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package kubernetes_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/gardener/gardener/pkg/utils/kubernetes"
)

var _ = Describe("ResolveRegistryCABundle", func() {
	var (
		ctx        = context.TODO()
		fakeClient client.Client
		namespace  = "garden"
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).Build()
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
			Data:       map[string][]byte{"bundle.crt": []byte("pem-data")},
		})).To(Succeed())

		result, err := kubernetes.ResolveRegistryCABundle(ctx, fakeClient, &corev1.SecretReference{Name: "my-ca", Namespace: namespace}, nil)

		Expect(err).NotTo(HaveOccurred())
		Expect(result).To(Equal("pem-data"))
	})

	It("should return an error when the secret does not exist", func() {
		_, err := kubernetes.ResolveRegistryCABundle(ctx, fakeClient, &corev1.SecretReference{Name: "missing-secret", Namespace: namespace}, nil)

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("missing-secret"))
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
		Expect(err.Error()).To(ContainSubstring("empty"))
	})
})

// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bootstraptoken_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	. "github.com/gardener/gardener/pkg/utils/kubernetes/bootstraptoken"
)

var _ = Describe("bootstraptoken", func() {
	var (
		ctx        = context.Background()
		fakeClient client.Client

		tokenID     = "abcdef"
		description = "test"
		validity    = time.Hour
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().Build()
	})

	Describe("#ComputeBootstrapToken", func() {
		It("should compute a new bootstrap token with a randomly generated secret", func() {
			secret, err := ComputeBootstrapToken(ctx, fakeClient, tokenID, description, validity)
			Expect(err).NotTo(HaveOccurred())

			Expect(secret.Name).To(Equal("bootstrap-token-" + tokenID))
			Expect(secret.Namespace).To(Equal("kube-system"))
			Expect(secret.Type).To(Equal(corev1.SecretTypeBootstrapToken))
			Expect(secret.Data).To(And(
				HaveKeyWithValue("token-id", Equal([]byte(tokenID))),
				HaveKeyWithValue("token-secret", HaveLen(16)),
				HaveKeyWithValue("description", Equal([]byte(description))),
				HaveKeyWithValue("expiration", Equal([]byte(metav1.Now().Add(validity).Format(time.RFC3339)))),
				HaveKeyWithValue("usage-bootstrap-authentication", Equal([]byte("true"))),
				HaveKeyWithValue("usage-bootstrap-signing", Equal([]byte("true"))),
			))

			By("Ensure it doesn't recompute the token secret")
			tokenSecret := secret.Data["token-secret"]

			secret, err = ComputeBootstrapToken(ctx, fakeClient, tokenID, description, validity)
			Expect(err).NotTo(HaveOccurred())
			Expect(secret.Data["token-secret"]).To(Equal(tokenSecret))
		})
	})

	Describe("#ComputeBootstrapTokenWithSecret", func() {
		var tokenSecret = "1234567890abcdef"

		It("should compute a new bootstrap token with a randomly generated secret", func() {
			secret, err := ComputeBootstrapTokenWithSecret(ctx, fakeClient, tokenID, tokenSecret, description, validity)
			Expect(err).NotTo(HaveOccurred())

			Expect(secret.Name).To(Equal("bootstrap-token-" + tokenID))
			Expect(secret.Namespace).To(Equal("kube-system"))
			Expect(secret.Type).To(Equal(corev1.SecretTypeBootstrapToken))
			Expect(secret.Data).To(And(
				HaveKeyWithValue("token-id", Equal([]byte(tokenID))),
				HaveKeyWithValue("token-secret", Equal([]byte(tokenSecret))),
				HaveKeyWithValue("description", Equal([]byte(description))),
				HaveKeyWithValue("expiration", Equal([]byte(metav1.Now().Add(validity).Format(time.RFC3339)))),
				HaveKeyWithValue("usage-bootstrap-authentication", Equal([]byte("true"))),
				HaveKeyWithValue("usage-bootstrap-signing", Equal([]byte("true"))),
			))

			By("Ensure it doesn't recompute the token secret")
			secret, err = ComputeBootstrapTokenWithSecret(ctx, fakeClient, tokenID, tokenSecret, description, validity)
			Expect(err).NotTo(HaveOccurred())
			Expect(secret.Data["token-secret"]).To(Equal([]byte(tokenSecret)))
		})
	})

	Describe("#FromSecretData", func() {
		It("should return the expected token", func() {
			Expect(FromSecretData(map[string][]byte{
				"token-id":     []byte("foo"),
				"token-secret": []byte("bar"),
			})).To(Equal("foo.bar"))
		})
	})

	Describe("#TokenID", func() {
		const (
			namespace = "bar"
			name      = "baz"
		)

		It("should compute the expected id (w/o namespace", func() {
			Expect(TokenID(metav1.ObjectMeta{Name: name})).To(Equal("baa5a0"))
		})

		It("should compute the expected id (w/ namespace", func() {
			Expect(TokenID(metav1.ObjectMeta{Name: name, Namespace: namespace})).To(Equal("cc19de"))
		})
	})
})

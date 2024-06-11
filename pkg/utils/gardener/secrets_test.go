// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardener_test

import (
	"context"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"golang.org/x/crypto/bcrypt"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	. "github.com/gardener/gardener/pkg/utils/gardener"
)

var _ = Describe("Secrets", func() {
	Describe("ReplicateGlobalMonitoringSecret", func() {
		var (
			ctx        = context.Background()
			fakeClient client.Client

			prefix                 = "prefix"
			namespace              = "namespace"
			globalMonitoringSecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "global-monitoring-secret",
					Namespace:   "foo",
					Labels:      map[string]string{"bar": "baz"},
					Annotations: map[string]string{"baz": "foo"},
				},
				Type:      corev1.SecretTypeOpaque,
				Immutable: ptr.To(false),
				Data:      map[string][]byte{"username": []byte("bar"), "password": []byte("baz")},
			}
		)

		BeforeEach(func() {
			fakeClient = fakeclient.NewClientBuilder().Build()
		})

		It("should replicate the secret", func() {
			assertions := func(secret *corev1.Secret) {
				Expect(secret.Labels).To(HaveKeyWithValue("gardener.cloud/purpose", "global-monitoring-secret-replica"))
				Expect(secret.Type).To(Equal(globalMonitoringSecret.Type))
				Expect(secret.Immutable).To(Equal(globalMonitoringSecret.Immutable))
				for k, v := range globalMonitoringSecret.Data {
					Expect(secret.Data).To(HaveKeyWithValue(k, v), "have key "+k+" with value "+string(v))
				}
				hashedPassword := strings.TrimPrefix(string(secret.Data["auth"]), string(secret.Data["username"])+":")
				Expect(bcrypt.CompareHashAndPassword([]byte(hashedPassword), secret.Data["password"])).To(Succeed())
			}

			secret, err := ReplicateGlobalMonitoringSecret(ctx, fakeClient, prefix, namespace, globalMonitoringSecret)
			Expect(err).NotTo(HaveOccurred())
			assertions(secret)

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())
			assertions(secret)
		})
	})
})

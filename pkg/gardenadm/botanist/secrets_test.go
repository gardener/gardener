// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	. "github.com/gardener/gardener/pkg/gardenadm/botanist"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	botanistpkg "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
)

var _ = Describe("Secrets", func() {
	var (
		ctx context.Context

		fakeClient1 client.Client
		fakeClient2 client.Client

		b *AutonomousBotanist
	)

	BeforeEach(func() {
		ctx = context.Background()

		fakeClient1 = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		fakeClient2 = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()

		b = &AutonomousBotanist{
			Botanist: &botanistpkg.Botanist{
				Operation: &operation.Operation{
					SeedClientSet: fakekubernetes.
						NewClientSetBuilder().
						WithClient(fakeClient1).
						WithRESTConfig(&rest.Config{}).
						Build(),
					Shoot: &shootpkg.Shoot{
						ControlPlaneNamespace: "kube-system",
					},
				},
			},
		}
	})

	Describe("#MigrateSecrets", func() {
		It("should copy all secrets from kube-system", func() {
			var (
				secret1 = &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "s1", Namespace: "kube-system", Finalizers: []string{"hugo"}},
					Type:       corev1.SecretTypeOpaque,
					Data:       map[string][]byte{"foo": []byte("bar")},
				}
				secret2 = &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "s1", Namespace: "default"},
					Type:       corev1.SecretTypeOpaque,
					Data:       map[string][]byte{"bar": []byte("foo")},
				}
				secret3 = &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "s2", Namespace: "kube-system", OwnerReferences: []metav1.OwnerReference{{}}},
					Type:       corev1.SecretTypeOpaque,
					Data:       map[string][]byte{"baz": []byte("bar")},
				}

				secretList = &corev1.SecretList{}
			)

			Expect(fakeClient1.Create(ctx, secret1)).To(Succeed())
			Expect(fakeClient1.Create(ctx, secret2)).To(Succeed())
			Expect(fakeClient1.Create(ctx, secret3)).To(Succeed())

			Expect(fakeClient2.List(ctx, secretList)).To(Succeed())
			Expect(secretList.Items).To(BeEmpty())

			Expect(b.MigrateSecrets(ctx, fakeClient1, fakeClient2)).To(Succeed())

			Expect(fakeClient2.List(ctx, secretList)).To(Succeed())
			Expect(secretList.Items).To(HaveExactElements(
				corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "s1", Namespace: "kube-system", ResourceVersion: "1"},
					Type:       corev1.SecretTypeOpaque,
					Data:       map[string][]byte{"foo": []byte("bar")},
				},
				corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "s2", Namespace: "kube-system", ResourceVersion: "1"},
					Type:       corev1.SecretTypeOpaque,
					Data:       map[string][]byte{"baz": []byte("bar")},
				},
			))
		})
	})
})

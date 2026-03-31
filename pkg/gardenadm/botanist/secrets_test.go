// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package botanist_test

import (
	"context"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/afero"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	fakekubernetes "github.com/gardener/gardener/pkg/client/kubernetes/fake"
	. "github.com/gardener/gardener/pkg/gardenadm/botanist"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	botanistpkg "github.com/gardener/gardener/pkg/gardenlet/operation/botanist"
	shootpkg "github.com/gardener/gardener/pkg/gardenlet/operation/shoot"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

var _ = Describe("Secrets", func() {
	var (
		ctx context.Context

		fakeClient1 client.Client
		fakeClient2 client.Client

		b *GardenadmBotanist
	)

	BeforeEach(func() {
		ctx = context.Background()

		fakeClient1 = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		fakeClient2 = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()

		b = &GardenadmBotanist{
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

		It("should tolerate already existing secrets in the target", func() {
			existingSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "s1", Namespace: "kube-system"},
				Type:       corev1.SecretTypeOpaque,
				Data:       map[string][]byte{"old": []byte("data")},
			}
			Expect(fakeClient2.Create(ctx, existingSecret)).To(Succeed())

			sourceSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "s1", Namespace: "kube-system"},
				Type:       corev1.SecretTypeOpaque,
				Data:       map[string][]byte{"foo": []byte("bar")},
			}
			Expect(fakeClient1.Create(ctx, sourceSecret)).To(Succeed())

			Expect(b.MigrateSecrets(ctx, fakeClient1, fakeClient2)).To(Succeed())
		})
	})

	Describe("#PersistBootstrapSecrets", func() {
		const configDir = "/config"

		var fakeSeedClient client.Client

		BeforeEach(func() {
			fakeSeedClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()

			shoot := &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{Name: "my-shoot", Namespace: "garden-my-project"},
			}

			b.FS = afero.Afero{Fs: afero.NewMemMapFs()}
			b.SeedClientSet = fakekubernetes.
				NewClientSetBuilder().
				WithClient(fakeSeedClient).
				WithRESTConfig(&rest.Config{}).
				Build()
			b.Shoot.SetInfo(shoot)
			b.Shoot.ControlPlaneNamespace = "kube-system"
		})

		It("should persist all secrets into a ShootState file", func() {
			caSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ca-cluster",
					Namespace: "kube-system",
					Labels: map[string]string{
						secretsmanager.LabelKeyPersist:   secretsmanager.LabelValueTrue,
						secretsmanager.LabelKeyManagedBy: secretsmanager.LabelValueSecretsManager,
					},
				},
				Type: corev1.SecretTypeTLS,
				Data: map[string][]byte{"tls.crt": []byte("cert"), "tls.key": []byte("key")},
			}
			derivedSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kube-apiserver-server",
					Namespace: "kube-system",
					Labels: map[string]string{
						secretsmanager.LabelKeyManagedBy: secretsmanager.LabelValueSecretsManager,
					},
				},
				Type: corev1.SecretTypeTLS,
				Data: map[string][]byte{"tls.crt": []byte("server-cert"), "tls.key": []byte("server-key")},
			}
			emptyDataSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "empty-data", Namespace: "kube-system"},
			}
			otherNSSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "other", Namespace: "default"},
				Data:       map[string][]byte{"key": []byte("val")},
			}

			Expect(fakeSeedClient.Create(ctx, caSecret)).To(Succeed())
			Expect(fakeSeedClient.Create(ctx, derivedSecret)).To(Succeed())
			Expect(fakeSeedClient.Create(ctx, emptyDataSecret)).To(Succeed())
			Expect(fakeSeedClient.Create(ctx, otherNSSecret)).To(Succeed())

			Expect(b.PersistBootstrapSecrets(ctx, configDir)).To(Succeed())

			fileBytes, err := b.FS.ReadFile(filepath.Join(configDir, "bootstrap-shootstate.yaml"))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(fileBytes)).To(ContainSubstring("ca-cluster"))
			Expect(string(fileBytes)).To(ContainSubstring("kube-apiserver-server"))
			Expect(string(fileBytes)).NotTo(ContainSubstring("other"))
			Expect(string(fileBytes)).NotTo(ContainSubstring("empty-data"))
		})
	})

	Describe("#CleanupBootstrapSecrets", func() {
		const configDir = "/config"

		BeforeEach(func() {
			b.FS = afero.Afero{Fs: afero.NewMemMapFs()}
		})

		It("should remove the bootstrap ShootState file", func() {
			Expect(b.FS.WriteFile(filepath.Join(configDir, "bootstrap-shootstate.yaml"), []byte("test"), 0600)).To(Succeed())

			Expect(b.CleanupBootstrapSecrets(configDir)).To(Succeed())

			exists, err := b.FS.Exists(filepath.Join(configDir, "bootstrap-shootstate.yaml"))
			Expect(err).NotTo(HaveOccurred())
			Expect(exists).To(BeFalse())
		})

		It("should not fail if the file does not exist", func() {
			Expect(b.CleanupBootstrapSecrets(configDir)).To(Succeed())
		})
	})
})

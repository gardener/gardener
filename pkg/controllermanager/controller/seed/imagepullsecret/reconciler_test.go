// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package imagepullsecret_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/controllermanager/controller/seed/imagepullsecret"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Reconciler", func() {
	var (
		ctx        context.Context
		fakeClient client.Client
		reconciler *Reconciler
	)

	BeforeEach(func() {
		ctx = context.Background()
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()
		reconciler = &Reconciler{
			Client:          fakeClient,
			GardenNamespace: v1beta1constants.GardenNamespace,
		}
	})

	// seedNs creates a namespace with the gardener.cloud/role=seed label.
	seedNs := func(seedName string) *corev1.Namespace {
		return &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name:   gardenerutils.ComputeGardenNamespace(seedName),
				Labels: map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleSeed},
			},
		}
	}

	// pullSecret builds a secret in the garden namespace with the image-pull-secret role.
	pullSecret := func(name string, seeds ...string) *corev1.Secret {
		s := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: v1beta1constants.GardenNamespace,
				Labels:    map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleImagePullSecret},
			},
			Type: corev1.SecretTypeDockerConfigJson,
			Data: map[string][]byte{corev1.DockerConfigJsonKey: []byte(`{"auths":{}}`)},
		}
		if len(seeds) > 0 {
			s.Annotations = map[string]string{
				v1beta1constants.AnnotationImagePullSecretSeedNames: joinSeeds(seeds),
			}
		}
		return s
	}

	Describe("#Reconcile", func() {
		Context("when the secret does not exist", func() {
			It("should return without error when no seed namespaces exist", func() {
				result, err := reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: client.ObjectKey{Namespace: v1beta1constants.GardenNamespace, Name: "gone"},
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))
			})

			It("should delete the secret from all seed namespaces", func() {
				ns1 := seedNs("seed-a")
				ns2 := seedNs("seed-b")
				Expect(fakeClient.Create(ctx, ns1)).To(Succeed())
				Expect(fakeClient.Create(ctx, ns2)).To(Succeed())

				// Pre-populate copies so we can verify deletion.
				for _, ns := range []string{ns1.Name, ns2.Name} {
					Expect(fakeClient.Create(ctx, &corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{Name: "pull", Namespace: ns},
					})).To(Succeed())
				}

				_, err := reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: client.ObjectKey{Namespace: v1beta1constants.GardenNamespace, Name: "pull"},
				})
				Expect(err).NotTo(HaveOccurred())

				for _, ns := range []string{ns1.Name, ns2.Name} {
					Expect(fakeClient.Get(ctx, client.ObjectKey{Namespace: ns, Name: "pull"}, &corev1.Secret{})).
						To(BeNotFoundError())
				}
			})

			It("should not fail if the copies were already absent", func() {
				Expect(fakeClient.Create(ctx, seedNs("seed-a"))).To(Succeed())

				_, err := reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: client.ObjectKey{Namespace: v1beta1constants.GardenNamespace, Name: "pull"},
				})
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when the secret exists", func() {
			It("should copy the secret to seed namespaces listed in the annotation", func() {
				nsA := seedNs("seed-a")
				nsB := seedNs("seed-b")
				Expect(fakeClient.Create(ctx, nsA)).To(Succeed())
				Expect(fakeClient.Create(ctx, nsB)).To(Succeed())

				secret := pullSecret("my-pull", "seed-a")
				Expect(fakeClient.Create(ctx, secret)).To(Succeed())

				result, err := reconciler.Reconcile(ctx, reconcile.Request{
					NamespacedName: client.ObjectKeyFromObject(secret),
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))

				// Copied to seed-a.
				copied := &corev1.Secret{}
				Expect(fakeClient.Get(ctx, client.ObjectKey{Namespace: nsA.Name, Name: secret.Name}, copied)).To(Succeed())
				Expect(copied.Data).To(Equal(secret.Data))
				Expect(copied.Labels).To(Equal(secret.Labels))

				// Not copied to seed-b (not in annotation).
				Expect(fakeClient.Get(ctx, client.ObjectKey{Namespace: nsB.Name, Name: secret.Name}, &corev1.Secret{})).
					To(BeNotFoundError())
			})

			It("should copy the secret to all seeds listed in the annotation", func() {
				nsA := seedNs("seed-a")
				nsB := seedNs("seed-b")
				Expect(fakeClient.Create(ctx, nsA)).To(Succeed())
				Expect(fakeClient.Create(ctx, nsB)).To(Succeed())

				secret := pullSecret("my-pull", "seed-a", "seed-b")
				Expect(fakeClient.Create(ctx, secret)).To(Succeed())

				_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(secret)})
				Expect(err).NotTo(HaveOccurred())

				for _, ns := range []string{nsA.Name, nsB.Name} {
					s := &corev1.Secret{}
					Expect(fakeClient.Get(ctx, client.ObjectKey{Namespace: ns, Name: secret.Name}, s)).To(Succeed())
					Expect(s.Data).To(Equal(secret.Data))
				}
			})

			It("should remove stale copies from seeds removed from the annotation", func() {
				nsA := seedNs("seed-a")
				nsB := seedNs("seed-b")
				Expect(fakeClient.Create(ctx, nsA)).To(Succeed())
				Expect(fakeClient.Create(ctx, nsB)).To(Succeed())

				// Pre-existing copy in seed-b that should be cleaned up.
				Expect(fakeClient.Create(ctx, &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "my-pull", Namespace: nsB.Name},
				})).To(Succeed())

				// Secret now only lists seed-a.
				secret := pullSecret("my-pull", "seed-a")
				Expect(fakeClient.Create(ctx, secret)).To(Succeed())

				_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(secret)})
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeClient.Get(ctx, client.ObjectKey{Namespace: nsA.Name, Name: "my-pull"}, &corev1.Secret{})).To(Succeed())
				Expect(fakeClient.Get(ctx, client.ObjectKey{Namespace: nsB.Name, Name: "my-pull"}, &corev1.Secret{})).
					To(BeNotFoundError())
			})

			It("should remove copies from all seeds when annotation is empty", func() {
				nsA := seedNs("seed-a")
				Expect(fakeClient.Create(ctx, nsA)).To(Succeed())
				Expect(fakeClient.Create(ctx, &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "my-pull", Namespace: nsA.Name},
				})).To(Succeed())

				// Secret has no annotation — no seed should receive a copy.
				secret := pullSecret("my-pull")
				Expect(fakeClient.Create(ctx, secret)).To(Succeed())

				_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(secret)})
				Expect(err).NotTo(HaveOccurred())

				Expect(fakeClient.Get(ctx, client.ObjectKey{Namespace: nsA.Name, Name: "my-pull"}, &corev1.Secret{})).
					To(BeNotFoundError())
			})

			It("should ignore annotation entries for non-existent seed namespaces", func() {
				// Only seed-a namespace exists; seed-b is listed in the annotation but absent.
				nsA := seedNs("seed-a")
				Expect(fakeClient.Create(ctx, nsA)).To(Succeed())

				secret := pullSecret("my-pull", "seed-a", "seed-b")
				Expect(fakeClient.Create(ctx, secret)).To(Succeed())

				_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(secret)})
				Expect(err).NotTo(HaveOccurred())

				// seed-a got the copy.
				Expect(fakeClient.Get(ctx, client.ObjectKey{Namespace: nsA.Name, Name: "my-pull"}, &corev1.Secret{})).To(Succeed())

				// No copy was created for seed-b (its namespace doesn't exist).
				seedBNs := gardenerutils.ComputeGardenNamespace("seed-b")
				Expect(fakeClient.Get(ctx, client.ObjectKey{Namespace: seedBNs, Name: "my-pull"}, &corev1.Secret{})).
					To(BeNotFoundError())
			})

			It("should update an existing copy when the secret data changes", func() {
				nsA := seedNs("seed-a")
				Expect(fakeClient.Create(ctx, nsA)).To(Succeed())
				Expect(fakeClient.Create(ctx, &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "my-pull", Namespace: nsA.Name,
						Labels: map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleImagePullSecret}},
					Data: map[string][]byte{corev1.DockerConfigJsonKey: []byte(`{"auths":{"old":{}}}`)},
				})).To(Succeed())

				secret := pullSecret("my-pull", "seed-a")
				Expect(fakeClient.Create(ctx, secret)).To(Succeed())

				_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(secret)})
				Expect(err).NotTo(HaveOccurred())

				updated := &corev1.Secret{}
				Expect(fakeClient.Get(ctx, client.ObjectKey{Namespace: nsA.Name, Name: "my-pull"}, updated)).To(Succeed())
				Expect(updated.Data).To(Equal(secret.Data))
			})
		})
	})
})

// joinSeeds joins seed names with a comma, as the annotation expects.
func joinSeeds(seeds []string) string {
	result := ""
	for i, s := range seeds {
		if i > 0 {
			result += ","
		}
		result += s
	}
	return result
}

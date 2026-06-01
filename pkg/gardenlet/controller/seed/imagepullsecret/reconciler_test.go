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
	. "github.com/gardener/gardener/pkg/gardenlet/controller/seed/imagepullsecret"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Reconciler", func() {
	const seedName = "test-seed"

	var (
		ctx           context.Context
		gardenClient  client.Client
		seedClient    client.Client
		reconciler    *Reconciler
		seedNamespace string
	)

	BeforeEach(func() {
		ctx = context.Background()
		seedNamespace = gardenerutils.ComputeGardenNamespace(seedName)

		gardenClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()
		seedClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		reconciler = &Reconciler{
			GardenClient: gardenClient,
			SeedClient:   seedClient,
			SeedName:     seedName,
		}
	})

	Describe("#Reconcile", func() {
		var req reconcile.Request

		BeforeEach(func() {
			req = reconcile.Request{NamespacedName: client.ObjectKey{
				Namespace: seedNamespace,
				Name:      "my-pull-secret",
			}}
		})

		Context("when secret does not exist in the seed-scoped garden namespace", func() {
			It("should delete the secret from the seed cluster's garden namespace if it exists there", func() {
				// Pre-create the secret in the seed cluster's garden namespace.
				Expect(seedClient.Create(ctx, &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-pull-secret",
						Namespace: v1beta1constants.GardenNamespace,
						Labels:    map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleImagePullSecret},
					},
				})).To(Succeed())

				result, err := reconciler.Reconcile(ctx, req)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))

				Expect(seedClient.Get(ctx, client.ObjectKey{
					Namespace: v1beta1constants.GardenNamespace,
					Name:      "my-pull-secret",
				}, &corev1.Secret{})).To(BeNotFoundError())
			})

			It("should not fail if the secret was already absent from the seed cluster", func() {
				result, err := reconciler.Reconcile(ctx, req)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))
			})

			It("should delete copies from extension namespaces", func() {
				extensionNs := &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "extension-foo",
						Labels: map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleExtension},
					},
				}
				Expect(seedClient.Create(ctx, extensionNs)).To(Succeed())
				Expect(seedClient.Create(ctx, &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "my-pull-secret", Namespace: extensionNs.Name},
				})).To(Succeed())

				result, err := reconciler.Reconcile(ctx, req)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))

				Expect(seedClient.Get(ctx, client.ObjectKey{
					Namespace: extensionNs.Name,
					Name:      "my-pull-secret",
				}, &corev1.Secret{})).To(BeNotFoundError())
			})

			It("should delete copies from shoot namespaces", func() {
				shootNs := &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "shoot--project--name",
						Labels: map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleShoot},
					},
				}
				Expect(seedClient.Create(ctx, shootNs)).To(Succeed())
				Expect(seedClient.Create(ctx, &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "my-pull-secret", Namespace: shootNs.Name},
				})).To(Succeed())

				result, err := reconciler.Reconcile(ctx, req)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))

				Expect(seedClient.Get(ctx, client.ObjectKey{
					Namespace: shootNs.Name,
					Name:      "my-pull-secret",
				}, &corev1.Secret{})).To(BeNotFoundError())
			})
		})

		Context("when secret exists in the seed-scoped garden namespace on the garden cluster", func() {
			var gardenSecret *corev1.Secret

			BeforeEach(func() {
				gardenSecret = &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-pull-secret",
						Namespace: seedNamespace,
						Labels:    map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleImagePullSecret},
					},
					Type: corev1.SecretTypeDockerConfigJson,
					Data: map[string][]byte{corev1.DockerConfigJsonKey: []byte(`{"auths":{}}`)},
				}
				Expect(gardenClient.Create(ctx, gardenSecret)).To(Succeed())
			})

			It("should sync the secret into the seed cluster's garden namespace", func() {
				result, err := reconciler.Reconcile(ctx, req)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))

				synced := &corev1.Secret{}
				Expect(seedClient.Get(ctx, client.ObjectKey{
					Namespace: v1beta1constants.GardenNamespace,
					Name:      "my-pull-secret",
				}, synced)).To(Succeed())
				Expect(synced.Type).To(Equal(corev1.SecretTypeDockerConfigJson))
				Expect(synced.Data).To(Equal(gardenSecret.Data))
				Expect(synced.Labels).To(HaveKeyWithValue(v1beta1constants.GardenRole, v1beta1constants.GardenRoleImagePullSecret))
			})

			It("should propagate the secret to extension namespaces", func() {
				extensionNs := &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "extension-foo",
						Labels: map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleExtension},
					},
				}
				Expect(seedClient.Create(ctx, extensionNs)).To(Succeed())

				result, err := reconciler.Reconcile(ctx, req)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))

				propagated := &corev1.Secret{}
				Expect(seedClient.Get(ctx, client.ObjectKey{
					Namespace: extensionNs.Name,
					Name:      "my-pull-secret",
				}, propagated)).To(Succeed())
				Expect(propagated.Data).To(Equal(gardenSecret.Data))
				Expect(propagated.Labels).To(HaveKeyWithValue(v1beta1constants.GardenRole, v1beta1constants.GardenRoleImagePullSecret))
			})

			It("should propagate the secret to shoot control plane namespaces", func() {
				shootNs := &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "shoot--project--name",
						Labels: map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleShoot},
					},
				}
				Expect(seedClient.Create(ctx, shootNs)).To(Succeed())

				result, err := reconciler.Reconcile(ctx, req)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))

				propagated := &corev1.Secret{}
				Expect(seedClient.Get(ctx, client.ObjectKey{
					Namespace: shootNs.Name,
					Name:      "my-pull-secret",
				}, propagated)).To(Succeed())
				Expect(propagated.Data).To(Equal(gardenSecret.Data))
			})

			It("should propagate to both extension and shoot namespaces", func() {
				extensionNs := &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "extension-bar",
						Labels: map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleExtension},
					},
				}
				shootNs := &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "shoot--p--s",
						Labels: map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleShoot},
					},
				}
				Expect(seedClient.Create(ctx, extensionNs)).To(Succeed())
				Expect(seedClient.Create(ctx, shootNs)).To(Succeed())

				result, err := reconciler.Reconcile(ctx, req)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))

				for _, ns := range []string{extensionNs.Name, shootNs.Name} {
					s := &corev1.Secret{}
					Expect(seedClient.Get(ctx, client.ObjectKey{Namespace: ns, Name: "my-pull-secret"}, s)).To(Succeed())
					Expect(s.Data).To(Equal(gardenSecret.Data))
				}
			})

			It("should update an existing copy in the seed cluster's garden namespace", func() {
				existing := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "my-pull-secret",
						Namespace: v1beta1constants.GardenNamespace,
						Labels:    map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleImagePullSecret},
					},
					Data: map[string][]byte{corev1.DockerConfigJsonKey: []byte(`{"auths":{"old":{}}}`)},
				}
				Expect(seedClient.Create(ctx, existing)).To(Succeed())

				result, err := reconciler.Reconcile(ctx, req)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))

				updated := &corev1.Secret{}
				Expect(seedClient.Get(ctx, client.ObjectKey{
					Namespace: v1beta1constants.GardenNamespace,
					Name:      "my-pull-secret",
				}, updated)).To(Succeed())
				Expect(updated.Data).To(Equal(gardenSecret.Data))
			})
		})
	})
})

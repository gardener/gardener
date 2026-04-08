// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package secrets_test

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/controllermanager/controller/seed/secrets"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Reconciler", func() {
	var (
		ctx = context.Background()

		reconciler *Reconciler

		fakeClient    client.Client
		seed          *gardencorev1beta1.Seed
		namespaceName string
	)

	BeforeEach(func() {
		seed = &gardencorev1beta1.Seed{
			ObjectMeta: metav1.ObjectMeta{
				Name: "seed",
				UID:  "abcdef",
			},
		}
		namespaceName = gardenerutils.ComputeGardenNamespace(seed.Name)
	})

	Describe("#Reconcile", func() {
		It("should not return an error if seed cannot be found", func() {
			fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()
			reconciler = &Reconciler{Client: fakeClient, GardenNamespace: v1beta1constants.GardenNamespace}

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(seed)})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{}))
		})

		It("should return an error if getting the seed fails", func() {
			fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).WithInterceptorFuncs(interceptor.Funcs{
				Get: func(_ context.Context, _ client.WithWatch, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
					if _, ok := obj.(*gardencorev1beta1.Seed); ok {
						return errors.New("fake")
					}
					return nil
				},
			}).Build()
			reconciler = &Reconciler{Client: fakeClient, GardenNamespace: v1beta1constants.GardenNamespace}

			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(seed)})
			Expect(err).To(MatchError(ContainSubstring("fake")))
		})

		Context("when seed exists", func() {
			BeforeEach(func() {
				fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).WithObjects(seed).Build()
				reconciler = &Reconciler{Client: fakeClient, GardenNamespace: v1beta1constants.GardenNamespace}
			})

			It("should fail if namespace exists and has no ownerReference", func() {
				ns := &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: namespaceName,
					},
				}
				Expect(fakeClient.Create(ctx, ns)).To(Succeed())

				_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(seed)})
				Expect(err).To(MatchError(ContainSubstring("not controlled by")))
			})

			It("should fail if namespace exists and is not controlled by seed", func() {
				ns := &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: namespaceName,
						OwnerReferences: []metav1.OwnerReference{
							*metav1.NewControllerRef(
								&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "boss", UID: "12345"}},
								corev1.SchemeGroupVersion.WithKind("ConfigMap"),
							),
						},
					},
				}
				Expect(fakeClient.Create(ctx, ns)).To(Succeed())

				_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(seed)})
				Expect(err).To(MatchError(ContainSubstring("not controlled by")))
			})

			It("should sync secrets and clean up stale secrets in seed namespace", func() {
				// Create the seed namespace controlled by the seed
				ns := &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name:            namespaceName,
						OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(seed, gardencorev1beta1.SchemeGroupVersion.WithKind("Seed"))},
						Labels:          map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleSeed},
					},
				}
				Expect(fakeClient.Create(ctx, ns)).To(Succeed())

				// Garden secret to sync
				gardenSecret := createSecret("garden-secret", v1beta1constants.GardenNamespace, []byte("data"), "foo")
				Expect(fakeClient.Create(ctx, gardenSecret)).To(Succeed())

				// Stale secret in seed namespace that should be deleted
				staleSecret := createSecret("stale-secret", namespaceName, []byte("old"), "foo")
				Expect(fakeClient.Create(ctx, staleSecret)).To(Succeed())

				result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(seed)})
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))

				syncedSecret := &corev1.Secret{}
				Expect(fakeClient.Get(ctx, client.ObjectKey{Namespace: namespaceName, Name: gardenSecret.Name}, syncedSecret)).To(Succeed())
				Expect(syncedSecret.Data).To(Equal(gardenSecret.Data))

				Expect(fakeClient.Get(ctx, client.ObjectKey{Namespace: namespaceName, Name: staleSecret.Name}, &corev1.Secret{})).To(BeNotFoundError())
			})

			It("should add garden role label to namespace if missing", func() {
				ns := &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name:            namespaceName,
						OwnerReferences: []metav1.OwnerReference{*metav1.NewControllerRef(seed, gardencorev1beta1.SchemeGroupVersion.WithKind("Seed"))},
					},
				}
				Expect(fakeClient.Create(ctx, ns)).To(Succeed())

				result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(seed)})
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))

				updatedNs := &corev1.Namespace{}
				Expect(fakeClient.Get(ctx, client.ObjectKey{Name: namespaceName}, updatedNs)).To(Succeed())
				Expect(updatedNs.Labels).To(HaveKeyWithValue(v1beta1constants.GardenRole, v1beta1constants.GardenRoleSeed))
			})
		})

		Context("when seed is new", func() {
			BeforeEach(func() {
				fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).WithObjects(seed).Build()
				reconciler = &Reconciler{Client: fakeClient, GardenNamespace: v1beta1constants.GardenNamespace}
			})

			It("should create namespace and sync secrets if namespace does not exist", func() {
				gardenSecret1 := createSecret("my-secret-1", v1beta1constants.GardenNamespace, []byte("data"), "foo")
				Expect(fakeClient.Create(ctx, gardenSecret1)).To(Succeed())

				gardenSecret2 := createSecret("my-secret-2", v1beta1constants.GardenNamespace, []byte("data"), "bar")
				Expect(fakeClient.Create(ctx, gardenSecret2)).To(Succeed())

				result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(seed)})
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))

				// Verify namespace was created
				ns := &corev1.Namespace{}
				Expect(fakeClient.Get(ctx, client.ObjectKey{Name: namespaceName}, ns)).To(Succeed())
				Expect(ns.Labels).To(HaveKeyWithValue(v1beta1constants.GardenRole, v1beta1constants.GardenRoleSeed))
				Expect(metav1.IsControlledBy(ns, seed)).To(BeTrue())

				// Verify secret was synced to seed namespace
				secretList := &corev1.SecretList{}
				Expect(fakeClient.List(ctx, secretList, client.InNamespace(namespaceName))).To(Succeed())
				Expect(secretList.Items).To(HaveLen(2))
				Expect(secretList.Items[0].Data).To(Equal(gardenSecret1.Data))
				Expect(secretList.Items[1].Data).To(Equal(gardenSecret2.Data))
			})
		})
	})
})

func createSecret(name, namespace string, data []byte, role string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				v1beta1constants.GardenRole: role,
			},
			Name:      name,
			Namespace: namespace,
		},
		Data: map[string][]byte{
			"key": data,
		},
	}
}

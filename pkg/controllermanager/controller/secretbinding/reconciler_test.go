// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package secretbinding

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
)

var _ = Describe("SecretBindingControl", func() {
	var (
		fakeClient client.Client
		ctx        = context.TODO()
	)

	BeforeEach(func() {
		testScheme := runtime.NewScheme()
		Expect(kubernetes.AddGardenSchemeToScheme(testScheme)).To(Succeed())

		fakeClient = fakeclient.NewClientBuilder().WithScheme(testScheme).Build()
	})

	Describe("#mayReleaseSecret", func() {
		var (
			reconciler *Reconciler

			secretBinding1Namespace = "foo"
			secretBinding1Name      = "bar"
			secretBinding2Namespace = "baz"
			secretBinding2Name      = "bax"
			secretNamespace         = "foo"
			secretName              = "bar"
		)

		BeforeEach(func() {
			reconciler = &Reconciler{Client: fakeClient}
		})

		It("should return true as no other secretbinding exists", func() {
			allowed, err := reconciler.mayReleaseSecret(ctx, secretBinding1Namespace, secretBinding1Name, secretNamespace, secretName)

			Expect(allowed).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return true as no other secretbinding references the secret", func() {
			secretBinding := &gardencorev1beta1.SecretBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretBinding1Name,
					Namespace: secretBinding1Namespace,
				},
				SecretRef: corev1.SecretReference{
					Namespace: secretNamespace,
					Name:      secretName,
				},
			}

			Expect(fakeClient.Create(ctx, secretBinding)).To(Succeed())

			allowed, err := reconciler.mayReleaseSecret(ctx, secretBinding1Namespace, secretBinding1Name, secretNamespace, secretName)

			Expect(allowed).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return false as another secretbinding references the secret", func() {
			secretBinding := &gardencorev1beta1.SecretBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretBinding2Name,
					Namespace: secretBinding2Namespace,
				},
				SecretRef: corev1.SecretReference{
					Namespace: secretNamespace,
					Name:      secretName,
				},
			}

			Expect(fakeClient.Create(ctx, secretBinding)).To(Succeed())

			allowed, err := reconciler.mayReleaseSecret(ctx, secretBinding1Namespace, secretBinding1Name, secretNamespace, secretName)

			Expect(allowed).To(BeFalse())
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("SecretBinding and Provider label for Secrets", func() {
		var (
			reconciler *Reconciler
			request    reconcile.Request

			secretBindingNamespace = "foo"
			secretBindingName      = "bar"

			secret        *corev1.Secret
			secretBinding *gardencorev1beta1.SecretBinding
		)

		BeforeEach(func() {
			secret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "secret",
					Namespace: "namespace",
				},
			}

			secretBinding = &gardencorev1beta1.SecretBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretBindingName,
					Namespace: secretBindingNamespace,
				},
				SecretRef: corev1.SecretReference{
					Namespace: secret.Namespace,
					Name:      secret.Name,
				},
				Provider: &gardencorev1beta1.SecretBindingProvider{
					Type: "provider",
				},
			}

			Expect(fakeClient.Create(ctx, secret)).To(Succeed())
			Expect(fakeClient.Create(ctx, secretBinding)).To(Succeed())

			reconciler = &Reconciler{Client: fakeClient}
			request = reconcile.Request{NamespacedName: types.NamespacedName{Namespace: secretBindingNamespace, Name: secretBindingName}}
		})

		It("should add the secretbinding referred label to the secret referred by the secretbinding", func() {
			_, err := reconciler.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())
			Expect(secret.ObjectMeta.Labels).To(And(
				HaveKeyWithValue("reference.gardener.cloud/secretbinding", "true"),
				HaveKeyWithValue("provider.shoot.gardener.cloud/provider", "true"),
			))
		})

		It("should remove both the labels from the secret when there are no other secretbindings referring it", func() {
			_, err := reconciler.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeClient.Delete(ctx, secretBinding)).To(Succeed())

			_, err = reconciler.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())
			Expect(secret.ObjectMeta.Labels).To(BeEmpty())
		})

		It("should not remove any of the label from the secret when there are other secretbindings referring it", func() {
			secretBinding2 := &gardencorev1beta1.SecretBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "secretbinding-2",
					Namespace: "some-namespace",
				},
				SecretRef: corev1.SecretReference{
					Namespace: secret.Namespace,
					Name:      secret.Name,
				},
			}
			Expect(fakeClient.Create(ctx, secretBinding2)).To(Succeed())

			_, err := reconciler.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeClient.Delete(ctx, secretBinding)).To(Succeed())

			_, err = reconciler.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())
			Expect(secret.ObjectMeta.Labels).To(And(
				HaveKeyWithValue("reference.gardener.cloud/secretbinding", "true"),
				HaveKeyWithValue("provider.shoot.gardener.cloud/provider", "true"),
			))
		})

		It("should only remove the secretbinding ref label from the secret when secret is labeled with credentialsbinding reference", func() {
			Expect(fakeClient.Delete(ctx, secret)).To(Succeed())
			secret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "secret",
					Namespace: "namespace",
					Labels:    map[string]string{"reference.gardener.cloud/credentialsbinding": "true"},
				},
			}
			Expect(fakeClient.Create(ctx, secret)).To(Succeed())

			_, err := reconciler.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeClient.Delete(ctx, secretBinding)).To(Succeed())

			_, err = reconciler.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())
			Expect(secret.ObjectMeta.Labels).To(Equal(map[string]string{
				"provider.shoot.gardener.cloud/provider":      "true",
				"reference.gardener.cloud/credentialsbinding": "true",
			}))
			Expect(secret.ObjectMeta.Finalizers).To(ConsistOf("gardener.cloud/gardener"))
		})
	})

	Describe("SecretBinding label for Quotas", func() {
		var (
			reconciler *Reconciler
			request    reconcile.Request

			secretBindingNamespace1 = "sb-ns-1"
			secretBindingName1      = "sb-1"
			secretBindingNamespace2 = "sb-ns-2"
			secretBindingName2      = "sb-2"
			quotaNamespace1         = "quota-ns-1"
			quotaName1              = "quota-1"
			quotaNamespace2         = "quota-ns-2"
			quotaName2              = "quota-2"

			secret                         *corev1.Secret
			secretBinding1, secretBinding2 *gardencorev1beta1.SecretBinding
			quota1, quota2                 *gardencorev1beta1.Quota
		)

		BeforeEach(func() {
			secret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "secret",
					Namespace: "namespace",
				},
			}

			quota1 = &gardencorev1beta1.Quota{
				ObjectMeta: metav1.ObjectMeta{
					Name:      quotaName1,
					Namespace: quotaNamespace1,
				},
			}
			quota2 = &gardencorev1beta1.Quota{
				ObjectMeta: metav1.ObjectMeta{
					Name:      quotaName2,
					Namespace: quotaNamespace2,
				},
			}

			secretBinding1 = &gardencorev1beta1.SecretBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretBindingName1,
					Namespace: secretBindingNamespace1,
				},
				Quotas: []corev1.ObjectReference{
					{
						Name:      quotaName1,
						Namespace: quotaNamespace1,
					},
					{
						Name:      quotaName2,
						Namespace: quotaNamespace2,
					},
				},
				SecretRef: corev1.SecretReference{
					Name:      secret.Name,
					Namespace: secret.Namespace,
				},
			}

			secretBinding2 = &gardencorev1beta1.SecretBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:       secretBindingName2,
					Namespace:  secretBindingNamespace2,
					Finalizers: []string{"gardener"},
				},
				Quotas: []corev1.ObjectReference{
					{
						Name:      quotaName2,
						Namespace: quotaNamespace2,
					},
				},
				SecretRef: corev1.SecretReference{
					Name:      secret.Name,
					Namespace: secret.Namespace,
				},
			}

			reconciler = &Reconciler{Client: fakeClient}
			request = reconcile.Request{NamespacedName: types.NamespacedName{Namespace: secretBindingNamespace1, Name: secretBindingName1}}

			Expect(fakeClient.Create(ctx, secret)).To(Succeed())
			Expect(fakeClient.Create(ctx, quota1)).To(Succeed())
			Expect(fakeClient.Create(ctx, quota2)).To(Succeed())
			Expect(fakeClient.Create(ctx, secretBinding1)).To(Succeed())
			Expect(fakeClient.Create(ctx, secretBinding2)).To(Succeed())
		})

		It("should add the label to the quota referred by the secretbinding", func() {
			_, err := reconciler.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(quota1), quota1)).To(Succeed())
			Expect(quota1.ObjectMeta.Labels).To(HaveKeyWithValue(
				"reference.gardener.cloud/secretbinding", "true",
			))
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(quota2), quota2)).To(Succeed())
			Expect(quota2.ObjectMeta.Labels).To(HaveKeyWithValue(
				"reference.gardener.cloud/secretbinding", "true",
			))
		})

		It("should remove the label from the quotas when there are no secretbindings referring it", func() {
			_, err := reconciler.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(secretBinding1), secretBinding1)).To(Succeed())
			Expect(fakeClient.Delete(ctx, secretBinding1)).To(Succeed())

			_, err = reconciler.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(quota1), quota1)).To(Succeed())
			Expect(quota1.ObjectMeta.Labels).To(BeEmpty())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(quota2), quota2)).To(Succeed())
			Expect(quota2.ObjectMeta.Labels).To(HaveKeyWithValue(
				"reference.gardener.cloud/secretbinding", "true",
			))

			request = reconcile.Request{NamespacedName: types.NamespacedName{Namespace: secretBindingNamespace2, Name: secretBindingName2}}
			_, err = reconciler.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeClient.Delete(ctx, secretBinding2)).To(Succeed())

			_, err = reconciler.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(quota2), quota2)).To(Succeed())
			Expect(quota2.ObjectMeta.Labels).To(BeEmpty())
		})
	})
})

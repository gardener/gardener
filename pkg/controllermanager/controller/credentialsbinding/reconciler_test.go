// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package credentialsbinding_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllermanager/controller/credentialsbinding"
)

var _ = Describe("CredentialsBindingControl", func() {
	var (
		fakeClient client.Client
		ctx        = context.TODO()
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()
	})

	Describe("CredentialsBinding and Provider label for Secrets", func() {
		var (
			reconciler *credentialsbinding.Reconciler
			request    reconcile.Request

			credentialsBindingNamespace = "foo"
			credentialsBindingName      = "bar"

			secret             *corev1.Secret
			credentialsBinding *securityv1alpha1.CredentialsBinding
		)

		BeforeEach(func() {
			secret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "secret",
					Namespace: "namespace",
				},
			}

			credentialsBinding = &securityv1alpha1.CredentialsBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      credentialsBindingName,
					Namespace: credentialsBindingNamespace,
				},
				CredentialsRef: corev1.ObjectReference{
					Kind:       "Secret",
					APIVersion: corev1.SchemeGroupVersion.String(),
					Namespace:  secret.Namespace,
					Name:       secret.Name,
				},
				Provider: securityv1alpha1.CredentialsBindingProvider{
					Type: "some-provider",
				},
			}

			Expect(fakeClient.Create(ctx, secret)).To(Succeed())
			Expect(fakeClient.Create(ctx, credentialsBinding)).To(Succeed())

			reconciler = &credentialsbinding.Reconciler{Client: fakeClient}
			request = reconcile.Request{NamespacedName: types.NamespacedName{Namespace: credentialsBindingNamespace, Name: credentialsBindingName}}
		})

		It("should add the credentialsbinding referred label to the secret referred by the credentialsbinding", func() {
			_, err := reconciler.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())
			Expect(secret.ObjectMeta.Labels).To(And(
				HaveKeyWithValue("reference.gardener.cloud/credentialsbinding", "true"),
				HaveKeyWithValue("provider.shoot.gardener.cloud/some-provider", "true"),
			))
		})

		It("should remove both the labels from the secret when there are no other credentialsbindings referring it", func() {
			_, err := reconciler.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeClient.Delete(ctx, credentialsBinding)).To(Succeed())

			_, err = reconciler.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())
			Expect(secret.ObjectMeta.Labels).To(BeEmpty())
		})

		It("should not remove any of the label from the secret when there are other credentialsbindings referring it", func() {
			credentialsBinding2 := &securityv1alpha1.CredentialsBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "credentialsbinding-2",
					Namespace: "some-namespace",
				},
				CredentialsRef: corev1.ObjectReference{
					Kind:       "Secret",
					APIVersion: corev1.SchemeGroupVersion.String(),
					Namespace:  secret.Namespace,
					Name:       secret.Name,
				},
			}
			Expect(fakeClient.Create(ctx, credentialsBinding2)).To(Succeed())

			_, err := reconciler.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeClient.Delete(ctx, credentialsBinding)).To(Succeed())

			_, err = reconciler.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())
			Expect(secret.ObjectMeta.Labels).To(And(
				HaveKeyWithValue("reference.gardener.cloud/credentialsbinding", "true"),
				HaveKeyWithValue("provider.shoot.gardener.cloud/some-provider", "true"),
			))
		})

		It("should only remove the credentialsbinding ref label from the secret when secret is labeled with secretbinding reference", func() {
			Expect(fakeClient.Delete(ctx, secret)).To(Succeed())
			secret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "secret",
					Namespace: "namespace",
					Labels:    map[string]string{"reference.gardener.cloud/secretbinding": "true"},
				},
			}
			Expect(fakeClient.Create(ctx, secret)).To(Succeed())

			_, err := reconciler.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeClient.Delete(ctx, credentialsBinding)).To(Succeed())

			_, err = reconciler.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())
			Expect(secret.ObjectMeta.Labels).To(Equal(map[string]string{
				"provider.shoot.gardener.cloud/some-provider": "true",
				"reference.gardener.cloud/secretbinding":      "true",
			}))
			Expect(secret.ObjectMeta.Finalizers).To(ConsistOf("gardener.cloud/gardener"))
		})
	})

	Describe("CredentialsBinding and Provider label for WorkloadIdentity", func() {
		var (
			reconciler *credentialsbinding.Reconciler
			request    reconcile.Request

			credentialsBindingNamespace = "foo"
			credentialsBindingName      = "bar"

			workloadIdentity   *securityv1alpha1.WorkloadIdentity
			credentialsBinding *securityv1alpha1.CredentialsBinding
		)

		BeforeEach(func() {
			workloadIdentity = &securityv1alpha1.WorkloadIdentity{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "wi",
					Namespace: "namespace",
				},
			}

			credentialsBinding = &securityv1alpha1.CredentialsBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      credentialsBindingName,
					Namespace: credentialsBindingNamespace,
				},
				CredentialsRef: corev1.ObjectReference{
					Kind:       "WorkloadIdentity",
					APIVersion: securityv1alpha1.SchemeGroupVersion.String(),
					Namespace:  workloadIdentity.Namespace,
					Name:       workloadIdentity.Name,
				},
				Provider: securityv1alpha1.CredentialsBindingProvider{
					Type: "some-provider",
				},
			}

			Expect(fakeClient.Create(ctx, workloadIdentity)).To(Succeed())
			Expect(fakeClient.Create(ctx, credentialsBinding)).To(Succeed())

			reconciler = &credentialsbinding.Reconciler{Client: fakeClient}
			request = reconcile.Request{NamespacedName: types.NamespacedName{Namespace: credentialsBindingNamespace, Name: credentialsBindingName}}
		})

		It("should add the credentialsbinding referred label to the WorkloadIdentity referred by the CredentialsBinding", func() {
			_, err := reconciler.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(workloadIdentity), workloadIdentity)).To(Succeed())
			Expect(workloadIdentity.ObjectMeta.Labels).To(And(
				HaveKeyWithValue("reference.gardener.cloud/credentialsbinding", "true"),
				HaveKeyWithValue("provider.shoot.gardener.cloud/some-provider", "true"),
			))
		})

		It("should remove both the labels from the WorkloadIdentity when there are no other CredentialsBindings referring it", func() {
			_, err := reconciler.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeClient.Delete(ctx, credentialsBinding)).To(Succeed())

			_, err = reconciler.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(workloadIdentity), workloadIdentity)).To(Succeed())
			Expect(workloadIdentity.ObjectMeta.Labels).To(BeEmpty())
		})

		It("should not remove any of the label from the WorkloadIdentity when there are other CredentialsBindings referring it", func() {
			credentialsBinding2 := &securityv1alpha1.CredentialsBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "credentialsbinding-2",
					Namespace: "some-namespace",
				},
				CredentialsRef: corev1.ObjectReference{
					Kind:       "WorkloadIdentity",
					APIVersion: securityv1alpha1.SchemeGroupVersion.String(),
					Namespace:  workloadIdentity.Namespace,
					Name:       workloadIdentity.Name,
				},
			}
			Expect(fakeClient.Create(ctx, credentialsBinding2)).To(Succeed())

			_, err := reconciler.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeClient.Delete(ctx, credentialsBinding)).To(Succeed())

			_, err = reconciler.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(workloadIdentity), workloadIdentity)).To(Succeed())
			Expect(workloadIdentity.ObjectMeta.Labels).To(And(
				HaveKeyWithValue("reference.gardener.cloud/credentialsbinding", "true"),
				HaveKeyWithValue("provider.shoot.gardener.cloud/some-provider", "true"),
			))
		})
	})

	Describe("CredentialsBinding label for Quotas", func() {
		var (
			reconciler *credentialsbinding.Reconciler
			request    reconcile.Request

			credentialsBindingNamespace1 = "cb-ns-1"
			credentialsBindingName1      = "cb-1"
			credentialsBindingNamespace2 = "cb-ns-2"
			credentialsBindingName2      = "cb-2"
			quotaNamespace1              = "quota-ns-1"
			quotaName1                   = "quota-1"
			quotaNamespace2              = "quota-ns-2"
			quotaName2                   = "quota-2"

			secret                                   *corev1.Secret
			credentialsBinding1, credentialsBinding2 *securityv1alpha1.CredentialsBinding
			quota1, quota2                           *gardencorev1beta1.Quota
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

			credentialsBinding1 = &securityv1alpha1.CredentialsBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      credentialsBindingName1,
					Namespace: credentialsBindingNamespace1,
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
				CredentialsRef: corev1.ObjectReference{
					Kind:       "Secret",
					APIVersion: corev1.SchemeGroupVersion.String(),
					Name:       secret.Name,
					Namespace:  secret.Namespace,
				},
			}

			credentialsBinding2 = &securityv1alpha1.CredentialsBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:       credentialsBindingName2,
					Namespace:  credentialsBindingNamespace2,
					Finalizers: []string{"gardener"},
				},
				Quotas: []corev1.ObjectReference{
					{
						Name:      quotaName2,
						Namespace: quotaNamespace2,
					},
				},
				CredentialsRef: corev1.ObjectReference{
					Kind:       "Secret",
					APIVersion: corev1.SchemeGroupVersion.String(),
					Name:       secret.Name,
					Namespace:  secret.Namespace,
				},
			}

			reconciler = &credentialsbinding.Reconciler{Client: fakeClient}
			request = reconcile.Request{NamespacedName: types.NamespacedName{Namespace: credentialsBindingNamespace1, Name: credentialsBindingName1}}

			Expect(fakeClient.Create(ctx, secret)).To(Succeed())
			Expect(fakeClient.Create(ctx, quota1)).To(Succeed())
			Expect(fakeClient.Create(ctx, quota2)).To(Succeed())
			Expect(fakeClient.Create(ctx, credentialsBinding1)).To(Succeed())
			Expect(fakeClient.Create(ctx, credentialsBinding2)).To(Succeed())
		})

		It("should add the label to the quota referred by the credentialsbinding", func() {
			_, err := reconciler.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(quota1), quota1)).To(Succeed())
			Expect(quota1.ObjectMeta.Labels).To(HaveKeyWithValue(
				"reference.gardener.cloud/credentialsbinding", "true",
			))
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(quota2), quota2)).To(Succeed())
			Expect(quota2.ObjectMeta.Labels).To(HaveKeyWithValue(
				"reference.gardener.cloud/credentialsbinding", "true",
			))
		})

		It("should remove the label from the quotas when there are no credentialsbindings referring it", func() {
			_, err := reconciler.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(credentialsBinding1), credentialsBinding1)).To(Succeed())
			Expect(fakeClient.Delete(ctx, credentialsBinding1)).To(Succeed())

			_, err = reconciler.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(quota1), quota1)).To(Succeed())
			Expect(quota1.ObjectMeta.Labels).To(BeEmpty())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(quota2), quota2)).To(Succeed())
			Expect(quota2.ObjectMeta.Labels).To(HaveKeyWithValue(
				"reference.gardener.cloud/credentialsbinding", "true",
			))

			request = reconcile.Request{NamespacedName: types.NamespacedName{Namespace: credentialsBindingNamespace2, Name: credentialsBindingName2}}
			_, err = reconciler.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeClient.Delete(ctx, credentialsBinding2)).To(Succeed())

			_, err = reconciler.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(quota2), quota2)).To(Succeed())
			Expect(quota2.ObjectMeta.Labels).To(BeEmpty())
		})
	})
})

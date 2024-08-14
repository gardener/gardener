// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package referencecleaner_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	"github.com/gardener/gardener/pkg/controllermanager/controller/credentialsbinding/referencecleaner"
)

var _ = Describe("ReferenceCleaner", func() {
	var (
		fakeClient client.Client
		ctx        = context.TODO()
		// TODO(dimityrmirchev): Check the finalizers of referenced credentials
		// once/if https://github.com/kubernetes-sigs/controller-runtime/issues/2923 is resolved.
		// Proper finalizers cleanup is tested in the integration test suite.
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()
	})

	Describe("CredentialsBinding and Provider label for Secrets", func() {
		var (
			reconciler *referencecleaner.Reconciler
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
					// Finalizers: []string{"gardener.cloud/gardener"},
					Labels: map[string]string{
						"reference.gardener.cloud/credentialsbinding": "true",
						"provider.shoot.gardener.cloud/some-provider": "true",
					},
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

			reconciler = &referencecleaner.Reconciler{Reader: fakeClient, Writer: fakeClient, Config: config.CredentialsBindingReferenceCleanerControllerConfiguration{
				SyncPeriod: &metav1.Duration{Duration: time.Hour},
			}}
			request = reconcile.Request{}
		})

		It("should remove both the labels from the secret when there are no other credentialsbindings referring it", func() {
			Expect(fakeClient.Delete(ctx, credentialsBinding)).To(Succeed())

			_, err := reconciler.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())
			Expect(secret.ObjectMeta.Labels).To(BeEmpty())
			Expect(secret.ObjectMeta.Finalizers).To(BeEmpty())
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
			Expect(fakeClient.Delete(ctx, credentialsBinding)).To(Succeed())

			_, err := reconciler.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())
			Expect(secret.ObjectMeta.Labels).To(And(
				HaveKeyWithValue("reference.gardener.cloud/credentialsbinding", "true"),
				HaveKeyWithValue("provider.shoot.gardener.cloud/some-provider", "true"),
			))
			// Expect(secret.ObjectMeta.Finalizers).To(ConsistOf("gardener.cloud/gardener"))
		})

		It("should only remove the credentialsbinding ref label from the secret when secret is labeled with secretbinding reference", func() {
			secret.Finalizers = []string{"gardener.cloud/gardener"}
			secret.Labels = map[string]string{
				"reference.gardener.cloud/secretbinding":      "true",
				"reference.gardener.cloud/credentialsbinding": "true",
				"provider.shoot.gardener.cloud/some-provider": "true",
			}
			Expect(fakeClient.Update(ctx, secret)).To(Succeed())
			Expect(fakeClient.Delete(ctx, credentialsBinding)).To(Succeed())

			_, err := reconciler.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())
			Expect(secret.ObjectMeta.Labels).To(Equal(map[string]string{
				"provider.shoot.gardener.cloud/some-provider": "true",
				"reference.gardener.cloud/secretbinding":      "true",
			}))
			// Expect(secret.ObjectMeta.Finalizers).To(ConsistOf("gardener.cloud/gardener"))
		})
	})

	Describe("CredentialsBinding and Provider label for WorkloadIdentity", func() {
		var (
			reconciler *referencecleaner.Reconciler
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
					// Finalizers: []string{"gardener.cloud/gardener"},
					Labels: map[string]string{
						"reference.gardener.cloud/credentialsbinding": "true",
						"provider.shoot.gardener.cloud/some-provider": "true",
					},
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

			reconciler = &referencecleaner.Reconciler{Reader: fakeClient, Writer: fakeClient, Config: config.CredentialsBindingReferenceCleanerControllerConfiguration{
				SyncPeriod: &metav1.Duration{Duration: time.Hour},
			}}
			request = reconcile.Request{}
		})

		It("should remove both the labels from the workloadIdentity when there are no other credentialsbindings referring it", func() {
			Expect(fakeClient.Delete(ctx, credentialsBinding)).To(Succeed())

			_, err := reconciler.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(workloadIdentity), workloadIdentity)).To(Succeed())
			Expect(workloadIdentity.ObjectMeta.Labels).To(BeEmpty())
			Expect(workloadIdentity.ObjectMeta.Finalizers).To(BeEmpty())
		})

		It("should not remove any of the label from the workload identity when there are other credentialsbindings referring it", func() {
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
			Expect(fakeClient.Delete(ctx, credentialsBinding)).To(Succeed())

			_, err := reconciler.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(workloadIdentity), workloadIdentity)).To(Succeed())
			Expect(workloadIdentity.ObjectMeta.Labels).To(And(
				HaveKeyWithValue("reference.gardener.cloud/credentialsbinding", "true"),
				HaveKeyWithValue("provider.shoot.gardener.cloud/some-provider", "true"),
			))
			// Expect(workloadIdentity.ObjectMeta.Finalizers).To(ConsistOf("gardener.cloud/gardener"))
		})
	})
})

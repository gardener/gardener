// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package quota

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Reconciler", func() {
	const finalizerName = "gardener"

	var (
		ctx        = context.TODO()
		fakeClient client.Client
		reconciler reconcile.Reconciler

		quotaName          string
		quota              *gardencorev1beta1.Quota
		secretBinding      *gardencorev1beta1.SecretBinding
		credentialsBinding *securityv1alpha1.CredentialsBinding
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()

		quotaName = "test-quota"
		reconciler = &Reconciler{Client: fakeClient, Recorder: &record.FakeRecorder{}}
		quota = &gardencorev1beta1.Quota{
			ObjectMeta: metav1.ObjectMeta{
				Name: quotaName,
			},
		}

		secretBinding = &gardencorev1beta1.SecretBinding{
			ObjectMeta: metav1.ObjectMeta{Name: "test-secretbinding", Namespace: "test-namespace"},
			Quotas: []corev1.ObjectReference{
				{
					Name: quotaName,
				},
			},
		}

		credentialsBinding = &securityv1alpha1.CredentialsBinding{
			ObjectMeta: metav1.ObjectMeta{Name: "test-credentialsbinding", Namespace: "test-namespace"},
			Quotas: []corev1.ObjectReference{
				{
					Name: quotaName,
				},
			},
		}
	})

	It("should return nil because object not found", func() {
		Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(quota), &gardencorev1beta1.Quota{})).To(BeNotFoundError())

		result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: quotaName}})
		Expect(result).To(Equal(reconcile.Result{}))
		Expect(err).NotTo(HaveOccurred())
	})

	Context("when deletion timestamp not set", func() {
		BeforeEach(func() {
			Expect(fakeClient.Create(ctx, quota)).To(Succeed())
		})

		It("should ensure the finalizer", func() {
			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: quotaName}})
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(quota), quota)).To(Succeed())
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(err).NotTo(HaveOccurred())
			Expect(quota.GetFinalizers()).Should(ConsistOf(finalizerName))
		})
	})

	Context("when deletion timestamp set", func() {
		BeforeEach(func() {
			quota.Finalizers = []string{finalizerName}

			Expect(fakeClient.Create(ctx, quota)).To(Succeed())
			Expect(fakeClient.Delete(ctx, quota)).To(Succeed())
		})

		It("should do nothing because finalizer is not present", func() {
			Expect(fakeClient.Create(ctx, secretBinding)).To(Succeed())
			Expect(fakeClient.Create(ctx, credentialsBinding)).To(Succeed())
			patch := client.MergeFrom(quota.DeepCopy())
			quota.Finalizers = nil
			Expect(fakeClient.Patch(ctx, quota, patch)).To(Succeed())

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: quotaName}})
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return an error because SecretBinding referencing Quota exists", func() {
			Expect(fakeClient.Create(ctx, secretBinding)).To(Succeed())

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: quotaName}})
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(err).To(MatchError(ContainSubstring("cannot delete Quota")))
		})

		It("should return an error because CredentialsBinding referencing Quota exists", func() {
			Expect(fakeClient.Create(ctx, credentialsBinding)).To(Succeed())

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: quotaName}})
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(err).To(MatchError(ContainSubstring("cannot delete Quota")))
		})

		It("should remove the finalizer because no SecretBinding or CredentialsBinding are referencing the Quota", func() {
			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: quotaName}})
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(err).NotTo(HaveOccurred())
			Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(quota), quota)).To(BeNotFoundError())
		})
	})
})

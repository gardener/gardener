// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package quota_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/clock"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	controllermanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/controllermanager/apis/config/v1alpha1"
	shootquota "github.com/gardener/gardener/pkg/controllermanager/controller/shoot/quota"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Reconciler", func() {
	var (
		ctx        = context.TODO()
		fakeClient client.Client
		reconciler reconcile.Reconciler

		lifetime           = ptr.To[int32](1)
		namespace          = "test-namespace"
		quotaName          = "test-quota"
		quota              *gardencorev1beta1.Quota
		secretBinding      *gardencorev1beta1.SecretBinding
		credentialsBinding *securityv1alpha1.CredentialsBinding
		shoot              *gardencorev1beta1.Shoot
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()

		reconciler = &shootquota.Reconciler{
			Client: fakeClient,
			Clock:  clock.RealClock{},
			Config: controllermanagerconfigv1alpha1.ShootQuotaControllerConfiguration{
				ConcurrentSyncs: ptr.To(1),
				SyncPeriod:      &metav1.Duration{},
			},
		}
		quota = &gardencorev1beta1.Quota{
			ObjectMeta: metav1.ObjectMeta{
				Name:      quotaName,
				Namespace: namespace,
			},
			Spec: gardencorev1beta1.QuotaSpec{
				ClusterLifetimeDays: ptr.To[int32](1),
			},
		}

		secretBinding = &gardencorev1beta1.SecretBinding{
			ObjectMeta: metav1.ObjectMeta{Name: "test-secretbinding", Namespace: namespace},
			Quotas: []corev1.ObjectReference{
				{
					Name:      quotaName,
					Namespace: namespace,
				},
			},
		}

		credentialsBinding = &securityv1alpha1.CredentialsBinding{
			ObjectMeta: metav1.ObjectMeta{Name: "test-credentialsbinding", Namespace: namespace},
			Quotas: []corev1.ObjectReference{
				{
					Name:      quotaName,
					Namespace: namespace,
				},
			},
		}

		shoot = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{Name: "test-shoot", Namespace: namespace, CreationTimestamp: metav1.Now()},
			Spec: gardencorev1beta1.ShootSpec{
				SecretBindingName: ptr.To("test-secretbinding"),
			},
		}
	})

	It("should delete the shoot using secret binding because it is expired", func() {
		expiredTime := shoot.CreationTimestamp.Add(-(time.Duration(*lifetime*24) * time.Hour) * 2)
		shoot.Annotations = map[string]string{
			"shoot.gardener.cloud/expiration-timestamp": expiredTime.Format(time.RFC3339),
		}

		Expect(fakeClient.Create(ctx, quota)).To(Succeed())
		Expect(fakeClient.Create(ctx, secretBinding)).To(Succeed())
		Expect(fakeClient.Create(ctx, shoot)).To(Succeed())

		result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: shoot.Name, Namespace: shoot.Namespace}})
		Expect(result).To(Equal(reconcile.Result{}))
		Expect(err).NotTo(HaveOccurred())
		Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(BeNotFoundError())
	})

	It("should not delete the shoot using secret binding because it is not expired", func() {
		notExpiredTime := shoot.CreationTimestamp.Add((time.Duration(*lifetime*24) * time.Hour) * 2)
		shoot.Annotations = map[string]string{
			"shoot.gardener.cloud/expiration-timestamp": notExpiredTime.Format(time.RFC3339),
		}

		Expect(fakeClient.Create(ctx, quota)).To(Succeed())
		Expect(fakeClient.Create(ctx, secretBinding)).To(Succeed())
		Expect(fakeClient.Create(ctx, shoot)).To(Succeed())

		result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: shoot.Name, Namespace: shoot.Namespace}})
		Expect(result).To(Equal(reconcile.Result{}))
		Expect(err).NotTo(HaveOccurred())
		Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
	})

	It("should delete the shoot using credentials binding because it is expired", func() {
		expiredTime := shoot.CreationTimestamp.Add(-(time.Duration(*lifetime*24) * time.Hour) * 2)
		shoot.Annotations = map[string]string{
			"shoot.gardener.cloud/expiration-timestamp": expiredTime.Format(time.RFC3339),
		}
		shoot.Spec.SecretBindingName = nil
		shoot.Spec.CredentialsBindingName = &credentialsBinding.Name

		Expect(fakeClient.Create(ctx, quota)).To(Succeed())
		Expect(fakeClient.Create(ctx, credentialsBinding)).To(Succeed())
		Expect(fakeClient.Create(ctx, shoot)).To(Succeed())

		result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: shoot.Name, Namespace: shoot.Namespace}})
		Expect(result).To(Equal(reconcile.Result{}))
		Expect(err).NotTo(HaveOccurred())
		Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(BeNotFoundError())
	})

	It("should not delete the shoot using credentials binding because it is not expired", func() {
		notExpiredTime := shoot.CreationTimestamp.Add((time.Duration(*lifetime*24) * time.Hour) * 2)
		shoot.Annotations = map[string]string{
			"shoot.gardener.cloud/expiration-timestamp": notExpiredTime.Format(time.RFC3339),
		}
		shoot.Spec.SecretBindingName = nil
		shoot.Spec.CredentialsBindingName = &credentialsBinding.Name

		Expect(fakeClient.Create(ctx, quota)).To(Succeed())
		Expect(fakeClient.Create(ctx, credentialsBinding)).To(Succeed())
		Expect(fakeClient.Create(ctx, shoot)).To(Succeed())

		result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: shoot.Name, Namespace: shoot.Namespace}})
		Expect(result).To(Equal(reconcile.Result{}))
		Expect(err).NotTo(HaveOccurred())
		Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
	})

	It("should set the expiration timestamp annotation on the shoot", func() {
		Expect(fakeClient.Create(ctx, quota)).To(Succeed())
		Expect(fakeClient.Create(ctx, secretBinding)).To(Succeed())
		Expect(fakeClient.Create(ctx, shoot)).To(Succeed())

		result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: shoot.Name, Namespace: shoot.Namespace}})
		Expect(result).To(Equal(reconcile.Result{}))
		Expect(err).NotTo(HaveOccurred())
		Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
		_, ok := shoot.Annotations["shoot.gardener.cloud/expiration-timestamp"]
		Expect(ok).To(BeTrue())
	})
})

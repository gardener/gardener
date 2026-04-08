// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bastion_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	controllermanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/controllermanager/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/controllermanager/controller/bastion"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Controller", func() {
	var (
		fakeClient client.Client
		reconciler reconcile.Reconciler

		namespace   = "garden-dev"
		bastionName = "bastion"
		shootName   = "myshoot"
		seedName    = "myseed"
		ctx         = context.TODO()
		maxLifetime = 12 * time.Hour
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()
		reconciler = &Reconciler{
			Client: fakeClient,
			Config: controllermanagerconfigv1alpha1.BastionControllerConfiguration{
				MaxLifetime: &metav1.Duration{Duration: maxLifetime},
			},
			Clock: clock.RealClock{},
		}
	})

	Describe("Reconciler", func() {
		It("should return nil because object not found", func() {
			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: client.ObjectKey{Namespace: namespace, Name: bastionName}})
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should requeue alive Bastions", func() {
			created := time.Now().Add(-maxLifetime / 2)
			requeueAfter := time.Until(created.Add(maxLifetime))

			shoot := newShoot(namespace, shootName, &seedName)
			Expect(fakeClient.Create(ctx, &shoot)).To(Succeed())

			bastion := newBastion(namespace, bastionName, shootName, &seedName, &created, nil)
			Expect(fakeClient.Create(ctx, &bastion)).To(Succeed())

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: client.ObjectKey{Namespace: namespace, Name: bastionName}})
			Expect(result.RequeueAfter).To(BeNumerically("~", requeueAfter, 1*time.Second))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should requeue soon-to-expire Bastions", func() {
			now := time.Now()
			remaining := 30 * time.Second
			expires := now.Add(remaining)

			shoot := newShoot(namespace, shootName, &seedName)
			Expect(fakeClient.Create(ctx, &shoot)).To(Succeed())

			bastion := newBastion(namespace, bastionName, shootName, &seedName, &now, &expires)
			Expect(fakeClient.Create(ctx, &bastion)).To(Succeed())

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: client.ObjectKey{Namespace: namespace, Name: bastionName}})
			Expect(result.RequeueAfter).To(BeNumerically("~", remaining, 1*time.Second))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should requeue soon-to-reach-max-lifetime Bastions", func() {
			now := time.Now()
			created := now.Add(-maxLifetime).Add(10 * time.Minute) // reaches end-of-life in 10 minutes
			expires := now.Add(30 * time.Minute)                   // expires in 30 minutes

			shoot := newShoot(namespace, shootName, &seedName)
			Expect(fakeClient.Create(ctx, &shoot)).To(Succeed())

			bastion := newBastion(namespace, bastionName, shootName, &seedName, &created, &expires)
			Expect(fakeClient.Create(ctx, &bastion)).To(Succeed())

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: client.ObjectKey{Namespace: namespace, Name: bastionName}})
			Expect(result.RequeueAfter).To(BeNumerically("~", 10*time.Minute, 1*time.Second))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should delete Bastions with missing shoots", func() {
			created := time.Now().Add(-maxLifetime / 2)
			bastion := newBastion(namespace, bastionName, shootName, &seedName, &created, nil)
			Expect(fakeClient.Create(ctx, &bastion)).To(Succeed())

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: client.ObjectKey{Namespace: namespace, Name: bastionName}})
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: bastionName}, &operationsv1alpha1.Bastion{})).To(BeNotFoundError())
		})

		It("should delete Bastions with shoots in deletion", func() {
			shoot := newShoot(namespace, shootName, &seedName)
			shoot.Finalizers = []string{"test-finalizer"}
			Expect(fakeClient.Create(ctx, &shoot)).To(Succeed())
			Expect(fakeClient.Delete(ctx, &shoot)).To(Succeed())

			created := time.Now().Add(-maxLifetime / 2)
			bastion := newBastion(namespace, bastionName, shootName, &seedName, &created, nil)
			Expect(fakeClient.Create(ctx, &bastion)).To(Succeed())

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: client.ObjectKey{Namespace: namespace, Name: bastionName}})
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: bastionName}, &operationsv1alpha1.Bastion{})).To(BeNotFoundError())
		})

		It("should delete expired Bastions", func() {
			shoot := newShoot(namespace, shootName, &seedName)
			Expect(fakeClient.Create(ctx, &shoot)).To(Succeed())

			created := time.Now().Add(-maxLifetime / 2)
			expires := time.Now().Add(-5 * time.Second)
			bastion := newBastion(namespace, bastionName, shootName, &seedName, &created, &expires)
			Expect(fakeClient.Create(ctx, &bastion)).To(Succeed())

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: client.ObjectKey{Namespace: namespace, Name: bastionName}})
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: bastionName}, &operationsv1alpha1.Bastion{})).To(BeNotFoundError())
		})

		It("should delete Bastions that have reached their TTL", func() {
			shoot := newShoot(namespace, shootName, &seedName)
			Expect(fakeClient.Create(ctx, &shoot)).To(Succeed())

			created := time.Now().Add(-maxLifetime * 2)
			bastion := newBastion(namespace, bastionName, shootName, &seedName, &created, nil)
			Expect(fakeClient.Create(ctx, &bastion)).To(Succeed())

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: client.ObjectKey{Namespace: namespace, Name: bastionName}})
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: bastionName}, &operationsv1alpha1.Bastion{})).To(BeNotFoundError())
		})

		It("should delete Bastions with outdated seed information", func() {
			newSeedName := "new-seed-after-migration"

			shoot := newShoot(namespace, shootName, &newSeedName)
			Expect(fakeClient.Create(ctx, &shoot)).To(Succeed())

			created := time.Now().Add(-maxLifetime / 2)
			expires := time.Now().Add(1 * time.Minute)
			bastion := newBastion(namespace, bastionName, shootName, &seedName, &created, &expires)
			Expect(fakeClient.Create(ctx, &bastion)).To(Succeed())

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: client.ObjectKey{Namespace: namespace, Name: bastionName}})
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: bastionName}, &operationsv1alpha1.Bastion{})).To(BeNotFoundError())
		})

		It("should delete Bastions with outdated seed information 2", func() {
			shoot := newShoot(namespace, shootName, nil) // shoot was removed from original seed and since then hasn't found a new seed
			Expect(fakeClient.Create(ctx, &shoot)).To(Succeed())

			created := time.Now().Add(-maxLifetime / 2)
			expires := time.Now().Add(1 * time.Minute)
			bastion := newBastion(namespace, bastionName, shootName, &seedName, &created, &expires)
			Expect(fakeClient.Create(ctx, &bastion)).To(Succeed())

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: client.ObjectKey{Namespace: namespace, Name: bastionName}})
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(err).NotTo(HaveOccurred())

			Expect(fakeClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: bastionName}, &operationsv1alpha1.Bastion{})).To(BeNotFoundError())
		})
	})
})

func newBastion(namespace string, name string, shootName string, seedName *string, createdAt *time.Time, expiresAt *time.Time) operationsv1alpha1.Bastion {
	bastion := operationsv1alpha1.Bastion{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: operationsv1alpha1.BastionSpec{
			ShootRef: corev1.LocalObjectReference{
				Name: shootName,
			},
			SeedName: seedName,
		},
	}

	if createdAt != nil {
		bastion.CreationTimestamp = metav1.NewTime(*createdAt)
	}

	if expiresAt != nil {
		expires := metav1.NewTime(*expiresAt)
		bastion.Status.ExpirationTimestamp = &expires
	}

	return bastion
}

func newShoot(namespace string, name string, seedName *string) gardencorev1beta1.Shoot {
	shoot := gardencorev1beta1.Shoot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: gardencorev1beta1.ShootSpec{
			SeedName: seedName,
		},
		Status: gardencorev1beta1.ShootStatus{
			SeedName: seedName,
		},
	}

	return shoot
}

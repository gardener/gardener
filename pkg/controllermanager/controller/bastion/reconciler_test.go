// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package bastion_test

import (
	"context"
	"time"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	. "github.com/gardener/gardener/pkg/controllermanager/controller/bastion"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

var _ = Describe("Controller", func() {
	var (
		mockCtrl   *gomock.Controller
		mockClient *mockclient.MockClient
		reconciler reconcile.Reconciler

		namespace   = "garden-dev"
		bastionName = "bastion"
		shootName   = "myshoot"
		seedName    = "myseed"
		ctx         = context.TODO()
		maxLifetime = 12 * time.Hour
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mockClient = mockclient.NewMockClient(mockCtrl)
		reconciler = &Reconciler{
			Client: mockClient,
			Config: config.BastionControllerConfiguration{
				MaxLifetime: &metav1.Duration{Duration: maxLifetime},
			},
			Clock: clock.RealClock{},
		}
	})

	AfterEach(func() {
		mockCtrl.Finish()
	})

	Describe("Reconciler", func() {
		It("should return nil because object not found", func() {
			mockClient.EXPECT().Get(ctx, kubernetesutils.Key(namespace, bastionName), gomock.AssignableToTypeOf(&operationsv1alpha1.Bastion{})).Return(apierrors.NewNotFound(schema.GroupResource{}, ""))

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: kubernetesutils.Key(namespace, bastionName)})
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should requeue alive Bastions", func() {
			created := time.Now().Add(-maxLifetime / 2)
			requeueAfter := time.Until(created.Add(maxLifetime))

			mockClient.EXPECT().Get(ctx, kubernetesutils.Key(namespace, shootName), gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *gardencorev1beta1.Shoot, _ ...client.GetOption) error {
				*obj = newShoot(namespace, shootName, &seedName)
				return nil
			})

			mockClient.EXPECT().Get(ctx, kubernetesutils.Key(namespace, bastionName), gomock.AssignableToTypeOf(&operationsv1alpha1.Bastion{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *operationsv1alpha1.Bastion, _ ...client.GetOption) error {
				*obj = newBastion(namespace, bastionName, shootName, &seedName, &created, nil)
				return nil
			})

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: kubernetesutils.Key(namespace, bastionName)})
			Expect(result.RequeueAfter).To(BeNumerically("~", requeueAfter, 1*time.Second))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should requeue soon-to-expire Bastions", func() {
			now := time.Now()
			remaining := 30 * time.Second
			expires := now.Add(remaining)

			mockClient.EXPECT().Get(ctx, kubernetesutils.Key(namespace, shootName), gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *gardencorev1beta1.Shoot, _ ...client.GetOption) error {
				*obj = newShoot(namespace, shootName, &seedName)
				return nil
			})

			mockClient.EXPECT().Get(ctx, kubernetesutils.Key(namespace, bastionName), gomock.AssignableToTypeOf(&operationsv1alpha1.Bastion{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *operationsv1alpha1.Bastion, _ ...client.GetOption) error {
				*obj = newBastion(namespace, bastionName, shootName, &seedName, &now, &expires)
				return nil
			})

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: kubernetesutils.Key(namespace, bastionName)})
			Expect(result.RequeueAfter).To(BeNumerically("~", remaining, 1*time.Second))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should requeue soon-to-reach-max-lifetime Bastions", func() {
			mockClient.EXPECT().Get(ctx, kubernetesutils.Key(namespace, shootName), gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *gardencorev1beta1.Shoot, _ ...client.GetOption) error {
				*obj = newShoot(namespace, shootName, &seedName)
				return nil
			})

			mockClient.EXPECT().Get(ctx, kubernetesutils.Key(namespace, bastionName), gomock.AssignableToTypeOf(&operationsv1alpha1.Bastion{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *operationsv1alpha1.Bastion, _ ...client.GetOption) error {
				now := time.Now()
				created := now.Add(-maxLifetime).Add(10 * time.Minute) // reaches end-of-life in 10 minutes
				expires := now.Add(30 * time.Minute)                   // expires in 30 minutes

				*obj = newBastion(namespace, bastionName, shootName, &seedName, &created, &expires)
				return nil
			})

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: kubernetesutils.Key(namespace, bastionName)})
			Expect(result.RequeueAfter).To(BeNumerically("~", 10*time.Minute, 1*time.Second))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should delete Bastions with missing shoots", func() {
			mockClient.EXPECT().Get(ctx, kubernetesutils.Key(namespace, shootName), gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{})).Return(apierrors.NewNotFound(schema.GroupResource{}, ""))

			mockClient.EXPECT().Get(ctx, kubernetesutils.Key(namespace, bastionName), gomock.AssignableToTypeOf(&operationsv1alpha1.Bastion{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *operationsv1alpha1.Bastion, _ ...client.GetOption) error {
				created := time.Now().Add(-maxLifetime / 2)

				*obj = newBastion(namespace, bastionName, shootName, &seedName, &created, nil)
				return nil
			})

			mockClient.EXPECT().Delete(ctx, gomock.AssignableToTypeOf(&operationsv1alpha1.Bastion{}))

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: kubernetesutils.Key(namespace, bastionName)})
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should delete Bastions with shoots in deletion", func() {
			mockClient.EXPECT().Get(ctx, kubernetesutils.Key(namespace, shootName), gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *gardencorev1beta1.Shoot, _ ...client.GetOption) error {
				shoot := newShoot(namespace, shootName, &seedName)
				now := metav1.Now()
				shoot.DeletionTimestamp = &now
				*obj = shoot
				return nil
			})

			mockClient.EXPECT().Get(ctx, kubernetesutils.Key(namespace, bastionName), gomock.AssignableToTypeOf(&operationsv1alpha1.Bastion{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *operationsv1alpha1.Bastion, _ ...client.GetOption) error {
				created := time.Now().Add(-maxLifetime / 2)

				*obj = newBastion(namespace, bastionName, shootName, &seedName, &created, nil)
				return nil
			})

			mockClient.EXPECT().Delete(ctx, gomock.AssignableToTypeOf(&operationsv1alpha1.Bastion{}))

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: kubernetesutils.Key(namespace, bastionName)})
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should delete expired Bastions", func() {
			mockClient.EXPECT().Get(ctx, kubernetesutils.Key(namespace, shootName), gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *gardencorev1beta1.Shoot, _ ...client.GetOption) error {
				*obj = newShoot(namespace, shootName, &seedName)
				return nil
			})

			mockClient.EXPECT().Get(ctx, kubernetesutils.Key(namespace, bastionName), gomock.AssignableToTypeOf(&operationsv1alpha1.Bastion{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *operationsv1alpha1.Bastion, _ ...client.GetOption) error {
				created := time.Now().Add(-maxLifetime / 2)
				expires := time.Now().Add(-5 * time.Second)

				*obj = newBastion(namespace, bastionName, shootName, &seedName, &created, &expires)
				return nil
			})

			mockClient.EXPECT().Delete(ctx, gomock.AssignableToTypeOf(&operationsv1alpha1.Bastion{}))

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: kubernetesutils.Key(namespace, bastionName)})
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should delete Bastions that have reached their TTL", func() {
			mockClient.EXPECT().Get(ctx, kubernetesutils.Key(namespace, shootName), gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *gardencorev1beta1.Shoot, _ ...client.GetOption) error {
				*obj = newShoot(namespace, shootName, &seedName)
				return nil
			})

			mockClient.EXPECT().Get(ctx, kubernetesutils.Key(namespace, bastionName), gomock.AssignableToTypeOf(&operationsv1alpha1.Bastion{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *operationsv1alpha1.Bastion, _ ...client.GetOption) error {
				created := time.Now().Add(-maxLifetime * 2)

				*obj = newBastion(namespace, bastionName, shootName, &seedName, &created, nil)
				return nil
			})

			mockClient.EXPECT().Delete(ctx, gomock.AssignableToTypeOf(&operationsv1alpha1.Bastion{}))

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: kubernetesutils.Key(namespace, bastionName)})
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should delete Bastions with outdated seed information", func() {
			newSeedName := "new-seed-after-migration"

			mockClient.EXPECT().Get(ctx, kubernetesutils.Key(namespace, shootName), gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *gardencorev1beta1.Shoot, _ ...client.GetOption) error {
				*obj = newShoot(namespace, shootName, &newSeedName)
				return nil
			})

			mockClient.EXPECT().Get(ctx, kubernetesutils.Key(namespace, bastionName), gomock.AssignableToTypeOf(&operationsv1alpha1.Bastion{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *operationsv1alpha1.Bastion, _ ...client.GetOption) error {
				created := time.Now().Add(-maxLifetime / 2)
				expires := time.Now().Add(1 * time.Minute)

				*obj = newBastion(namespace, bastionName, shootName, &seedName, &created, &expires)
				return nil
			})

			mockClient.EXPECT().Delete(ctx, gomock.AssignableToTypeOf(&operationsv1alpha1.Bastion{}))

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: kubernetesutils.Key(namespace, bastionName)})
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should delete Bastions with outdated seed information 2", func() {
			mockClient.EXPECT().Get(ctx, kubernetesutils.Key(namespace, shootName), gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *gardencorev1beta1.Shoot, _ ...client.GetOption) error {
				*obj = newShoot(namespace, shootName, nil) // shoot was removed from original seed and since then hasn't found a new seed
				return nil
			})

			mockClient.EXPECT().Get(ctx, kubernetesutils.Key(namespace, bastionName), gomock.AssignableToTypeOf(&operationsv1alpha1.Bastion{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *operationsv1alpha1.Bastion, _ ...client.GetOption) error {
				created := time.Now().Add(-maxLifetime / 2)
				expires := time.Now().Add(1 * time.Minute)

				*obj = newBastion(namespace, bastionName, shootName, &seedName, &created, &expires)
				return nil
			})

			mockClient.EXPECT().Delete(ctx, gomock.AssignableToTypeOf(&operationsv1alpha1.Bastion{}))

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: kubernetesutils.Key(namespace, bastionName)})
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(err).NotTo(HaveOccurred())
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
		bastion.ObjectMeta.CreationTimestamp = metav1.NewTime(*createdAt)
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

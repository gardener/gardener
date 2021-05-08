// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package bastion

import (
	"context"
	"time"

	operationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
	"github.com/gardener/gardener/pkg/logger"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("Controller", func() {
	var (
		mockCtrl   *gomock.Controller
		mockClient *mockclient.MockClient
		reconciler reconcile.Reconciler

		bastionName = "bastion"
		ctx         = context.TODO()
		log         = logger.NewNopLogger()
		maxLifetime = 12 * time.Hour
	)

	BeforeEach(func() {
		mockCtrl = gomock.NewController(GinkgoT())
		mockClient = mockclient.NewMockClient(mockCtrl)
		reconciler = NewBastionReconciler(log, mockClient, maxLifetime)
	})

	Describe("Reconciler", func() {
		It("should return nil because object not found", func() {
			mockClient.EXPECT().Get(ctx, kutil.Key(bastionName), gomock.AssignableToTypeOf(&operationsv1alpha1.Bastion{})).Return(apierrors.NewNotFound(schema.GroupResource{}, ""))

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: bastionName}})
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(err).To(Succeed())
			mockCtrl.Finish()
		})

		It("should requeue alive Bastions", func() {
			created := time.Now().Add(-maxLifetime / 2)
			requeueAfter := time.Until(created.Add(maxLifetime))

			mockClient.EXPECT().Get(ctx, kutil.Key(bastionName), gomock.AssignableToTypeOf(&operationsv1alpha1.Bastion{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *operationsv1alpha1.Bastion) error {
				*obj = newBastion(bastionName, &created, nil)
				return nil
			})

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: bastionName}})
			Expect(result.RequeueAfter).To(BeNumerically("~", requeueAfter, 1*time.Second))
			Expect(err).To(Succeed())
			mockCtrl.Finish()
		})

		It("should requeue soon-to-expire Bastions", func() {
			now := time.Now()
			remaining := 30 * time.Second
			expires := now.Add(remaining)

			mockClient.EXPECT().Get(ctx, kutil.Key(bastionName), gomock.AssignableToTypeOf(&operationsv1alpha1.Bastion{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *operationsv1alpha1.Bastion) error {
				*obj = newBastion(bastionName, &now, &expires)
				return nil
			})

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: bastionName}})
			Expect(result.RequeueAfter).To(BeNumerically("~", remaining, 1*time.Second))
			Expect(err).To(Succeed())
			mockCtrl.Finish()
		})

		It("should requeue soon-to-reach-max-lifetime Bastions", func() {
			mockClient.EXPECT().Get(ctx, kutil.Key(bastionName), gomock.AssignableToTypeOf(&operationsv1alpha1.Bastion{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *operationsv1alpha1.Bastion) error {
				now := time.Now()
				created := now.Add(-maxLifetime).Add(10 * time.Minute) // reaches end-of-life in 10 minutes
				expires := now.Add(30 * time.Minute)                   // expires in 30 minutes

				*obj = newBastion(bastionName, &created, &expires)
				return nil
			})

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: bastionName}})
			Expect(result.RequeueAfter).To(BeNumerically("~", 10*time.Minute, 1*time.Second))
			Expect(err).To(Succeed())
			mockCtrl.Finish()
		})

		It("should delete expired Bastions", func() {
			mockClient.EXPECT().Get(ctx, kutil.Key(bastionName), gomock.AssignableToTypeOf(&operationsv1alpha1.Bastion{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *operationsv1alpha1.Bastion) error {
				created := time.Now().Add(-maxLifetime / 2)
				expires := time.Now().Add(-5 * time.Second)

				*obj = newBastion(bastionName, &created, &expires)
				return nil
			})

			mockClient.EXPECT().Delete(ctx, gomock.AssignableToTypeOf(&operationsv1alpha1.Bastion{}))

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: bastionName}})
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(err).To(Succeed())
			mockCtrl.Finish()
		})

		It("should delete Bastions that have reached their TTL", func() {
			mockClient.EXPECT().Get(ctx, kutil.Key(bastionName), gomock.AssignableToTypeOf(&operationsv1alpha1.Bastion{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *operationsv1alpha1.Bastion) error {
				created := time.Now().Add(-maxLifetime * 2)

				*obj = newBastion(bastionName, &created, nil)
				return nil
			})

			mockClient.EXPECT().Delete(ctx, gomock.AssignableToTypeOf(&operationsv1alpha1.Bastion{}))

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: bastionName}})
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(err).To(Succeed())
			mockCtrl.Finish()
		})
	})
})

func newBastion(name string, createdAt *time.Time, expiresAt *time.Time) operationsv1alpha1.Bastion {
	bastion := operationsv1alpha1.Bastion{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
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

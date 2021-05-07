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
		})

		It("should delete expired Bastions", func() {
			mockClient.EXPECT().Get(ctx, kutil.Key(bastionName), gomock.AssignableToTypeOf(&operationsv1alpha1.Bastion{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *operationsv1alpha1.Bastion) error {
				created := time.Now().Add(-maxLifetime / 2)
				expires := metav1.NewTime(time.Now().Add(-5 * time.Second))
				*obj = operationsv1alpha1.Bastion{
					ObjectMeta: metav1.ObjectMeta{
						Name:              bastionName,
						CreationTimestamp: metav1.NewTime(created),
					},
					Status: operationsv1alpha1.BastionStatus{
						ExpirationTimestamp: &expires,
					},
				}

				return nil
			})

			deleted := false
			mockClient.EXPECT().Delete(ctx, gomock.AssignableToTypeOf(&operationsv1alpha1.Bastion{})).Do(func(_ context.Context, _ *operationsv1alpha1.Bastion) {
				deleted = true
			})

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: bastionName}})
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(err).To(Succeed())
			Expect(deleted).To(BeTrue())
		})

		It("should delete Bastions that have reached their TTL", func() {
			mockClient.EXPECT().Get(ctx, kutil.Key(bastionName), gomock.AssignableToTypeOf(&operationsv1alpha1.Bastion{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *operationsv1alpha1.Bastion) error {
				created := time.Now().Add(-maxLifetime * 2)
				*obj = operationsv1alpha1.Bastion{
					ObjectMeta: metav1.ObjectMeta{
						Name:              bastionName,
						CreationTimestamp: metav1.NewTime(created),
					},
				}

				return nil
			})

			deleted := false
			mockClient.EXPECT().Delete(ctx, gomock.AssignableToTypeOf(&operationsv1alpha1.Bastion{})).Do(func(_ context.Context, _ *operationsv1alpha1.Bastion) {
				deleted = true
			})

			result, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: bastionName}})
			Expect(result).To(Equal(reconcile.Result{}))
			Expect(err).To(Succeed())
			Expect(deleted).To(BeTrue())
		})
	})
})

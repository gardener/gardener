// Copyright (c) 2021 SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package secretbinding

import (
	"context"
	"fmt"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("SecretBindingControl", func() {
	var (
		ctrl *gomock.Controller
		c    *mockclient.MockClient

		ctx     = context.TODO()
		fakeErr = fmt.Errorf("fake err")
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#mayReleaseSecret", func() {
		var (
			reconciler *secretBindingReconciler

			secretBinding1Namespace = "foo"
			secretBinding1Name      = "bar"
			secretBinding2Namespace = "baz"
			secretBinding2Name      = "bax"
			secretNamespace         = "foo"
			secretName              = "bar"
		)

		BeforeEach(func() {
			reconciler = &secretBindingReconciler{gardenClient: c}
		})

		It("should return true as no other secretbinding exists", func() {
			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&gardencorev1beta1.SecretBindingList{}))

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

			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&gardencorev1beta1.SecretBindingList{})).DoAndReturn(func(_ context.Context, list *gardencorev1beta1.SecretBindingList, _ ...client.ListOption) error {
				(&gardencorev1beta1.SecretBindingList{Items: []gardencorev1beta1.SecretBinding{*secretBinding}}).DeepCopyInto(list)
				return nil
			})

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

			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&gardencorev1beta1.SecretBindingList{})).DoAndReturn(func(_ context.Context, list *gardencorev1beta1.SecretBindingList, _ ...client.ListOption) error {
				(&gardencorev1beta1.SecretBindingList{Items: []gardencorev1beta1.SecretBinding{*secretBinding}}).DeepCopyInto(list)
				return nil
			})

			allowed, err := reconciler.mayReleaseSecret(ctx, secretBinding1Namespace, secretBinding1Name, secretNamespace, secretName)

			Expect(allowed).To(BeFalse())
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return an error as the list failed", func() {
			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&gardencorev1beta1.SecretBindingList{})).Return(fakeErr)

			allowed, err := reconciler.mayReleaseSecret(ctx, secretBinding1Namespace, secretBinding1Name, secretNamespace, secretName)

			Expect(allowed).To(BeFalse())
			Expect(err).To(MatchError(fakeErr))
		})
	})

	Describe("SecretBinding label for Secrets", func() {
		var (
			reconciler *secretBindingReconciler
			request    reconcile.Request

			secretBindingNamespace = "foo"
			secretBindingName      = "bar"
			secretNamespace        = "foo"
			secretName             = "bar"

			secretBinding = &gardencorev1beta1.SecretBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretBindingName,
					Namespace: secretBindingNamespace,
				},
				SecretRef: corev1.SecretReference{
					Namespace: secretNamespace,
					Name:      secretName,
				},
			}

			secret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretName,
					Namespace: secretNamespace,
				},
			}
		)

		BeforeEach(func() {
			reconciler = &secretBindingReconciler{gardenClient: c}
			request = reconcile.Request{NamespacedName: types.NamespacedName{Namespace: secretBindingNamespace, Name: secretBindingName}}

			c.EXPECT().Get(ctx, gomock.Any(), gomock.AssignableToTypeOf(&gardencorev1beta1.SecretBinding{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *gardencorev1beta1.SecretBinding) error {
				secretBinding.DeepCopyInto(obj)
				return nil
			})

			c.EXPECT().Get(ctx, gomock.Any(), gomock.AssignableToTypeOf(&corev1.Secret{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *corev1.Secret) error {
				secret.DeepCopyInto(obj)
				return nil
			})

			c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&gardencorev1beta1.SecretBinding{}), gomock.Any()).DoAndReturn(
				func(_ context.Context, sb *gardencorev1beta1.SecretBinding, _ client.Patch, _ ...client.PatchOption) error {
					*secretBinding = *sb
					return nil
				},
			).AnyTimes()

			c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&corev1.Secret{}), gomock.Any()).DoAndReturn(
				func(_ context.Context, s *corev1.Secret, _ client.Patch, _ ...client.PatchOption) error {
					*secret = *s
					return nil
				},
			).AnyTimes()
		})

		It("should add the label to the secret referred by the secretbinding", func() {
			_, err := reconciler.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())

			expectedLabels := map[string]string{
				"secretbinding.gardener.cloud/referred": "true",
			}

			Expect(secret.ObjectMeta.Labels).To(Equal(expectedLabels))
		})

		It("should remove the label from the secret when there are no secretbindings referring it", func() {
			secretBinding.DeletionTimestamp = &metav1.Time{Time: time.Date(1, 1, 1, 1, 1, 1, 1, time.UTC)}

			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&gardencorev1beta1.SecretBindingList{})).DoAndReturn(func(_ context.Context, list *gardencorev1beta1.SecretBindingList, _ ...client.ListOption) error {
				(&gardencorev1beta1.SecretBindingList{Items: []gardencorev1beta1.SecretBinding{*secretBinding}}).DeepCopyInto(list)
				return nil
			})

			c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&gardencorev1beta1.ShootList{})).DoAndReturn(func(_ context.Context, list *gardencorev1beta1.ShootList, _ ...client.ListOption) error {
				(&gardencorev1beta1.ShootList{}).DeepCopyInto(list)
				return nil
			})

			_, err := reconciler.Reconcile(ctx, request)
			Expect(err).NotTo(HaveOccurred())

			Expect(len(secret.ObjectMeta.Labels)).To(Equal(0))
		})
	})
})

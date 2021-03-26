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

package gardener_test

import (
	"context"
	"errors"
	"time"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	mocktime "github.com/gardener/gardener/pkg/mock/go/time"
	. "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/test"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("DeletionConfirmation", func() {
	Describe("#CheckIfDeletionIsConfirmed", func() {
		It("should prevent the deletion due to missing annotations", func() {
			obj := &corev1.Namespace{}

			Expect(CheckIfDeletionIsConfirmed(obj)).To(HaveOccurred())
		})

		It("should prevent the deletion due annotation value != true", func() {
			obj := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						ConfirmationDeletion: "false",
					},
				},
			}

			Expect(CheckIfDeletionIsConfirmed(obj)).To(HaveOccurred())
		})

		It("should allow the deletion due annotation value == true", func() {
			obj := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						ConfirmationDeletion: "true",
					},
				},
			}

			Expect(CheckIfDeletionIsConfirmed(obj)).To(Succeed())
		})
	})

	Describe("#ConfirmDeletion", func() {
		var (
			ctrl    *gomock.Controller
			c       *mockclient.MockClient
			now     time.Time
			mockNow *mocktime.MockNow
		)

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
			mockNow = mocktime.NewMockNow(ctrl)
			c = mockclient.NewMockClient(ctrl)
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		It("should add the deletion confirmation annotation for an object without annotations", func() {
			var (
				ctx = context.TODO()
				obj = &corev1.Namespace{}
			)

			defer test.WithVars(&TimeNow, mockNow.Do)()

			expectedObj := obj.DeepCopy()
			expectedObj.Annotations = map[string]string{ConfirmationDeletion: "true", v1beta1constants.GardenerTimestamp: now.UTC().String()}

			mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

			c.EXPECT().Get(ctx, gomock.AssignableToTypeOf(client.ObjectKey{}), obj)
			c.EXPECT().Update(ctx, expectedObj)

			Expect(ConfirmDeletion(ctx, c, obj)).To(Succeed())
		})

		It("should add the deletion confirmation annotation for an object with annotations", func() {
			var (
				ctx = context.TODO()
				obj = &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							"foo": "bar",
						},
					},
				}
			)

			defer test.WithVars(&TimeNow, mockNow.Do)()

			expectedObj := obj.DeepCopy()
			expectedObj.Annotations[ConfirmationDeletion] = "true"
			expectedObj.Annotations[v1beta1constants.GardenerTimestamp] = now.UTC().String()

			mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()
			c.EXPECT().Get(ctx, gomock.AssignableToTypeOf(client.ObjectKey{}), obj)
			c.EXPECT().Update(ctx, expectedObj)

			Expect(ConfirmDeletion(ctx, c, obj)).To(Succeed())
		})

		It("should ignore non-existing objects", func() {
			var (
				ctx         = context.TODO()
				obj         = &corev1.Namespace{}
				expectedObj = obj.DeepCopy()
			)

			c.EXPECT().Get(ctx, gomock.AssignableToTypeOf(client.ObjectKey{}), obj).Return(apierrors.NewNotFound(corev1.Resource("namespaces"), ""))

			Expect(ConfirmDeletion(ctx, c, obj)).To(Succeed())
			Expect(obj).To(Equal(expectedObj))
		})

		It("should retry on conflict and add the deletion confirmation annotation", func() {
			var (
				ctx     = context.TODO()
				baseObj = &corev1.Namespace{}
				obj     = baseObj.DeepCopy()
			)

			defer test.WithVars(&TimeNow, mockNow.Do)()

			expectedObj := obj.DeepCopy()
			expectedObj.Annotations = map[string]string{ConfirmationDeletion: "true", v1beta1constants.GardenerTimestamp: now.UTC().String()}

			mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()
			c.EXPECT().Get(ctx, gomock.AssignableToTypeOf(client.ObjectKey{}), obj)
			c.EXPECT().Update(ctx, expectedObj).Return(apierrors.NewConflict(corev1.Resource("namespaces"), "", errors.New("conflict")))
			c.EXPECT().Get(ctx, gomock.AssignableToTypeOf(client.ObjectKey{}), expectedObj).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj client.Object) error {
				baseObj.DeepCopyInto(obj.(*corev1.Namespace))
				return nil
			})
			c.EXPECT().Update(ctx, expectedObj)

			Expect(ConfirmDeletion(ctx, c, obj)).To(Succeed())
		})
	})
})

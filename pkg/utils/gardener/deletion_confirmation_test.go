// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardener_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	. "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	mocktime "github.com/gardener/gardener/third_party/mock/go/time"
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
						v1beta1constants.ConfirmationDeletion: "false",
					},
				},
			}

			Expect(CheckIfDeletionIsConfirmed(obj)).To(HaveOccurred())
		})

		It("should allow the deletion due annotation value == true", func() {
			obj := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						v1beta1constants.ConfirmationDeletion: "true",
					},
				},
			}

			Expect(CheckIfDeletionIsConfirmed(obj)).To(Succeed())
		})
	})

	Describe("#ConfirmDeletion", func() {
		var (
			ctx     context.Context
			ctrl    *gomock.Controller
			c       client.Client
			now     time.Time
			mockNow *mocktime.MockNow
			obj     client.Object
		)

		BeforeEach(func() {
			ctx = context.Background()
			ctrl = gomock.NewController(GinkgoT())
			mockNow = mocktime.NewMockNow(ctrl)
			obj = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "foo"}}
			c = fake.NewClientBuilder().WithObjects(obj).Build()
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		It("should add the deletion confirmation annotation for an object without annotations", func() {
			DeferCleanup(test.WithVars(
				&TimeNow, mockNow.Do,
			))
			mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

			expectedAnnotations := map[string]string{v1beta1constants.ConfirmationDeletion: "true", v1beta1constants.GardenerTimestamp: now.UTC().Format(time.RFC3339Nano)}

			Expect(ConfirmDeletion(ctx, c, obj)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(obj), obj)).To(Succeed())
			Expect(obj.GetAnnotations()).To(Equal(expectedAnnotations))
		})

		It("should add the deletion confirmation annotation for an object with annotations", func() {
			DeferCleanup(test.WithVars(
				&TimeNow, mockNow.Do,
			))
			mockNow.EXPECT().Do().Return(now.UTC()).AnyTimes()

			obj.SetAnnotations(map[string]string{"foo": "bar"})
			Expect(c.Update(ctx, obj)).To(Succeed())

			expectedAnnotations := map[string]string{"foo": "bar", v1beta1constants.ConfirmationDeletion: "true", v1beta1constants.GardenerTimestamp: now.UTC().Format(time.RFC3339Nano)}

			Expect(ConfirmDeletion(ctx, c, obj)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(obj), obj)).To(Succeed())
			Expect(obj.GetAnnotations()).To(Equal(expectedAnnotations))
		})

		It("should fail for non-existing objects", func() {
			Expect(c.Delete(ctx, obj)).To(Succeed())

			Expect(ConfirmDeletion(ctx, c, obj)).To(BeNotFoundError())
		})
	})
})

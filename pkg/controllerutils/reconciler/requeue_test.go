// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package reconciler_test

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	. "github.com/gardener/gardener/pkg/controllerutils/reconciler"
	"github.com/gardener/gardener/pkg/controllerutils/reconciler/mock"
	mockworkqueue "github.com/gardener/gardener/third_party/mock/client-go/util/workqueue"
)

var _ = Describe("Requeue", func() {
	var (
		cause        = fmt.Errorf("cause")
		requeueAfter = time.Hour
	)

	DescribeTable("#Error",
		func(err *RequeueAfterError, expectedMsg string) {
			Expect(err.Error()).To(Equal(expectedMsg))
		},

		Entry("w/o cause", &RequeueAfterError{RequeueAfter: requeueAfter}, "requeue in "+requeueAfter.String()),
		Entry("w/ cause", &RequeueAfterError{Cause: cause, RequeueAfter: requeueAfter}, "requeue in "+requeueAfter.String()+" due to "+cause.Error()),
	)

	Describe("RateLimitedRequeueReconcilerAdapter", func() {
		var (
			ctx context.Context

			ctrl                  *gomock.Controller
			mockRequeueReconciler *mock.MockRequeueReconciler
			mockRateLimiter       *mockworkqueue.MockTypedRateLimiter[reconcile.Request]

			request                  reconcile.Request
			requeueReconcilerAdapter reconcile.Reconciler
		)

		BeforeEach(func() {
			ctx = context.Background()

			ctrl = gomock.NewController(GinkgoT())
			mockRequeueReconciler = mock.NewMockRequeueReconciler(ctrl)
			mockRateLimiter = mockworkqueue.NewMockTypedRateLimiter[reconcile.Request](ctrl)

			request = reconcile.Request{NamespacedName: types.NamespacedName{Namespace: "foo", Name: "bar"}}
			requeueReconcilerAdapter = RateLimitedRequeueReconcilerAdapter(mockRequeueReconciler, mockRateLimiter)
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		Context("when reconciler returns an error", func() {
			var testErr = fmt.Errorf("test error")

			It("should return the error immediately without touching rate limiter", func() {
				mockRequeueReconciler.EXPECT().Reconcile(ctx, request).Return(false, reconcile.Result{}, testErr)
				// No expectations on rate limiter - it shouldn't be called at all

				result, err := requeueReconcilerAdapter.Reconcile(ctx, request)

				Expect(err).To(Equal(testErr))
				Expect(result).To(Equal(reconcile.Result{}))
			})
		})

		Context("when result does not have RequeueAfter set", func() {
			Context("when requeue is false", func() {
				It("should return the result and call Forget on rate limiter", func() {
					mockRequeueReconciler.EXPECT().Reconcile(ctx, request).Return(false, reconcile.Result{}, nil)
					mockRateLimiter.EXPECT().Forget(request)

					result, err := requeueReconcilerAdapter.Reconcile(ctx, request)

					Expect(err).NotTo(HaveOccurred())
					Expect(result).To(Equal(reconcile.Result{}))
				})
			})

			Context("when requeue is true", func() {
				It("should apply rate limiting", func() {
					mockRequeueReconciler.EXPECT().Reconcile(ctx, request).Return(true, reconcile.Result{}, nil)
					mockRateLimiter.EXPECT().When(request).Return(100 * time.Millisecond)

					result, err := requeueReconcilerAdapter.Reconcile(ctx, request)

					Expect(err).NotTo(HaveOccurred())
					Expect(result.RequeueAfter).To(Equal(100 * time.Millisecond))
				})

				It("should increase rate limiting on subsequent failures", func() {
					// First reconcile
					mockRequeueReconciler.EXPECT().Reconcile(ctx, request).Return(true, reconcile.Result{}, nil)
					mockRateLimiter.EXPECT().When(request).Return(100 * time.Millisecond)

					result1, err1 := requeueReconcilerAdapter.Reconcile(ctx, request)

					Expect(err1).NotTo(HaveOccurred())
					Expect(result1.RequeueAfter).To(Equal(100 * time.Millisecond))

					// Second reconcile - with longer delay
					mockRequeueReconciler.EXPECT().Reconcile(ctx, request).Return(true, reconcile.Result{}, nil)
					mockRateLimiter.EXPECT().When(request).Return(200 * time.Millisecond)

					result2, err2 := requeueReconcilerAdapter.Reconcile(ctx, request)

					Expect(err2).NotTo(HaveOccurred())
					Expect(result2.RequeueAfter).To(Equal(200 * time.Millisecond))
				})
			})
		})

		Context("when result has RequeueAfter set", func() {
			var requeueAfterDuration = 10 * time.Minute

			var test = func(requeue bool) {
				It("should return the result as-is and call Forget on rate limiter", func() {
					mockRequeueReconciler.EXPECT().Reconcile(ctx, request).Return(requeue, reconcile.Result{RequeueAfter: requeueAfterDuration}, nil)
					mockRateLimiter.EXPECT().Forget(request)

					result, err := requeueReconcilerAdapter.Reconcile(ctx, request)

					Expect(err).NotTo(HaveOccurred())
					Expect(result).To(Equal(reconcile.Result{RequeueAfter: requeueAfterDuration}))
				})
			}

			Context("when requeue is false", func() {
				test(false)
			})

			Context("when requeue is true", func() {
				test(true)
			})
		})
	})
})

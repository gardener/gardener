// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package retry_test

import (
	"errors"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	"go.uber.org/mock/gomock"

	mockutilcontext "github.com/gardener/gardener/pkg/utils/context/mock"
	. "github.com/gardener/gardener/pkg/utils/retry"
	mockretry "github.com/gardener/gardener/pkg/utils/retry/mock"
	mockcontext "github.com/gardener/gardener/third_party/mock/go/context"
)

var _ = Describe("Retry", func() {
	var (
		ctrl       *gomock.Controller
		closedChan <-chan struct{}
		openChan   <-chan struct{}
	)
	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		closedChan = func() <-chan struct{} {
			c := make(chan struct{})
			close(c)
			return c
		}()
		openChan = make(chan struct{})
	})
	AfterEach(func() {
		ctrl.Finish()
	})

	Context("LastErrorAggregator", func() {
		It("should return the last minor error", func() {
			var (
				err1 = fmt.Errorf("error 1")
				err2 = fmt.Errorf("error 2")
				agg  = NewLastErrorAggregator()
			)

			agg.Minor(err1)
			agg.Minor(err2)
			Expect(agg.Error()).To(BeIdenticalTo(err2))
		})

		It("should return the severe error", func() {
			var (
				err1 = fmt.Errorf("error 1")
				err2 = fmt.Errorf("error 2")
				agg  = NewLastErrorAggregator()
			)

			agg.Minor(err1)
			agg.Severe(err2)
			Expect(agg.Error()).To(BeIdenticalTo(err2))
		})

		It("should return nil if no error was given", func() {
			Expect(NewLastErrorAggregator().Error()).To(Succeed())
		})
	})

	Describe("#UntilFor", func() {
		It("should succeed", func() {
			var (
				ctx      = mockcontext.NewMockContext(ctrl)
				waitFunc = mockretry.NewMockWaitFunc(ctrl)
				agg      = mockretry.NewMockErrorAggregator(ctrl)
				f        = mockretry.NewMockFunc(ctrl)
			)

			f.EXPECT().Do(ctx).Return(Ok())

			Expect(UntilFor(ctx, waitFunc.Do, agg, f.Do)).To(Succeed())
		})

		It("should retry, wait and succeed", func() {
			var (
				ctx            = mockcontext.NewMockContext(ctrl)
				waitFunc       = mockretry.NewMockWaitFunc(ctrl)
				waitCtx        = mockcontext.NewMockContext(ctrl)
				waitCancelFunc = mockcontext.NewMockCancelFunc(ctrl)
				agg            = mockretry.NewMockErrorAggregator(ctrl)
				f              = mockretry.NewMockFunc(ctrl)
				minorErr       = fmt.Errorf("minor error")
			)

			gomock.InOrder(
				f.EXPECT().Do(ctx).Return(MinorError(minorErr)),
				agg.EXPECT().Minor(minorErr),

				waitFunc.EXPECT().Do(ctx).Return(waitCtx, waitCancelFunc.Do),

				waitCtx.EXPECT().Done().Return(closedChan),
				ctx.EXPECT().Done().Return(openChan),
				waitCancelFunc.EXPECT().Do(),

				f.EXPECT().Do(ctx).Return(Ok()),
			)

			Expect(UntilFor(ctx, waitFunc.Do, agg, f.Do)).To(Succeed())
		})

		It("should fail immediately with a severe error", func() {
			var (
				ctx       = mockcontext.NewMockContext(ctrl)
				waitFunc  = mockretry.NewMockWaitFunc(ctrl)
				agg       = mockretry.NewMockErrorAggregator(ctrl)
				f         = mockretry.NewMockFunc(ctrl)
				severeErr = fmt.Errorf("severe error")
			)

			gomock.InOrder(
				f.EXPECT().Do(ctx).Return(SevereError(severeErr)),
				agg.EXPECT().Severe(severeErr),
				agg.EXPECT().Error().Return(severeErr),
			)

			Expect(UntilFor(ctx, waitFunc.Do, agg, f.Do)).To(BeIdenticalTo(severeErr))
		})

		It("should fail after a timeout with a retry error containing the last error", func() {
			var (
				ctx            = mockcontext.NewMockContext(ctrl)
				waitFunc       = mockretry.NewMockWaitFunc(ctrl)
				waitCtx        = mockcontext.NewMockContext(ctrl)
				waitCancelFunc = mockcontext.NewMockCancelFunc(ctrl)
				agg            = mockretry.NewMockErrorAggregator(ctrl)
				f              = mockretry.NewMockFunc(ctrl)

				minorErr = fmt.Errorf("minor error")
				ctxErr   = fmt.Errorf("ctx error")
			)

			gomock.InOrder(
				f.EXPECT().Do(ctx).Return(MinorError(minorErr)),
				agg.EXPECT().Minor(minorErr),

				waitFunc.EXPECT().Do(ctx).Return(waitCtx, waitCancelFunc.Do),

				waitCtx.EXPECT().Done().Return(openChan),
				ctx.EXPECT().Done().Return(closedChan),
				ctx.EXPECT().Err().Return(ctxErr),
				agg.EXPECT().Error().Return(minorErr),
				waitCancelFunc.EXPECT().Do(),
			)

			Expect(UntilFor(ctx, waitFunc.Do, agg, f.Do)).To(Equal(NewError(ctxErr, minorErr)))
		})

		It("should always fail with a timeout when both regular and wait context are expired", func() {
			var (
				ctx            = mockcontext.NewMockContext(ctrl)
				waitFunc       = mockretry.NewMockWaitFunc(ctrl)
				waitCtx        = mockcontext.NewMockContext(ctrl)
				waitCancelFunc = mockcontext.NewMockCancelFunc(ctrl)
				agg            = mockretry.NewMockErrorAggregator(ctrl)
				f              = mockretry.NewMockFunc(ctrl)

				minorErr = fmt.Errorf("minor error")
				ctxErr   = fmt.Errorf("ctx error")
			)

			gomock.InOrder(
				f.EXPECT().Do(ctx).Return(MinorError(minorErr)),
				agg.EXPECT().Minor(minorErr),

				waitFunc.EXPECT().Do(ctx).Return(waitCtx, waitCancelFunc.Do),

				waitCtx.EXPECT().Done().Return(closedChan),
				ctx.EXPECT().Done().Return(closedChan),
				ctx.EXPECT().Err().Return(ctxErr),
				agg.EXPECT().Error().Return(minorErr),
				waitCancelFunc.EXPECT().Do(),
			)

			Expect(UntilFor(ctx, waitFunc.Do, agg, f.Do)).To(Equal(NewError(ctxErr, minorErr)))
		})
	})

	Context("IntervalFactory", func() {
		Describe("#New", func() {
			It("should return a context with the given timeout", func() {
				var (
					contextOps = mockutilcontext.NewMockOps(ctrl)
					mockCtx1   = mockcontext.NewMockContext(ctrl)
					mockCtx2   = mockcontext.NewMockContext(ctrl)
					cancelFunc = mockcontext.NewMockCancelFunc(ctrl)
					interval   = 2 * time.Second
				)

				contextOps.EXPECT().WithTimeout(mockCtx1, interval).Return(mockCtx2, cancelFunc.Do)

				ctx, _ := NewIntervalFactory(contextOps).New(interval)(mockCtx1)
				Expect(ctx).To(BeIdenticalTo(mockCtx2))
			})

			It("should trigger the correct cancel function", func() {
				var (
					contextOps = mockutilcontext.NewMockOps(ctrl)
					mockCtx1   = mockcontext.NewMockContext(ctrl)
					mockCtx2   = mockcontext.NewMockContext(ctrl)
					cancelFunc = mockcontext.NewMockCancelFunc(ctrl)
					interval   = 2 * time.Second
				)

				gomock.InOrder(
					contextOps.EXPECT().WithTimeout(mockCtx1, interval).Return(mockCtx2, cancelFunc.Do),
					cancelFunc.EXPECT().Do(),
				)

				ctx, cancel := NewIntervalFactory(contextOps).New(interval)(mockCtx1)
				Expect(ctx).To(BeIdenticalTo(mockCtx2))
				cancel()
			})
		})
	})

	Describe("#SevereError", func() {
		It("should return done=true and the error", func() {
			severeErr := fmt.Errorf("severe error")

			done, err := SevereError(severeErr)
			Expect(done).To(BeTrue())
			Expect(err).To(BeIdenticalTo(severeErr))
		})
	})

	Describe("#MinorError", func() {
		It("should return done=false and the error", func() {
			minorErr := fmt.Errorf("minor error")

			done, err := MinorError(minorErr)
			Expect(done).To(BeFalse())
			Expect(err).To(BeIdenticalTo(minorErr))
		})
	})

	DescribeTable("#MinorOrSevereError",
		func(retryCountUntilSevere, threshold int, minorOrSevereErr error, matcher types.GomegaMatcher) {
			done, err := MinorOrSevereError(retryCountUntilSevere, threshold, minorOrSevereErr)

			Expect(done).To(matcher)
			Expect(err).To(BeIdenticalTo(minorOrSevereErr))
		},
		Entry("count below threshold", 0, 1, errors.New("foo"), BeFalse()),
		Entry("count exactly threshold", 1, 1, errors.New("foo"), BeFalse()),
		Entry("count above threshold", 2, 1, errors.New("foo"), BeTrue()),
	)

	Describe("#Ok", func() {
		It("should return done=true and no error", func() {
			done, err := Ok()

			Expect(done).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("#NotOk", func() {
		It("should return done=false and no error", func() {
			done, err := NotOk()

			Expect(done).To(BeFalse())
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("Ops", func() {
		Describe("#Until", func() {
			It("should create an interval factory and error aggregator", func() {
				var (
					intervalFactory        = mockretry.NewMockIntervalFactory(ctrl)
					errorAggregatorFactory = mockretry.NewMockErrorAggregatorFactory(ctrl)
					contextOps             = mockutilcontext.NewMockOps(ctrl)
					interval               = 2 * time.Second
					f                      = mockretry.NewMockFunc(ctrl)

					waitFunc = mockretry.NewMockWaitFunc(ctrl)
					agg      = mockretry.NewMockErrorAggregator(ctrl)
					ctx      = mockcontext.NewMockContext(ctrl)
				)

				gomock.InOrder(
					intervalFactory.EXPECT().New(interval).Return(waitFunc.Do),
					errorAggregatorFactory.EXPECT().New().Return(agg),

					f.EXPECT().Do(ctx).Return(Ok()),
				)

				ops := NewOps(intervalFactory, errorAggregatorFactory, contextOps)

				Expect(ops.Until(ctx, interval, f.Do)).To(Succeed())
			})
		})

		Describe("#UntilTimeout", func() {
			It("should create a context that times out after the given duration", func() {
				var (
					intervalFactory        = mockretry.NewMockIntervalFactory(ctrl)
					errorAggregatorFactory = mockretry.NewMockErrorAggregatorFactory(ctrl)
					contextOps             = mockutilcontext.NewMockOps(ctrl)
					interval               = 2 * time.Second
					timeout                = 4 * time.Second
					f                      = mockretry.NewMockFunc(ctrl)

					waitFunc   = mockretry.NewMockWaitFunc(ctrl)
					agg        = mockretry.NewMockErrorAggregator(ctrl)
					ctx1       = mockcontext.NewMockContext(ctrl)
					ctx2       = mockcontext.NewMockContext(ctrl)
					cancelFunc = mockcontext.NewMockCancelFunc(ctrl)
				)

				gomock.InOrder(
					contextOps.EXPECT().WithTimeout(ctx1, timeout).Return(ctx2, cancelFunc.Do),

					intervalFactory.EXPECT().New(interval).Return(waitFunc.Do),
					errorAggregatorFactory.EXPECT().New().Return(agg),

					f.EXPECT().Do(ctx2).Return(Ok()),

					cancelFunc.EXPECT().Do(),
				)

				ops := NewOps(intervalFactory, errorAggregatorFactory, contextOps)

				Expect(ops.UntilTimeout(ctx1, interval, timeout, f.Do)).To(Succeed())
			})
		})
	})
})

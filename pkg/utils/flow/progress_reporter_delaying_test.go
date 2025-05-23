// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package flow_test

import (
	"context"
	"sync/atomic"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/goleak"
	testclock "k8s.io/utils/clock/testing"

	. "github.com/gardener/gardener/pkg/utils/flow"
)

var _ = Describe("ProgressReporterDelaying", func() {
	It("should behave correctly", func() {
		defer goleak.VerifyNone(GinkgoT(), goleak.IgnoreCurrent())

		var (
			ctx           = context.TODO()
			fakeClock     = testclock.NewFakeClock(time.Now())
			period        = 50 * time.Second
			reportedStats atomic.Value
			reporterFn    = func(_ context.Context, stats *Stats) { reportedStats.Store(stats) }
			p             = NewDelayingProgressReporter(fakeClock, reporterFn, period)
		)

		Expect(p.Start(ctx)).To(Succeed())
		Expect(reportedStats.Load()).To(BeNil())

		stats1 := &Stats{FlowName: "1"}
		p.Report(ctx, stats1)
		Expect(reportedStats.Load()).To(Equal(stats1))

		stats2 := &Stats{FlowName: "2"}
		p.Report(ctx, stats2)
		Consistently(reportedStats.Load).Should(Equal(stats1))
		fakeClock.Step(period)
		Eventually(reportedStats.Load).Should(Equal(stats2))

		stats3 := &Stats{FlowName: "3"}
		p.Report(ctx, stats3)
		Consistently(reportedStats.Load).Should(Equal(stats2))
		fakeClock.Step(period)
		Eventually(reportedStats.Load).Should(Equal(stats3))

		stats4 := &Stats{FlowName: "4"}
		p.Report(ctx, stats4)
		stats5 := &Stats{FlowName: "5"}
		p.Report(ctx, stats5)
		Consistently(reportedStats.Load).Should(Equal(stats3))
		fakeClock.Step(period)
		Eventually(reportedStats.Load).Should(Equal(stats5))

		stats6 := &Stats{FlowName: "6"}
		p.Report(ctx, stats6)
		p.Stop()
		Expect(reportedStats.Load()).To(Equal(stats6))
	})
})

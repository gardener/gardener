// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package flow_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/gardener/gardener/pkg/utils/flow"
)

var _ = Describe("ProgressReporterImmediate", func() {
	var ctx = context.TODO()

	Describe("#Start", func() {
		It("should do nothing", func() {
			pr := NewImmediateProgressReporter(nil)
			Expect(pr.Start(ctx)).To(Succeed())
		})
	})

	Describe("#Report", func() {
		It("should call the reporter function", func() {
			var (
				resultContext context.Context
				resultStats   *Stats

				stats = &Stats{FlowName: "foo"}
				pr    = NewImmediateProgressReporter(func(ctx context.Context, stats *Stats) {
					resultContext = ctx
					resultStats = stats
				})
			)

			pr.Report(ctx, stats)

			Expect(resultContext).To(Equal(ctx))
			Expect(resultStats).To(Equal(stats))
		})
	})
})

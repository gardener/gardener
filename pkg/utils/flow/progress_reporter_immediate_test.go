// Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package flow_test

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/gardener/gardener/pkg/utils/flow"
)

var _ = Describe("ProgressReporter", func() {
	Describe("#MakeDescription", func() {
		var (
			tasks = map[TaskID]struct{}{"Foo": {}, "Bar": {}, "Baz": {}}
			stats *Stats
		)

		BeforeEach(func() {
			stats = &Stats{
				FlowName: "test",
				Running:  tasks,
				All:      tasks,
			}
		})

		It("should yield the correct description when progress is 0", func() {
			Expect(MakeDescription(stats)).To(Equal("Starting test"))
		})

		It("should yield the correct description when 0 < progress < 100", func() {
			stats.Running = map[TaskID]struct{}{"Foo": {}, "Baz": {}}
			stats.Succeeded = map[TaskID]struct{}{"Bar": {}}
			Expect(strings.Split(MakeDescription(stats), ", ")).To(ConsistOf("Foo", "Baz"))
		})

		It("should yield the correct description when progress is 100", func() {
			stats.Succeeded = tasks
			Expect(MakeDescription(stats)).To(Equal("test finished"))
		})
	})
})

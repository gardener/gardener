// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package flow_test

import (
	"github.com/gardener/gardener/pkg/utils/flow"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Graph", func() {
	Describe("#Add", func() {
		It("should fail due to a duplicate task id", func() {
			graph := flow.NewGraph("foo")

			graph.Add(flow.Task{Name: "x"})
			Expect(func() { graph.Add(flow.Task{Name: "x"}) }).To(Panic())
		})

		It("should fail due to missing dependencies", func() {
			graph := flow.NewGraph("foo")

			Expect(func() {
				graph.Add(flow.Task{Name: "x", Dependencies: flow.NewTaskIDs(flow.TaskID("y"))})
			}).To(Panic())
		})
	})
})

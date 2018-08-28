// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

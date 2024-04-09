// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

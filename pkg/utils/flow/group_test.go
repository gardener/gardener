// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package flow_test

import (
	"context"
	"slices"
	"sync"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/gardener/gardener/pkg/utils/flow"
)

// recorder captures the order in which tasks are executed by the flow engine. Because tasks in
// a compiled `Flow` may run concurrently, callers should only rely on relative ordering of tasks
// with an explicit dependency edge between them.
type recorder struct {
	lock  sync.Mutex
	names []string
}

// task returns a `flow.Task` whose `Fn` appends the given name to the recorder.
func (r *recorder) task(name string, dependencies ...flow.TaskIDer) flow.Task {
	return flow.Task{
		Name: name,
		Fn: func(_ context.Context) error {
			r.lock.Lock()
			defer r.lock.Unlock()
			r.names = append(r.names, name)
			return nil
		},
		Dependencies: flow.NewTaskIDs(dependencies...),
	}
}

func (r *recorder) executed() []string {
	r.lock.Lock()
	defer r.lock.Unlock()
	return slices.Clone(r.names)
}

var _ = Describe("TaskGroup", func() {
	Describe("#NewTaskGroup", func() {
		It("should create an empty group when no tasks are given", func() {
			group := flow.NewTaskGroup("group")

			Expect(group.TaskIDs()).To(BeEmpty())
		})

		It("should create a group containing the given tasks", func() {
			group := flow.NewTaskGroup("group", flow.Task{Name: "a"}, flow.Task{Name: "b"})

			Expect(group.TaskIDs()).To(ConsistOf(flow.TaskID("a"), flow.TaskID("b")))
		})
	})

	Describe("#Add / #AddAll", func() {
		It("should add tasks incrementally", func() {
			group := flow.NewTaskGroup("group").AddAll(flow.Task{Name: "a"}, flow.Task{Name: "b"})

			Expect(group.TaskIDs()).To(ConsistOf(flow.TaskID("a"), flow.TaskID("b")))
		})

		It("should panic on duplicate task ids within the same group", func() {
			group := flow.NewTaskGroup("group", flow.Task{Name: "a"})

			Expect(func() { group.Add(flow.Task{Name: "a"}) }).To(Panic())
		})
	})
})

var _ = Describe("Graph #AddGroup", func() {
	It("should add every task of the group to the graph", func() {
		var (
			rec   = &recorder{}
			graph = flow.NewGraph("foo")
			ids   = graph.AddGroup(flow.NewTaskGroup("group", rec.task("a"), rec.task("b")))
		)

		Expect(ids.StringList()).To(Equal([]string{"a", "b"}))
		Expect(graph.Compile().Run(context.Background(), flow.Opts{})).To(Succeed())
		Expect(rec.executed()).To(ConsistOf("a", "b"))
	})

	It("should propagate task-level dependencies to each task in the group", func() {
		var (
			rec   = &recorder{}
			graph = flow.NewGraph("foo")
		)

		pre := graph.Add(rec.task("pre"))
		graph.AddGroup(
			flow.NewTaskGroup("group", rec.task("a"), rec.task("b")).WithDependencies(pre),
		)

		Expect(graph.Compile().Run(context.Background(), flow.Opts{})).To(Succeed())
		executed := rec.executed()
		Expect(executed[0]).To(Equal("pre"))
		Expect(executed[1:]).To(ConsistOf("a", "b"))
	})

	It("should expand a group dependency into a dependency on every task in that group", func() {
		var (
			rec   = &recorder{}
			graph = flow.NewGraph("foo")
		)

		first := flow.NewTaskGroup("first", rec.task("a"), rec.task("b"))
		graph.AddGroup(first)
		graph.AddGroup(flow.NewTaskGroup("second", rec.task("c")).WithDependencies(first))

		Expect(graph.Compile().Run(context.Background(), flow.Opts{})).To(Succeed())
		executed := rec.executed()
		Expect(executed[:2]).To(ConsistOf("a", "b"))
		Expect(executed[2]).To(Equal("c"))
	})

	It("should expand a group dependency referenced by group id", func() {
		// `WithDependencies` also accepts the id of another group added to the graph. `AddGroup`
		// looks it up in the graph's registered groups and fans out to that group's task ids.
		var (
			rec   = &recorder{}
			graph = flow.NewGraph("foo")
		)

		graph.AddGroup(flow.NewTaskGroup("first", rec.task("a"), rec.task("b")))
		graph.AddGroup(flow.NewTaskGroup("second", rec.task("c")).WithDependencies(flow.TaskID("first")))

		Expect(graph.Compile().Run(context.Background(), flow.Opts{})).To(Succeed())
		executed := rec.executed()
		Expect(executed[:2]).To(ConsistOf("a", "b"))
		Expect(executed[2]).To(Equal("c"))
	})

	It("should preserve pre-existing task-level dependencies alongside group dependencies", func() {
		var (
			rec   = &recorder{}
			graph = flow.NewGraph("foo")
		)

		pre := graph.Add(rec.task("pre"))
		extra := graph.Add(rec.task("extra"))
		graph.AddGroup(
			flow.NewTaskGroup("group", rec.task("a", extra)).WithDependencies(pre),
		)

		Expect(graph.Compile().Run(context.Background(), flow.Opts{})).To(Succeed())
		executed := rec.executed()
		Expect(executed).To(ContainElements("pre", "extra", "a"))
		Expect(slices.Index(executed, "a")).To(BeNumerically(">", slices.Index(executed, "pre")))
		Expect(slices.Index(executed, "a")).To(BeNumerically(">", slices.Index(executed, "extra")))
	})

	It("should panic when a task id already exists in the graph", func() {
		graph := flow.NewGraph("foo")

		graph.Add(flow.Task{Name: "a"})
		Expect(func() {
			graph.AddGroup(flow.NewTaskGroup("group", flow.Task{Name: "a"}))
		}).To(Panic())
	})

	It("should respect intra-group task-level dependencies regardless of map iteration order", func() {
		// `group.tasks` is a Go map; iterating it in insertion order is not guaranteed. Adding a task
		// before its intra-group dependency has been registered would panic at `g.Add`. `AddGroup`
		// must therefore add tasks in a dependency-respecting order. Run the scenario many times to
		// beat the map-iteration randomness.
		for range 50 {
			var (
				rec   = &recorder{}
				graph = flow.NewGraph("foo")
			)

			first := rec.task("first")
			second := rec.task("second", flow.TaskID("first"))
			third := rec.task("third", flow.TaskID("second"))

			graph.AddGroup(flow.NewTaskGroup("group", third, first, second))
			Expect(graph.Compile().Run(context.Background(), flow.Opts{})).To(Succeed())
			Expect(rec.executed()).To(Equal([]string{"first", "second", "third"}))
		}
	})
})

// SPDX-FileCopyrightText: Contributors to the Gardener project
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

	Describe("#WithID", func() {
		It("should override the group id used for later `WithDependencies` lookups", func() {
			var (
				rec   = &recorder{}
				graph = flow.NewGraph("foo")
			)

			graph.AddGroup(flow.NewTaskGroup("original", rec.task("a")).WithID("aliased"))
			graph.AddGroup(flow.NewTaskGroup("dependent", rec.task("b")).WithDependencies(flow.TaskID("aliased")))

			Expect(graph.Compile().Run(context.Background(), flow.Opts{})).To(Succeed())
			Expect(rec.executed()).To(Equal([]string{"a", "b"}))
		})

		It("should namespace contained task names so the same group can be added multiple times", func() {
			var (
				rec   = &recorder{}
				graph = flow.NewGraph("foo")
				group = flow.NewTaskGroup("g", rec.task("first"), rec.task("second", flow.TaskID("first")))
			)

			// Adding the same group twice would normally panic on duplicate task ids. `WithID` on the second
			// registration namespaces the task names so the graph stays valid.
			graph.AddGroup(group)
			graph.AddGroup(group.WithID("alias").WithDependencies(flow.TaskID("g")))

			Expect(graph.Compile().Run(context.Background(), flow.Opts{})).To(Succeed())
			// The recorder captures the original task names (twice, once per registration).
			Expect(rec.executed()).To(Equal([]string{"first", "second", "first", "second"}))
		})
	})

	Describe("#WithDependenciesIf", func() {
		It("should not add dependencies when the condition is false", func() {
			var (
				rec   = &recorder{}
				graph = flow.NewGraph("foo")
			)

			pre := graph.Add(rec.task("pre"))
			graph.AddGroup(flow.NewTaskGroup("group", rec.task("a"), rec.task("b")).WithDependenciesIf(false, pre))

			Expect(graph.Compile().Run(context.Background(), flow.Opts{})).To(Succeed())
			Expect(rec.executed()).To(ConsistOf("pre", "a", "b"))
		})

		It("should add dependencies when the condition is true", func() {
			var (
				rec   = &recorder{}
				graph = flow.NewGraph("foo")
			)

			pre := graph.Add(rec.task("pre"))
			graph.AddGroup(flow.NewTaskGroup("group", rec.task("a"), rec.task("b")).WithDependenciesIf(true, pre))

			Expect(graph.Compile().Run(context.Background(), flow.Opts{})).To(Succeed())
			executed := rec.executed()
			Expect(executed[0]).To(Equal("pre"))
			Expect(executed[1:]).To(ConsistOf("a", "b"))
		})
	})

	Describe("#Tasks", func() {
		It("should return an empty slice for an empty group", func() {
			group := flow.NewTaskGroup("group")
			Expect(group.Tasks()).To(BeEmpty())
		})

		It("should return tasks in insertion order", func() {
			group := flow.NewTaskGroup("group", flow.Task{Name: "a"}, flow.Task{Name: "b"}, flow.Task{Name: "c"})
			tasks := group.Tasks()
			Expect(tasks).To(HaveLen(3))
			Expect(tasks[0].Name).To(Equal("a"))
			Expect(tasks[1].Name).To(Equal("b"))
			Expect(tasks[2].Name).To(Equal("c"))
		})

		It("should return tasks with their original dependencies intact", func() {
			dep := flow.TaskID("dep")
			group := flow.NewTaskGroup("group", flow.Task{Name: "a", Dependencies: flow.NewTaskIDs(dep)})
			Expect(group.Tasks()[0].Dependencies.StringList()).To(ConsistOf("dep"))
		})
	})

	Describe("#WithTaskDependencies", func() {
		It("should return tasks in insertion order and add no extra dependencies when called with no args", func() {
			group := flow.NewTaskGroup("group", flow.Task{Name: "a"}, flow.Task{Name: "b"})
			tasks := group.WithTaskDependencies()
			Expect(tasks).To(HaveLen(2))
			Expect(tasks[0].Name).To(Equal("a"))
			Expect(tasks[1].Name).To(Equal("b"))
			Expect(tasks[0].Dependencies).To(BeEmpty())
			Expect(tasks[1].Dependencies).To(BeEmpty())
		})

		It("should insert the given dependency into every task", func() {
			gate := flow.TaskID("gate")
			group := flow.NewTaskGroup("group", flow.Task{Name: "a"}, flow.Task{Name: "b"})
			tasks := group.WithTaskDependencies(gate)
			Expect(tasks).To(HaveLen(2))
			Expect(tasks[0].Name).To(Equal("a"))
			Expect(tasks[0].Dependencies.StringList()).To(ConsistOf("gate"))
			Expect(tasks[1].Name).To(Equal("b"))
			Expect(tasks[1].Dependencies.StringList()).To(ConsistOf("gate"))
		})

		It("should insert multiple dependencies into every task", func() {
			dep1, dep2 := flow.TaskID("dep1"), flow.TaskID("dep2")
			group := flow.NewTaskGroup("group", flow.Task{Name: "a"})
			tasks := group.WithTaskDependencies(dep1, dep2)
			Expect(tasks[0].Dependencies.StringList()).To(ConsistOf("dep1", "dep2"))
		})

		It("should preserve pre-existing task-level dependencies alongside the new ones", func() {
			pre := flow.TaskID("pre")
			gate := flow.TaskID("gate")
			group := flow.NewTaskGroup("group", flow.Task{Name: "a", Dependencies: flow.NewTaskIDs(pre)})
			tasks := group.WithTaskDependencies(gate)
			Expect(tasks[0].Dependencies.StringList()).To(ConsistOf("pre", "gate"))
		})

		It("should not mutate the original group's task dependencies", func() {
			gate := flow.TaskID("gate")
			pre := flow.TaskID("pre")
			group := flow.NewTaskGroup("group", flow.Task{Name: "a", Dependencies: flow.NewTaskIDs(pre)})
			group.WithTaskDependencies(gate)
			Expect(group.Tasks()[0].Dependencies.StringList()).To(ConsistOf("pre"))
		})
	})

	Describe("#SkipIf", func() {
		It("should not skip tasks when the condition is false", func() {
			var (
				rec   = &recorder{}
				graph = flow.NewGraph("foo")
			)

			graph.AddGroup(flow.NewTaskGroup("group", rec.task("a"), rec.task("b")).SkipIf(false))

			Expect(graph.Compile().Run(context.Background(), flow.Opts{})).To(Succeed())
			Expect(rec.executed()).To(ConsistOf("a", "b"))
		})

		It("should skip every task in the group when the condition is true", func() {
			var (
				rec   = &recorder{}
				graph = flow.NewGraph("foo")
			)

			graph.AddGroup(flow.NewTaskGroup("group", rec.task("a"), rec.task("b")).SkipIf(true))

			Expect(graph.Compile().Run(context.Background(), flow.Opts{})).To(Succeed())
			Expect(rec.executed()).To(BeEmpty())
		})

		It("should OR multiple SkipIf calls", func() {
			var (
				rec   = &recorder{}
				graph = flow.NewGraph("foo")
			)

			graph.AddGroup(
				flow.NewTaskGroup("group", rec.task("a")).SkipIf(false).SkipIf(true),
			)

			Expect(graph.Compile().Run(context.Background(), flow.Opts{})).To(Succeed())
			Expect(rec.executed()).To(BeEmpty())
		})

		It("should preserve a task's own SkipIf when the group's SkipIf is false", func() {
			var (
				rec      = &recorder{}
				graph    = flow.NewGraph("foo")
				skipped  = rec.task("skipped")
				executed = rec.task("executed")
			)
			skipped.SkipIf = true

			graph.AddGroup(flow.NewTaskGroup("group", skipped, executed).SkipIf(false))

			Expect(graph.Compile().Run(context.Background(), flow.Opts{})).To(Succeed())
			Expect(rec.executed()).To(ConsistOf("executed"))
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
		Expect(executed[:2]).To(ConsistOf("pre", "extra"))
		Expect(executed[2]).To(Equal("a"))
	})

	It("should panic when a task id already exists in the graph", func() {
		graph := flow.NewGraph("foo")

		graph.Add(flow.Task{Name: "a"})
		Expect(func() {
			graph.AddGroup(flow.NewTaskGroup("group", flow.Task{Name: "a"}))
		}).To(Panic())
	})

	It("should add tasks in the order they were supplied to the group", func() {
		var (
			rec   = &recorder{}
			graph = flow.NewGraph("foo")
		)

		first := rec.task("first")
		second := rec.task("second", flow.TaskID("first"))
		third := rec.task("third", flow.TaskID("second"))

		graph.AddGroup(flow.NewTaskGroup("group", first, second, third))
		Expect(graph.Compile().Run(context.Background(), flow.Opts{})).To(Succeed())
		Expect(rec.executed()).To(Equal([]string{"first", "second", "third"}))
	})

	It("should panic when an intra-group dependency is supplied after its dependent", func() {
		graph := flow.NewGraph("foo")

		first := flow.Task{Name: "first"}
		second := flow.Task{Name: "second", Dependencies: flow.NewTaskIDs(flow.TaskID("first"))}

		Expect(func() { graph.AddGroup(flow.NewTaskGroup("group", second, first)) }).To(Panic())
	})
})

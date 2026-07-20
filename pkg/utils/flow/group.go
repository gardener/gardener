// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package flow

import (
	"fmt"
)

// TaskGroup is a logical bundle of tasks that share a common set of dependencies.
//
// A group has its own `id` which acts as an alias: other tasks or groups can depend on the group
// (via `WithDependencies`), and that dependency is expanded to a dependency on every task in the
// referenced group when the group is added to a `Graph` via `Graph.AddGroup`.
//
// A `TaskGroup` value is safe to build up incrementally via the fluent `AddAll`/`WithDependencies`
// methods before being added to a `Graph`.
type TaskGroup struct {
	id    TaskID
	tasks map[TaskID]Task

	// dependencies holds the `TaskID`s of tasks or groups this group depends on.
	// When the group is added to a `Graph`, every task in the group inherits this set.
	dependencies TaskIDs
}

// NewTaskGroup returns a new `TaskGroup` with the given id and the given initial tasks.
func NewTaskGroup(id TaskID, tasks ...Task) TaskGroup {
	return (TaskGroup{
		id:           id,
		tasks:        make(map[TaskID]Task, len(tasks)),
		dependencies: make(TaskIDs, len(tasks)),
	}).AddAll(tasks...)
}

// TaskIDs returns the `TaskID`s of all tasks contained in the group.
// It makes `TaskGroup` satisfy the `TaskIDer` interface so it can itself be used as a dependency.
func (g TaskGroup) TaskIDs() []TaskID {
	taskIDs := make(TaskIDs, len(g.tasks))
	for id := range g.tasks {
		taskIDs.Insert(id)
	}
	return taskIDs.TaskIDs()
}

// AddAll adds all given tasks to the group and returns the group for chaining.
// It panics if any of the tasks has an id that is already present in the group.
func (g TaskGroup) AddAll(tasks ...Task) TaskGroup {
	for _, task := range tasks {
		g.Add(task)
	}
	return g
}

// Add adds a single task to the group and returns its `TaskID`.
// It panics if a task with the same id is already present in the group.
func (g TaskGroup) Add(task Task) TaskID {
	id := task.ID()
	if _, ok := g.tasks[id]; ok {
		panic(fmt.Sprintf("Task with id %q already exists in group %q", id, g.id))
	}
	g.tasks[id] = task
	return id
}

// WithDependencies records the given dependencies on the group and returns the group for chaining.
// Each dependency may reference either a regular task or another group's id — the actual expansion
// (group id → all tasks in that group) happens at `Graph.AddGroup` time.
func (g TaskGroup) WithDependencies(dependencies ...TaskIDer) TaskGroup {
	for _, dependency := range dependencies {
		g.dependencies.Insert(dependency)
	}
	return g
}

// AddGroup adds all tasks of the given `TaskGroup` to the graph as regular tasks and returns
// their `TaskID`s so callers can wire them as dependencies of later `Add` calls.
//
// The group is a build-time convenience only: the compiled `Flow` has no notion of groups —
// it only sees individual tasks connected by task-level dependencies. `AddGroup` flattens the
// group by:
//   - Attaching every group-level dependency to each task in the group, alongside any
//     dependencies already declared on the task itself.
//   - Resolving a dependency that names another group (either the group value or its id) to
//     the `TaskID`s of every task in that group.
//
// Because resolution happens at `AddGroup` time, groups referenced by id via `WithDependencies`
// must be added before the groups that depend on them; otherwise the id is treated as a plain
// task id, which then panics for lack of a matching task.
//
// Like `Add`, this panics on duplicate task ids or unknown task-level dependencies.
func (g *Graph) AddGroup(group TaskGroup) TaskIDs {
	g.groups[group.id] = group

	dependencies := make(TaskIDs)
	for dependency := range group.dependencies {
		if gg, isGroup := g.groups[dependency]; isGroup {
			dependencies.Insert(gg)
		} else {
			dependencies.Insert(dependency)
		}
	}

	// Prepare the tasks by attaching the group-level dependencies.
	pending := make(map[TaskID]Task, len(group.tasks))
	for id, task := range group.tasks {
		if task.Dependencies == nil {
			task.Dependencies = make(TaskIDs, len(dependencies))
		}
		task.Dependencies.Insert(dependencies)
		pending[id] = task
	}

	// Delegate to `g.Add` in an order that respects intra-group task-level dependencies. Iterate until
	// every task has been added; on each pass, add tasks whose intra-group dependencies have already
	// been registered. Fail loudly on cycles or dangling intra-group dependencies to surface authoring
	// bugs the same way `g.Add` would for a single-task graph.
	ids := make(TaskIDs, len(group.tasks))
	for len(pending) > 0 {
		progress := false
		for id, task := range pending {
			ready := true
			for dependencyID := range task.Dependencies {
				if _, inGroup := group.tasks[dependencyID]; !inGroup {
					continue
				}
				if _, added := g.tasks[dependencyID]; !added {
					ready = false
					break
				}
			}
			if !ready {
				continue
			}
			ids.Insert(g.Add(task))
			delete(pending, id)
			progress = true
		}
		if !progress {
			// Every remaining task depends on another remaining task — either a cycle inside the group
			// or a task-level dependency that names a task outside the group and outside the graph.
			// Delegate to `g.Add` for one of them so it panics with its usual message.
			for _, task := range pending {
				g.Add(task)
			}
		}
	}

	return ids
}

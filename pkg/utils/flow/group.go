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
// A `TaskGroup` value is safe to build up incrementally via the fluent `AddAll`, `WithDependencies`
// and `SkipIf` methods before being added to a `Graph`.
//
// Tasks must be added in an order that respects intra-group task-level dependencies (dependencies
// before dependents), mirroring `Graph.Add`. `Graph.AddGroup` iterates tasks in the order they were
// added; an out-of-order dependency triggers the usual `g.Add` panic for the missing task.
type TaskGroup struct {
	id    TaskID
	tasks map[TaskID]Task
	// taskOrder preserves the insertion order of tasks so `Graph.AddGroup` can add them in a
	// dependency-respecting order without re-solving the dependency graph. The `tasks` map is
	// retained for O(1) duplicate detection and intra-group dependency lookup.
	taskOrder []TaskID
	// originalID is the id assigned in `NewTaskGroup`. `WithID` updates `id` but leaves this untouched
	// so `Graph.AddGroup` can detect an alias and namespace the contained task names — allowing the
	// same group to be added multiple times to a graph under different aliases.
	originalID TaskID

	// dependencies holds the `TaskID`s of tasks or groups this group depends on.
	// When the group is added to a `Graph`, every task in the group inherits this set.
	dependencies TaskIDs

	// skipIf, when true, causes every task in the group to be marked as skipped when the group
	// is added to a `Graph`. It is OR-ed with each task's own `Task.SkipIf`.
	skipIf bool
}

// NewTaskGroup returns a new `TaskGroup` with the given id and the given initial tasks.
func NewTaskGroup(id TaskID, tasks ...Task) TaskGroup {
	return (TaskGroup{
		id:           id,
		originalID:   id,
		tasks:        make(map[TaskID]Task, len(tasks)),
		taskOrder:    make([]TaskID, 0, len(tasks)),
		dependencies: make(TaskIDs),
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
func (g *TaskGroup) Add(task Task) TaskID {
	id := task.ID()
	if _, ok := g.tasks[id]; ok {
		panic(fmt.Sprintf("Task with id %q already exists in group %q", id, g.id))
	}
	g.tasks[id] = task
	g.taskOrder = append(g.taskOrder, id)
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

// SkipIf marks every task in the group as skipped when the given condition evaluates to true.
// Multiple calls OR the conditions together, and per-task `Task.SkipIf` values are preserved:
// a task already flagged skipped stays skipped regardless of the group condition.
func (g TaskGroup) SkipIf(skip bool) TaskGroup {
	g.skipIf = g.skipIf || skip
	return g
}

// WithID overrides the group's id and returns the group for chaining. Useful when the same task group is added under
// a different alias than the one baked in by its constructor, or when the same group is added multiple times to a
// graph — `Graph.AddGroup` namespaces contained task names with the alias so their ids remain unique in the graph.
func (g TaskGroup) WithID(id TaskID) TaskGroup {
	g.id = id
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
//   - OR-ing the group's `SkipIf` condition onto each task's own `Task.SkipIf`.
//   - Namespacing task names with the group's alias when the group was renamed via `WithID`,
//     so the same group can be added multiple times to the same graph without task-id
//     collisions. Intra-group task-level dependencies are rewritten to the namespaced ids.
//
// Tasks are added in the order they were supplied to the group. Callers must ensure intra-group
// dependencies are added before their dependents, just like when calling `Graph.Add` directly.
//
// Because resolution happens at `AddGroup` time, groups referenced by id via `WithDependencies`
// must be added before the groups that depend on them; otherwise the id is treated as a plain
// task id, which then panics for lack of a matching task.
//
// Like `Add`, this panics on duplicate task ids or unknown task-level dependencies.
func (g *Graph) AddGroup(group TaskGroup) TaskIDs {
	dependencies := make(TaskIDs, len(group.dependencies))
	for dependency := range group.dependencies {
		if gg, isGroup := g.groups[dependency]; isGroup {
			dependencies.Insert(gg)
		} else {
			dependencies.Insert(dependency)
		}
	}

	// If the group was aliased via `WithID`, prefix contained task names with the alias so the same group can be
	// added multiple times to the graph without task-id collisions. Intra-group dependencies must be rewritten to
	// point at the namespaced ids.
	aliased := group.id != group.originalID
	rename := func(id TaskID) TaskID { return id }
	if aliased {
		rename = func(id TaskID) TaskID { return TaskID(string(group.id) + ": " + string(id)) }
	}

	ids := make(TaskIDs, len(group.tasks))
	for _, id := range group.taskOrder {
		task := group.tasks[id]
		task.Name = string(rename(TaskID(task.Name)))
		if task.Dependencies == nil {
			task.Dependencies = make(TaskIDs, len(dependencies))
		}
		if aliased {
			renamed := make(TaskIDs, len(task.Dependencies))
			for depID := range task.Dependencies {
				if _, inGroup := group.tasks[depID]; inGroup {
					renamed.Insert(rename(depID))
				} else {
					renamed.Insert(depID)
				}
			}
			task.Dependencies = renamed
		}
		task.Dependencies.Insert(dependencies)
		task.SkipIf = task.SkipIf || group.skipIf
		ids.Insert(g.Add(task))
	}

	// Register the group's namespaced task ids so later `AddGroup` calls can fan out this group's id
	// via `WithDependencies` to the correct set of task ids in the graph.
	g.groups[group.id] = ids
	return ids
}

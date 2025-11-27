// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package flow

import (
	"context"
	"fmt"
	"time"

	"k8s.io/utils/clock"

	"github.com/gardener/gardener/pkg/utils/retry"
)

// Tasker allows multiple implementations of a task, that might wrap flow.Task with custom behavior.
type Tasker interface {
	// Spec returns the TaskSpec of a task.
	Spec() *TaskSpec
	GetID() TaskID
}

// Task is a unit of work. It has a name, a payload function and a set of dependencies.
// A is only started once all its dependencies have been completed successfully.
type Task struct {
	Name         string
	Fn           TaskFn
	SkipIf       bool
	Dependencies TaskIDs
}

// GetID returns the ID of the task.
func (t Task) GetID() TaskID {
	return TaskID(t.Name)
}

// Spec returns the TaskSpec of a task.
func (t Task) Spec() *TaskSpec {
	return &TaskSpec{
		t.Fn,
		t.SkipIf,
		t.Dependencies.Copy(),
	}
}

// RetryableTask is like Task, but it is retried until the timeout is reached.
//
// This is similar to a Task with a TaskFn that is wrapped with .RetryUntilTimeout(), but the task can optionally
// specify a reporter that will be called on every retry.
type RetryableTask struct {
	Name         string
	Fn           TaskFn
	SkipIf       bool
	Dependencies TaskIDs

	Reporter TaskRetryReporter
	Timeout  time.Duration
	Interval time.Duration
}

// GetID returns the ID of the task.
func (t RetryableTask) GetID() TaskID {
	return TaskID(t.Name)
}

// Spec returns the TaskSpec of a task.
func (t RetryableTask) Spec() *TaskSpec {
	return &TaskSpec{
		Fn: func(ctx context.Context) error {
			ctx, cancel := context.WithTimeout(ctx, t.Timeout)
			defer cancel()

			return retry.Until(ctx, t.Interval, func(ctx context.Context) (done bool, err error) {
				if err := t.Fn(ctx); err != nil {
					if t.Reporter != nil {
						t.Reporter.ReportRetry(ctx, t.GetID(), err)
					}
					return retry.MinorError(err)
				}
				return retry.Ok()
			})
		},
		Skip:         t.SkipIf,
		Dependencies: t.Dependencies.Copy(),
	}
}

// TaskSpec is functional body of a Task, consisting only of the payload function and
// the dependencies of the Task.
type TaskSpec struct {
	Fn           TaskFn
	Skip         bool
	Dependencies TaskIDs
}

// Tasks is a mapping from TaskID to TaskSpec.
type Tasks map[TaskID]*TaskSpec

// Graph is a builder for a Flow.
type Graph struct {
	name  string
	tasks Tasks

	// Clock is used to retrieve the current time.
	Clock clock.Clock
}

// Name returns the name of a graph.
func (g *Graph) Name() string {
	return g.name
}

// NewGraph returns a new Graph with the given name.
func NewGraph(name string) *Graph {
	return &Graph{name: name, tasks: make(Tasks), Clock: clock.RealClock{}}
}

// Add adds the given Task to the graph.
// This panics if
// - There is already a Task present with the same name
// - One of the dependencies of the Task is not present
func (g *Graph) Add(task Tasker) TaskID {
	id := task.GetID()
	if _, ok := g.tasks[id]; ok {
		panic(fmt.Sprintf("Task with id %q already exists", id))
	}

	spec := task.Spec()
	for dependencyID := range spec.Dependencies {
		if _, ok := g.tasks[dependencyID]; !ok {
			panic(fmt.Sprintf("Task %q is missing dependency %q", id, dependencyID))
		}
	}
	g.tasks[id] = task.Spec()
	return id
}

// Compile compiles the graph into an executable Flow.
func (g *Graph) Compile() *Flow {
	nodes := make(nodes, len(g.tasks))

	for taskName, taskSpec := range g.tasks {
		for dependencyID := range taskSpec.Dependencies {
			dependency := nodes.getOrCreate(dependencyID)
			dependency.addTargets(taskName)
		}

		node := nodes.getOrCreate(taskName)
		node.fn = taskSpec.Fn
		node.skip = taskSpec.Skip
		node.required = taskSpec.Dependencies.Len()
	}

	return &Flow{
		name:  g.name,
		nodes: nodes,
		clock: g.Clock,
	}
}

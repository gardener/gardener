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

package flow

import (
	"fmt"
)

// Task is a unit of work. It has a name, a payload function and a set of dependencies.
// A is only started once all its dependencies have been completed successfully.
type Task struct {
	Name         string
	Fn           TaskFn
	Dependencies TaskIDs
}

// Spec returns the TaskSpec of a task.
func (t *Task) Spec() *TaskSpec {
	return &TaskSpec{
		t.Fn,
		t.Dependencies.Copy(),
	}
}

// TaskSpec is functional body of a Task, consisting only of the payload function and
// the dependencies of the Task.
type TaskSpec struct {
	Fn           TaskFn
	Dependencies TaskIDs
}

// Tasks is a mapping from TaskID to TaskSpec.
type Tasks map[TaskID]*TaskSpec

// Graph is a builder for a Flow.
type Graph struct {
	name  string
	tasks Tasks
}

// Name returns the name of a graph.
func (g *Graph) Name() string {
	return g.name
}

// NewGraph returns a new Graph with the given name.
func NewGraph(name string) *Graph {
	return &Graph{name: name, tasks: make(Tasks)}
}

// Add adds the given Task to the graph.
// This panics if
// - There is already a Task present with the same name
// - One of the dependencies of the Task is not present
func (g *Graph) Add(task Task) TaskID {
	id := TaskID(task.Name)
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
		node.required = taskSpec.Dependencies.Len()
	}

	return &Flow{
		g.name,
		nodes,
	}
}

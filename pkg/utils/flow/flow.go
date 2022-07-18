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

// Package flow provides utilities to construct a directed acyclic computational graph
// that is then executed and monitored with maximum parallelism.
package flow

import (
	"context"
	"fmt"
	"time"

	utilerrors "github.com/gardener/gardener/pkg/utils/errors"

	"github.com/go-logr/logr"
	"github.com/hashicorp/go-multierror"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	logKeyFlow = "flow"
	logKeyTask = "task"
)

// ErrorCleaner is called when a task which errored during the previous reconciliation phase completes with success
type ErrorCleaner func(context.Context, string)

type nodes map[TaskID]*node

func (ns nodes) rootIDs() TaskIDs {
	roots := NewTaskIDs()
	for taskID, node := range ns {
		if node.required == 0 {
			roots.Insert(taskID)
		}
	}
	return roots
}

func (ns nodes) getOrCreate(id TaskID) *node {
	n, ok := ns[id]
	if !ok {
		n = &node{}
		ns[id] = n
	}
	return n
}

// Flow is a validated executable Graph.
type Flow struct {
	name  string
	nodes nodes
}

// Name retrieves the name of a flow.
func (f *Flow) Name() string {
	return f.name
}

// Len retrieves the amount of tasks in a Flow.
func (f *Flow) Len() int {
	return len(f.nodes)
}

// node is a compiled Task that contains the triggered Tasks, the
// number of triggers the node itself requires and its payload function.
type node struct {
	targetIDs TaskIDs
	required  int
	fn        TaskFn
}

func (n *node) String() string {
	return fmt.Sprintf("node{targets=%s, required=%d}", n.targetIDs.List(), n.required)
}

// addTargets adds the given TaskIDs as targets to the node.
func (n *node) addTargets(taskIDs ...TaskID) {
	if n.targetIDs == nil {
		n.targetIDs = NewTaskIDs(TaskIDSlice(taskIDs))
		return
	}
	n.targetIDs.Insert(TaskIDSlice(taskIDs))
}

// Opts are options for a Flow execution. If they are not set, they
// are left blank and don't affect the Flow.
type Opts struct {
	// Log is used to log any output during flow execution.
	Log logr.Logger
	// ProgressReporter is used to report the progress during flow execution.
	ProgressReporter ProgressReporter
	// ErrorCleaner is used to clean up a previously failed task.
	ErrorCleaner func(ctx context.Context, taskID string)
	// ErrorContext is used to store any error related context.
	ErrorContext *utilerrors.ErrorContext
}

// Run starts an execution of a Flow.
// It blocks until the Flow has finished and returns the error, if any.
func (f *Flow) Run(ctx context.Context, opts Opts) error {
	return newExecution(f, opts).run(ctx)
}

type nodeResult struct {
	TaskID TaskID
	Error  error
}

// Stats are the statistics of a Flow execution.
type Stats struct {
	FlowName  string
	All       TaskIDs
	Succeeded TaskIDs
	Failed    TaskIDs
	Running   TaskIDs
	Pending   TaskIDs
}

// ProgressPercent retrieves the progress of a Flow execution in percent.
func (s *Stats) ProgressPercent() int32 {
	progress := (100 * s.Succeeded.Len()) / s.All.Len()
	return int32(progress)
}

// Copy deeply copies a Stats object.
func (s *Stats) Copy() *Stats {
	return &Stats{
		s.FlowName,
		s.All.Copy(),
		s.Succeeded.Copy(),
		s.Failed.Copy(),
		s.Running.Copy(),
		s.Pending.Copy(),
	}
}

// InitialStats creates a new Stats object with the given set of initial TaskIDs.
// The initial TaskIDs are added to all TaskIDs as well as to the pending ones.
func InitialStats(flowName string, all TaskIDs) *Stats {
	return &Stats{
		flowName,
		all,
		NewTaskIDs(),
		NewTaskIDs(),
		NewTaskIDs(),
		all.Copy(),
	}
}

func newExecution(flow *Flow, opts Opts) *execution {
	all := NewTaskIDs()

	for name := range flow.nodes {
		all.Insert(name)
	}

	log := logf.Log.WithName("flow").WithValues(logKeyFlow, flow.name)
	if opts.Log.GetSink() != nil {
		log = opts.Log.WithValues(logKeyFlow, flow.name)
	}

	return &execution{
		flow,
		InitialStats(flow.name, all),
		nil,
		log,
		opts.ProgressReporter,
		opts.ErrorCleaner,
		opts.ErrorContext,
		make(chan *nodeResult),
		make(map[TaskID]int),
	}
}

type execution struct {
	flow *Flow

	stats      *Stats
	taskErrors []error

	log              logr.Logger
	progressReporter ProgressReporter
	errorCleaner     ErrorCleaner
	errorContext     *utilerrors.ErrorContext

	done          chan *nodeResult
	triggerCounts map[TaskID]int
}

func (e *execution) runNode(ctx context.Context, id TaskID) {
	if e.errorContext != nil {
		e.errorContext.AddErrorID(string(id))
	}
	e.stats.Pending.Delete(id)
	e.stats.Running.Insert(id)
	go func() {
		start := time.Now().UTC()

		e.log.WithValues(logKeyTask, id).V(1).Info("Started")
		err := e.flow.nodes[id].fn(ctx)
		end := time.Now().UTC()
		e.log.WithValues(logKeyTask, id).V(1).Info("Finished", "duration", end.Sub(start))

		if err != nil {
			e.log.WithValues(logKeyTask, id).Error(err, "Error")
			err = fmt.Errorf("task %q failed: %w", id, err)
		} else {
			e.log.WithValues(logKeyTask, id).Info("Succeeded")
		}

		e.done <- &nodeResult{TaskID: id, Error: err}
	}()
}

func (e *execution) updateSuccess(id TaskID) {
	e.stats.Running.Delete(id)
	e.stats.Succeeded.Insert(id)
}

func (e *execution) updateFailure(id TaskID) {
	e.stats.Running.Delete(id)
	e.stats.Failed.Insert(id)
}

func (e *execution) processTriggers(ctx context.Context, id TaskID) {
	node := e.flow.nodes[id]
	for target := range node.targetIDs {
		e.triggerCounts[target]++
		if e.triggerCounts[target] == e.flow.nodes[target].required {
			e.runNode(ctx, target)
		}
	}
}

func (e *execution) cleanErrors(ctx context.Context, taskID TaskID) {
	if e.errorCleaner != nil {
		e.errorCleaner(ctx, string(taskID))
	}
}

func (e *execution) reportProgress(ctx context.Context) {
	if e.progressReporter != nil {
		e.progressReporter.Report(ctx, e.stats.Copy())
	}
}

func (e *execution) run(ctx context.Context) error {
	defer close(e.done)

	if e.progressReporter != nil {
		if err := e.progressReporter.Start(ctx); err != nil {
			return err
		}
		defer e.progressReporter.Stop()
	}

	e.log.Info("Starting")
	e.reportProgress(ctx)

	var (
		cancelErr error
		roots     = e.flow.nodes.rootIDs()
	)
	for name := range roots {
		if cancelErr = ctx.Err(); cancelErr == nil {
			e.runNode(ctx, name)
		}
	}
	e.reportProgress(ctx)

	for e.stats.Running.Len() > 0 {
		result := <-e.done
		if result.Error != nil {
			e.taskErrors = append(e.taskErrors, utilerrors.WithID(string(result.TaskID), result.Error))
			e.updateFailure(result.TaskID)
		} else {
			e.updateSuccess(result.TaskID)
			if e.errorContext != nil && e.errorContext.HasLastErrorWithID(string(result.TaskID)) {
				e.cleanErrors(ctx, result.TaskID)
			}
			if cancelErr = ctx.Err(); cancelErr == nil {
				e.processTriggers(ctx, result.TaskID)
			}
		}
		e.reportProgress(ctx)
	}

	e.log.Info("Finished")
	return e.result(cancelErr)
}

func (e *execution) result(cancelErr error) error {
	if cancelErr != nil {
		return &flowCanceled{
			name:       e.flow.name,
			taskErrors: e.taskErrors,
			cause:      cancelErr,
		}
	}

	if len(e.taskErrors) > 0 {
		return &flowFailed{
			name:       e.flow.name,
			taskErrors: e.taskErrors,
		}
	}
	return nil
}

type flowCanceled struct {
	name       string
	taskErrors []error
	cause      error
}

type flowFailed struct {
	name       string
	taskErrors []error
}

func (f *flowCanceled) Error() string {
	if len(f.taskErrors) == 0 {
		return fmt.Sprintf("flow %q was canceled: %v", f.name, f.cause)
	}
	return fmt.Sprintf("flow %q was canceled: %v. Encountered task errors: %v",
		f.name, f.cause, f.taskErrors)
}

func (f *flowCanceled) Unwrap() error {
	return f.cause
}

func (f *flowFailed) Error() string {
	return fmt.Sprintf("flow %q encountered task errors: %v", f.name, f.taskErrors)
}

func (f *flowFailed) Unwrap() error {
	return &multierror.Error{Errors: f.taskErrors}
}

// Errors reports all wrapped Task errors of the given Flow error.
func Errors(err error) *multierror.Error {
	switch e := err.(type) {
	case *flowCanceled:
		return &multierror.Error{Errors: e.taskErrors}
	case *flowFailed:
		return &multierror.Error{Errors: e.taskErrors}
	}
	return nil
}

// Causes reports the causes of all Task errors of the given Flow error.
func Causes(err error) *multierror.Error {
	var (
		errs   = Errors(err).Errors
		causes = make([]error, 0, len(errs))
	)
	for _, err := range errs {
		causes = append(causes, utilerrors.Unwrap(err))
	}
	return &multierror.Error{Errors: causes}
}

// WasCanceled determines whether the given flow error was caused by cancellation.
func WasCanceled(err error) bool {
	_, ok := err.(*flowCanceled)
	return ok
}

// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

// Package flow provides utilities to construct a directed acyclic computational graph
// that is then executed and monitored with maximum parallelism.
package flow

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"github.com/hashicorp/go-multierror"
	"k8s.io/utils/clock"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/gardener/gardener/pkg/utils"
	errorsutils "github.com/gardener/gardener/pkg/utils/errors"
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

	clock clock.Clock
	start time.Time
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
	skip      bool
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
	ErrorContext *errorsutils.ErrorContext
}

// Run starts an execution of a Flow.
// It blocks until the Flow has finished and returns the error, if any.
func (f *Flow) Run(ctx context.Context, opts Opts) error {
	return newExecution(f, opts).run(ctx)
}

type nodeResult struct {
	TaskID  TaskID
	Error   error
	skipped bool

	delay    time.Duration
	duration time.Duration
}

// Stats are the statistics of a Flow execution.
type Stats struct {
	FlowName  string
	All       TaskIDs
	Succeeded TaskIDs
	Failed    TaskIDs
	Running   TaskIDs
	Skipped   TaskIDs
	Pending   TaskIDs
}

// ProgressPercent retrieves the progress of a Flow execution in percent.
func (s *Stats) ProgressPercent() int32 {
	progress := (100 * s.Succeeded.Len()) / s.All.Len()
	return int32(progress) // #nosec G115 -- s.All.Len() >= s.Succeeded.Len(), so progress is <= 100.
}

// Copy deeply copies a Stats object.
func (s *Stats) Copy() *Stats {
	return &Stats{
		s.FlowName,
		s.All.Copy(),
		s.Succeeded.Copy(),
		s.Failed.Copy(),
		s.Running.Copy(),
		s.Skipped.Copy(),
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
		NewTaskIDs(),
		all.Copy(),
	}
}

func newExecution(flow *Flow, opts Opts) *execution {
	all := NewTaskIDs()

	for name, task := range flow.nodes {
		if !task.skip {
			all.Insert(name)
		}
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
	errorContext     *errorsutils.ErrorContext

	done          chan *nodeResult
	triggerCounts map[TaskID]int
}

func (e *execution) runNode(ctx context.Context, id TaskID) {
	log := e.log.WithValues(logKeyTask, id)
	taskStartDelay := e.flow.clock.Now().UTC().Sub(e.flow.start.UTC())

	node := e.flow.nodes[id]
	if node.skip {
		log.V(1).Info("Skipped")
		e.stats.Skipped.Insert(id)

		go func() {
			e.done <- &nodeResult{TaskID: id, Error: nil, skipped: true, delay: taskStartDelay}
		}()

		return
	}

	if e.errorContext != nil {
		e.errorContext.AddErrorID(string(id))
	}

	e.stats.Pending.Delete(id)
	e.stats.Running.Insert(id)

	go func() {
		start := e.flow.clock.Now().UTC()
		log.V(1).Info("Started")
		err := node.fn(ctx)
		duration := e.flow.clock.Now().UTC().Sub(start)
		log.V(1).Info("Finished", "duration", duration)

		if err != nil {
			log.Error(err, "Error")
			err = fmt.Errorf("task %q failed: %w", id, err)
		} else {
			log.Info("Succeeded")
		}

		e.done <- &nodeResult{TaskID: id, Error: err, delay: taskStartDelay, duration: duration}
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
	e.flow.start = e.flow.clock.Now()
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

	for e.stats.Running.Len() > 0 || e.stats.Skipped.Len() > 0 {
		result := <-e.done
		e.reportTaskMetrics(result)
		if result.skipped {
			e.stats.Skipped.Delete(result.TaskID)
			if cancelErr = ctx.Err(); cancelErr == nil {
				e.processTriggers(ctx, result.TaskID)
			}
		} else {
			if result.Error != nil {
				e.taskErrors = append(e.taskErrors, errorsutils.WithID(string(result.TaskID), result.Error))
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
		}

		e.reportProgress(ctx)
	}

	e.log.Info("Finished")
	return e.result(cancelErr)
}

func (e *execution) result(cancelErr error) error {
	e.reportFlowMetrics()
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

func (e *execution) reportTaskMetrics(r *nodeResult) {
	if flowTaskDelaySeconds != nil {
		flowTaskDelaySeconds.
			WithLabelValues(e.flow.name, string(r.TaskID), utils.IifString(r.skipped, "true", "false")).
			Observe(r.delay.Seconds())
	}
	if flowTaskDurationSeconds != nil && !r.skipped {
		flowTaskDurationSeconds.WithLabelValues(e.flow.name, string(r.TaskID)).Observe(r.duration.Seconds())
	}
	if flowTaskResults != nil {
		flowTaskResults.WithLabelValues(e.flow.name, string(r.TaskID), utils.IifString(r.Error == nil, "success", "error")).Inc()
	}
}

func (e *execution) reportFlowMetrics() {
	if flowDurationSeconds != nil {
		flowDurationSeconds.WithLabelValues(e.flow.name).Observe(e.flow.clock.Now().UTC().Sub(e.flow.start.UTC()).Seconds())
	}
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
		causes = append(causes, errorsutils.Unwrap(err))
	}
	return &multierror.Error{Errors: causes}
}

// WasCanceled determines whether the given flow error was caused by cancellation.
func WasCanceled(err error) bool {
	_, ok := err.(*flowCanceled)
	return ok
}

// Copyright 2018 The Gardener Authors.
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
	"time"

	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	utilerrors "github.com/gardener/gardener/pkg/operation/errors"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/sirupsen/logrus"
)

// New creates a new Flow object.
func New(name string) *Flow {
	return &Flow{
		Name:           name,
		DoneCh:         make(chan *Task),
		ActiveTasks:    TaskList{},
		RootTasks:      TaskList{},
		ErrornousTasks: TaskList{},
	}
}

// AddTask takes a <function> and a <retryDuration> and returns a pointer to a Task object.
func (f *Flow) AddTask(function func() error, retryDuration time.Duration, dependsOn ...*Task) *Task {
	task := &Task{
		Function:                    function,
		RetryDuration:               retryDuration,
		TriggerTasks:                TaskList{},
		NumberOfPendingDependencies: len(dependsOn),
	}
	if len(dependsOn) == 0 {
		f.RootTasks = append(f.RootTasks, task)
	}
	for _, t := range dependsOn {
		t.TriggerTasks = append(t.TriggerTasks, task)
	}
	f.NumberOfExecutableTasks++
	return task
}

// AddTaskConditional takes a <function> and a <retryDuration> and returns a pointer to a Task object.
// In case the <condition> is false, the task will be marked as "skipped".
func (f *Flow) AddTaskConditional(function func() error, retryDuration time.Duration, condition bool, dependsOn ...*Task) *Task {
	task := f.AddTask(function, retryDuration, dependsOn...)
	if !condition {
		f.NumberOfExecutableTasks--
	}
	task.Skip = !condition
	return task
}

// AddSyncPoint takes a list of tasks and returns a dummy task which can be used by others
// as dependency. With that, a long list of dependencies must only defined once.
func (f *Flow) AddSyncPoint(dependsOn ...*Task) *Task {
	return f.AddTaskConditional(func() error { return nil }, 0, false, dependsOn...)
}

// SetProgressReporter will take a function <reporter> and store it on the Flow object. The
// function will be called whenever the state changes. It will receive the percentage of compeleted
// tasks of the Flow and the list of currently executed functions as arguments.
func (f *Flow) SetProgressReporter(reporter func(int, string)) *Flow {
	f.ProgressReporterFunc = reporter
	return f
}

// SetLogger will take a <logger> and store it on the Flow object. The logger will be used at the begin of each
// function invocation, and in case of errors.
func (f *Flow) SetLogger(logger *logrus.Entry) *Flow {
	f.Logger = logger
	return f
}

// Execute will execute all tasks in the flow.
func (f *Flow) Execute() *gardenv1beta1.LastError {
	f.infof("Starting flow %s", f.Name)
	for _, task := range f.RootTasks {
		f.startTask(task)
	}
	f.handleFlow()

	err := f.aggregateErrors()
	if err != nil {
		return err
	}

	f.infof("Completed flow %s successfully", f.Name)
	return nil
}

func (f *Flow) handleFlow() {
	for len(f.ActiveTasks) > 0 {
		t := <-f.DoneCh
		if !t.Skip {
			f.NumberOfCompletedTasks++
		}
		f.removeFromActiveTasks(t)
		if t.Error != nil {
			f.errorf("An error occurred while executing %s: %s", t, t.Error.Description)
			f.ErrornousTasks = append(f.ErrornousTasks, t)
		} else {
			f.triggerDependencies(t)
		}
		if f.ProgressReporterFunc != nil {
			f.ProgressReporterFunc(100*f.NumberOfCompletedTasks/(f.NumberOfExecutableTasks), f.ActiveTasks.String())
		}
	}
	close(f.DoneCh)
}

func (f *Flow) startTask(task *Task) {
	f.ActiveTasks = append(f.ActiveTasks, task)
	go func() {
		if !task.Skip {
			f.infof("Executing %s", task)
			err := utils.Retry(f.Logger, task.RetryDuration, utils.RetryFunc(task.Function))
			if err != nil {
				task.Error = utilerrors.New(err)
			}
		} else {
			f.infof("Skipped %s", task)
		}
		f.DoneCh <- task
	}()
}

func (f *Flow) triggerDependencies(task *Task) {
	for _, t := range task.TriggerTasks {
		t.NumberOfPendingDependencies--
		if t.NumberOfPendingDependencies == 0 {
			f.startTask(t)
		}
	}
}

func (f *Flow) removeFromActiveTasks(task *Task) {
	for i, t := range f.ActiveTasks {
		if task == t {
			f.ActiveTasks = append(f.ActiveTasks[:i], f.ActiveTasks[i+1:]...)
			break
		}
	}
}

func (f *Flow) aggregateErrors() *gardenv1beta1.LastError {
	if len(f.ErrornousTasks) == 0 {
		return nil
	}

	var (
		lastError = &gardenv1beta1.LastError{
			Codes: []gardenv1beta1.ErrorCode{},
		}
		e   = "Errors occurred during flow execution: "
		sep = ""
	)

	for _, t := range f.ErrornousTasks {
		if t.Error.Code != nil {
			lastError.Codes = append(lastError.Codes, *(t.Error.Code))
		}
		e += sep + fmt.Sprintf("'%s' returned '%s'", t, t.Error.Description)
		sep = ", "
	}

	lastError.Description = e

	return lastError
}

func (f *Flow) infof(format string, args ...interface{}) {
	if f.Logger != nil {
		f.Logger.Infof(format, args...)
	}
}

func (f *Flow) errorf(format string, args ...interface{}) {
	if f.Logger != nil {
		f.Logger.Errorf(format, args...)
	}
}

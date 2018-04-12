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
	"time"

	utilerrors "github.com/gardener/gardener/pkg/operation/errors"
	"github.com/sirupsen/logrus"
)

// TaskList is a list of tasks.
type TaskList []*Task

// Flow is the definition of a flow.
type Flow struct {
	Name                    string
	Logger                  *logrus.Entry
	ProgressReporterFunc    func(int, string)
	DoneCh                  chan *Task
	RootTasks               TaskList
	ActiveTasks             TaskList
	ErrornousTasks          TaskList
	NumberOfExecutableTasks int
	NumberOfCompletedTasks  int
}

// Task is the definition of a task in the flow.
type Task struct {
	Function                    func() error
	RetryDuration               time.Duration
	Error                       *utilerrors.Error
	Skip                        bool
	TriggerTasks                TaskList
	NumberOfPendingDependencies int
}

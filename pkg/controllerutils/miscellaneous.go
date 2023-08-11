// Copyright 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package controllerutils

import (
	"context"
	"strings"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/utils"
)

// DefaultReconciliationTimeout is the default timeout for the context of reconciliation functions.
const DefaultReconciliationTimeout = 3 * time.Minute

const separator = ","

// GetTasks returns the list of tasks in the ShootTasks annotation.
func GetTasks(annotations map[string]string) []string {
	var tasks []string
	if val := annotations[v1beta1constants.ShootTasks]; len(val) > 0 {
		tasks = strings.Split(val, separator)
	}
	return tasks
}

// HasTask checks if the passed task is part of the ShootTasks annotation.
func HasTask(annotations map[string]string, task string) bool {
	tasks := GetTasks(annotations)
	if len(tasks) == 0 {
		return false
	}
	return utils.ValueExists(task, tasks)
}

// AddTasks adds tasks to the ShootTasks annotation of the passed map.
func AddTasks(annotations map[string]string, tasksToAdd ...string) {
	tasks := GetTasks(annotations)

	for _, taskToAdd := range tasksToAdd {
		if !utils.ValueExists(taskToAdd, tasks) {
			tasks = append(tasks, taskToAdd)
		}
	}

	setTaskAnnotations(annotations, tasks)
}

// RemoveTasks removes tasks from the ShootTasks annotation of the passed map.
func RemoveTasks(annotations map[string]string, tasksToRemove ...string) {
	tasks := GetTasks(annotations)

	for i := len(tasks) - 1; i >= 0; i-- {
		if utils.ValueExists(tasks[i], tasksToRemove) {
			tasks = append((tasks)[:i], (tasks)[i+1:]...)
		}
	}

	setTaskAnnotations(annotations, tasks)
}

// RemoveAllTasks removes the ShootTasks annotation from the passed map.
func RemoveAllTasks(annotations map[string]string) {
	delete(annotations, v1beta1constants.ShootTasks)
}

func setTaskAnnotations(annotations map[string]string, tasks []string) {
	if len(tasks) == 0 {
		RemoveAllTasks(annotations)
		return
	}

	annotations[v1beta1constants.ShootTasks] = strings.Join(tasks, separator)
}

// GetMainReconciliationContext returns a context with timeout for the controller's main client. The resulting context has a timeout equal to the timeout passed in the argument but
// not more than DefaultReconciliationTimeout.
func GetMainReconciliationContext(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	t := timeout
	if timeout > DefaultReconciliationTimeout {
		t = DefaultReconciliationTimeout
	}

	return context.WithTimeout(ctx, t)
}

// GetChildReconciliationContext returns context with timeout for the controller's secondary client. The resulting context has a timeout equal to half of the timeout
// for the controller's main client.
func GetChildReconciliationContext(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	t := timeout
	if timeout > DefaultReconciliationTimeout {
		t = DefaultReconciliationTimeout
	}

	return context.WithTimeout(ctx, t/2)
}

// GetControllerInstallationNames returns a list of the names of the controllerinstallations passed.
func GetControllerInstallationNames(controllerInstallations []gardencorev1beta1.ControllerInstallation) []string {
	var names = make([]string, 0, len(controllerInstallations))

	for _, controllerInstallation := range controllerInstallations {
		names = append(names, controllerInstallation.Name)
	}

	return names
}

// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controllerutils

import (
	"context"
	"slices"
	"strings"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
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
	return slices.Contains(tasks, task)
}

// AddTasks adds tasks to the ShootTasks annotation of the passed map.
func AddTasks(annotations map[string]string, tasksToAdd ...string) {
	tasks := GetTasks(annotations)

	for _, taskToAdd := range tasksToAdd {
		if !slices.Contains(tasks, taskToAdd) {
			tasks = append(tasks, taskToAdd)
		}
	}

	setTaskAnnotations(annotations, tasks)
}

// RemoveTasks removes tasks from the ShootTasks annotation of the passed map.
func RemoveTasks(annotations map[string]string, tasksToRemove ...string) {
	tasks := GetTasks(annotations)

	for i := len(tasks) - 1; i >= 0; i-- {
		if slices.Contains(tasksToRemove, tasks[i]) {
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

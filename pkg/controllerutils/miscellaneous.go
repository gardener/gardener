// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controllerutils

import (
	"strings"

	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils"
)

const separator = ","

// GetTasks returns the list of tasks in the ShootTasks annotation.
func GetTasks(annotations map[string]string) []string {
	var tasks []string
	if val := annotations[common.ShootTasks]; len(val) > 0 {
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

	for i := 0; i < len(tasks); i++ {
		if utils.ValueExists(tasks[i], tasksToRemove) {
			tasks = append(tasks[:i], tasks[i+1:]...)
			i--
		}
	}

	setTaskAnnotations(annotations, tasks)
}

// RemoveAllTasks removes the ShootTasks annotation from the passed map.
func RemoveAllTasks(annotations map[string]string) {
	delete(annotations, common.ShootTasks)
	// TODO: remove in a future release
	delete(annotations, common.ShootTasksDeprecated)
}

func setTaskAnnotations(annotations map[string]string, tasks []string) {
	if len(tasks) == 0 {
		RemoveAllTasks(annotations)
		return
	}

	annotations[common.ShootTasks] = strings.Join(tasks, separator)
}

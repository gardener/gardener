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

package controllerutils

import (
	"strings"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/utils"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

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

var (
	// Now is a function for returning the current time.
	Now = time.Now
	// RandomDuration is a function for returning a random duration.
	RandomDuration = utils.RandomDuration
)

// ReconcileOncePer24hDuration returns the duration until the next reconciliation should happen while respecting that
// only one reconciliation should happen per 24h. If the deletion timestamp is set or the generation has changed or the
// last operation does not indicate success or indicates that the last reconciliation happened more than 24h ago then 0
// will be returned.
func ReconcileOncePer24hDuration(objectMeta metav1.ObjectMeta, observedGeneration int64, lastOperation *gardencorev1beta1.LastOperation) time.Duration {
	if objectMeta.DeletionTimestamp != nil {
		return 0
	}

	if objectMeta.Generation != observedGeneration {
		return 0
	}

	if v1beta1helper.HasOperationAnnotation(objectMeta) {
		return 0
	}

	if lastOperation == nil ||
		lastOperation.State != gardencorev1beta1.LastOperationStateSucceeded ||
		(lastOperation.Type != gardencorev1beta1.LastOperationTypeCreate && lastOperation.Type != gardencorev1beta1.LastOperationTypeReconcile) {
		return 0
	}

	// If last reconciliation happened more than 24h ago then we want to reconcile immediately, so let's only compute
	// a delay if the last reconciliation was within the last 24h.
	if lastReconciliation := lastOperation.LastUpdateTime.Time; Now().UTC().Before(lastReconciliation.UTC().Add(24 * time.Hour)) {
		durationUntilLastReconciliationWas24hAgo := lastReconciliation.UTC().Add(24 * time.Hour).Sub(Now().UTC())
		return RandomDuration(durationUntilLastReconciliationWas24hAgo)
	}

	return 0
}

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

package controllerutils_test

import (
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	testclock "k8s.io/utils/clock/testing"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	. "github.com/gardener/gardener/pkg/controllerutils"
)

var _ = Describe("controller", func() {
	Describe("utils", func() {
		DescribeTable("#GetTasks",
			func(existingTasks map[string]string, expectedResult []string) {
				result := GetTasks(existingTasks)
				Expect(result).To(Equal(expectedResult))
			},

			Entry("absent task annotation", map[string]string{}, nil),
			Entry("empty task list", map[string]string{v1beta1constants.ShootTasks: ""}, nil),
			Entry("task list", map[string]string{v1beta1constants.ShootTasks: "some-task" + "," + v1beta1constants.ShootTaskDeployInfrastructure},
				[]string{"some-task", v1beta1constants.ShootTaskDeployInfrastructure}),
		)

		DescribeTable("#AddTasks",
			func(existingTasks map[string]string, tasks []string, expectedTasks []string) {
				AddTasks(existingTasks, tasks...)

				if expectedTasks == nil {
					Expect(existingTasks[v1beta1constants.ShootTasks]).To(BeEmpty())
				} else {
					Expect(strings.Split(existingTasks[v1beta1constants.ShootTasks], ",")).To(Equal(expectedTasks))
				}
			},

			Entry("task to absent annotation", map[string]string{},
				[]string{v1beta1constants.ShootTaskDeployInfrastructure}, []string{v1beta1constants.ShootTaskDeployInfrastructure}),
			Entry("tasks to empty list", map[string]string{},
				[]string{v1beta1constants.ShootTaskDeployInfrastructure}, []string{v1beta1constants.ShootTaskDeployInfrastructure}),
			Entry("task to empty list", map[string]string{"foo": "bar"},
				[]string{v1beta1constants.ShootTaskDeployInfrastructure}, []string{v1beta1constants.ShootTaskDeployInfrastructure}),
			Entry("no task to empty list", map[string]string{},
				[]string{}, nil),
			Entry("no task to empty list", map[string]string{"foo": "bar"},
				[]string{}, nil),
			Entry("task to empty list twice", map[string]string{},
				[]string{v1beta1constants.ShootTaskDeployInfrastructure, v1beta1constants.ShootTaskDeployInfrastructure}, []string{v1beta1constants.ShootTaskDeployInfrastructure}),
			Entry("tasks to filled list", map[string]string{v1beta1constants.ShootTasks: v1beta1constants.ShootTaskDeployInfrastructure},
				[]string{"some-task"}, []string{v1beta1constants.ShootTaskDeployInfrastructure, "some-task"}),
			Entry("tasks already in list", map[string]string{v1beta1constants.ShootTasks: v1beta1constants.ShootTaskDeployInfrastructure},
				[]string{"some-task", v1beta1constants.ShootTaskDeployInfrastructure}, []string{v1beta1constants.ShootTaskDeployInfrastructure, "some-task"}),
		)

		DescribeTable("#RemoveTasks",
			func(existingTasks map[string]string, tasks []string, expectedTasks []string) {
				RemoveTasks(existingTasks, tasks...)

				if expectedTasks == nil {
					Expect(existingTasks[v1beta1constants.ShootTasks]).To(BeEmpty())
				} else {
					Expect(strings.Split(existingTasks[v1beta1constants.ShootTasks], ",")).To(Equal(expectedTasks))
				}
			},

			Entry("task from absent annotation", map[string]string{},
				[]string{v1beta1constants.ShootTaskDeployInfrastructure}, nil),
			Entry("task from empty list", map[string]string{"foo": "bar"},
				[]string{v1beta1constants.ShootTaskDeployInfrastructure}, nil),
			Entry("no task from empty list", map[string]string{},
				[]string{}, nil),
			Entry("task from empty list", map[string]string{"foo": "bar"},
				[]string{}, nil),
			Entry("task from empty list twice", map[string]string{},
				[]string{v1beta1constants.ShootTaskDeployInfrastructure, v1beta1constants.ShootTaskDeployInfrastructure}, nil),
			Entry("non-existing tasks from filled list", map[string]string{v1beta1constants.ShootTasks: v1beta1constants.ShootTaskDeployInfrastructure},
				[]string{"some-task"}, []string{v1beta1constants.ShootTaskDeployInfrastructure}),
			Entry("existing task from filled list", map[string]string{v1beta1constants.ShootTasks: v1beta1constants.ShootTaskDeployInfrastructure + ",foo"},
				[]string{v1beta1constants.ShootTaskDeployInfrastructure}, []string{"foo"}),
			Entry("all existing tasks from filled list", map[string]string{v1beta1constants.ShootTasks: v1beta1constants.ShootTaskDeployInfrastructure + ",foo"},
				[]string{"foo", v1beta1constants.ShootTaskDeployInfrastructure}, nil),
			Entry("all occurances of a task", map[string]string{v1beta1constants.ShootTasks: v1beta1constants.ShootTaskDeployInfrastructure + "," + v1beta1constants.ShootTaskDeployInfrastructure},
				[]string{v1beta1constants.ShootTaskDeployInfrastructure}, nil),
		)

		DescribeTable("#HasTask",
			func(existingTasks map[string]string, task string, expectedResult bool) {
				result := HasTask(existingTasks, task)
				Expect(result).To(Equal(expectedResult))
			},

			Entry("absent task annotation", map[string]string{}, v1beta1constants.ShootTaskDeployInfrastructure, false),
			Entry("empty task list", map[string]string{v1beta1constants.ShootTasks: ""}, v1beta1constants.ShootTaskDeployInfrastructure, false),
			Entry("task not in list", map[string]string{v1beta1constants.ShootTasks: "some-task" + "," + "dummyTask"}, v1beta1constants.ShootTaskDeployInfrastructure, false),
			Entry("task in list", map[string]string{v1beta1constants.ShootTasks: "some-task" + "," + v1beta1constants.ShootTaskDeployInfrastructure}, "some-task", true),
		)
	})

	var deletionTimestamp = metav1.Now()
	DescribeTable("#ReconcileOncePer24hDuration",
		func(objectMeta metav1.ObjectMeta, observedGeneration int64, lastOperation *gardencorev1beta1.LastOperation, expectedDuration time.Duration) {
			fakeClock := testclock.NewFakeClock(time.Date(1, 1, 2, 1, 0, 0, 0, time.UTC))

			oldRandomDuration := RandomDuration
			defer func() { RandomDuration = oldRandomDuration }()
			RandomDuration = func(time.Duration) time.Duration { return time.Minute }

			Expect(ReconcileOncePer24hDuration(fakeClock, objectMeta, observedGeneration, lastOperation)).To(Equal(expectedDuration))
		},

		Entry("deletion timestamp set", metav1.ObjectMeta{DeletionTimestamp: &deletionTimestamp}, int64(0), nil, time.Duration(0)),
		Entry("generation not equal observed generation", metav1.ObjectMeta{Generation: int64(1)}, int64(0), nil, time.Duration(0)),
		Entry("operation annotation is set", metav1.ObjectMeta{Annotations: map[string]string{v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationReconcile}}, int64(0), nil, time.Duration(0)),
		Entry("last operation is nil", metav1.ObjectMeta{}, int64(0), nil, time.Duration(0)),
		Entry("last operation state is succeeded", metav1.ObjectMeta{}, int64(0), &gardencorev1beta1.LastOperation{State: gardencorev1beta1.LastOperationStateSucceeded}, time.Duration(0)),
		Entry("last operation type is not create or reconcile", metav1.ObjectMeta{}, int64(0), &gardencorev1beta1.LastOperation{State: gardencorev1beta1.LastOperationStateSucceeded, Type: gardencorev1beta1.LastOperationTypeRestore}, time.Duration(0)),
		Entry("last reconciliation was more than 24h ago", metav1.ObjectMeta{}, int64(0), &gardencorev1beta1.LastOperation{State: gardencorev1beta1.LastOperationStateSucceeded, Type: gardencorev1beta1.LastOperationTypeReconcile, LastUpdateTime: metav1.Time{Time: time.Date(1, 1, 1, 0, 30, 0, 0, time.UTC)}}, time.Duration(0)),
		Entry("last reconciliation was not more than 24h ago", metav1.ObjectMeta{}, int64(0), &gardencorev1beta1.LastOperation{State: gardencorev1beta1.LastOperationStateSucceeded, Type: gardencorev1beta1.LastOperationTypeReconcile, LastUpdateTime: metav1.Time{Time: time.Date(1, 1, 1, 1, 30, 0, 0, time.UTC)}}, time.Minute),
	)
})

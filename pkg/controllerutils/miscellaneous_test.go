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

package controllerutils_test

import (
	"strings"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/operation/common"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("controller", func() {
	Describe("utils", func() {
		DescribeTable("#GetTasks",
			func(existingTasks map[string]string, expectedResult []string) {
				result := GetTasks(existingTasks)
				Expect(result).To(Equal(expectedResult))
			},

			Entry("absent task annotation", map[string]string{}, nil),
			Entry("empty task list", map[string]string{common.ShootTasks: ""}, nil),
			Entry("task list", map[string]string{common.ShootTasks: "some-task" + "," + common.ShootTaskDeployInfrastructure},
				[]string{"some-task", common.ShootTaskDeployInfrastructure}),
		)

		DescribeTable("#AddTasks",
			func(existingTasks map[string]string, tasks []string, expectedTasks []string) {
				AddTasks(existingTasks, tasks...)

				if expectedTasks == nil {
					Expect(existingTasks[common.ShootTasks]).To(BeEmpty())
				} else {
					Expect(strings.Split(existingTasks[common.ShootTasks], ",")).To(Equal(expectedTasks))
				}
			},

			Entry("task to absent annotation", map[string]string{},
				[]string{common.ShootTaskDeployInfrastructure}, []string{common.ShootTaskDeployInfrastructure}),
			Entry("tasks to empty list", map[string]string{},
				[]string{common.ShootTaskDeployInfrastructure}, []string{common.ShootTaskDeployInfrastructure}),
			Entry("task to empty list", map[string]string{"foo": "bar"},
				[]string{common.ShootTaskDeployInfrastructure}, []string{common.ShootTaskDeployInfrastructure}),
			Entry("no task to empty list", map[string]string{},
				[]string{}, nil),
			Entry("no task to empty list", map[string]string{"foo": "bar"},
				[]string{}, nil),
			Entry("task to empty list twice", map[string]string{},
				[]string{common.ShootTaskDeployInfrastructure, common.ShootTaskDeployInfrastructure}, []string{common.ShootTaskDeployInfrastructure}),
			Entry("tasks to filled list", map[string]string{common.ShootTasks: common.ShootTaskDeployInfrastructure},
				[]string{"some-task"}, []string{common.ShootTaskDeployInfrastructure, "some-task"}),
			Entry("tasks already in list", map[string]string{common.ShootTasks: common.ShootTaskDeployInfrastructure},
				[]string{"some-task", common.ShootTaskDeployInfrastructure}, []string{common.ShootTaskDeployInfrastructure, "some-task"}),
		)

		DescribeTable("#RemoveTasks",
			func(existingTasks map[string]string, tasks []string, expectedTasks []string) {
				RemoveTasks(existingTasks, tasks...)

				if expectedTasks == nil {
					Expect(existingTasks[common.ShootTasks]).To(BeEmpty())
				} else {
					Expect(strings.Split(existingTasks[common.ShootTasks], ",")).To(Equal(expectedTasks))
				}
			},

			Entry("task from absent annotation", map[string]string{},
				[]string{common.ShootTaskDeployInfrastructure}, nil),
			Entry("task from empty list", map[string]string{"foo": "bar"},
				[]string{common.ShootTaskDeployInfrastructure}, nil),
			Entry("no task from empty list", map[string]string{},
				[]string{}, nil),
			Entry("task from empty list", map[string]string{"foo": "bar"},
				[]string{}, nil),
			Entry("task from empty list twice", map[string]string{},
				[]string{common.ShootTaskDeployInfrastructure, common.ShootTaskDeployInfrastructure}, nil),
			Entry("non-existing tasks from filled list", map[string]string{common.ShootTasks: common.ShootTaskDeployInfrastructure},
				[]string{"some-task"}, []string{common.ShootTaskDeployInfrastructure}),
			Entry("existing task from filled list", map[string]string{common.ShootTasks: common.ShootTaskDeployInfrastructure + ",foo"},
				[]string{common.ShootTaskDeployInfrastructure}, []string{"foo"}),
			Entry("all existing tasks from filled list", map[string]string{common.ShootTasks: common.ShootTaskDeployInfrastructure + ",foo"},
				[]string{"foo", common.ShootTaskDeployInfrastructure}, nil),
			Entry("all occurances of a task", map[string]string{common.ShootTasks: common.ShootTaskDeployInfrastructure + "," + common.ShootTaskDeployInfrastructure},
				[]string{common.ShootTaskDeployInfrastructure}, nil),
		)

		DescribeTable("#HasTask",
			func(existingTasks map[string]string, task string, expectedResult bool) {
				result := HasTask(existingTasks, task)
				Expect(result).To(Equal(expectedResult))
			},

			Entry("absent task annotation", map[string]string{}, common.ShootTaskDeployInfrastructure, false),
			Entry("empty task list", map[string]string{common.ShootTasks: ""}, common.ShootTaskDeployInfrastructure, false),
			Entry("task not in list", map[string]string{common.ShootTasks: "some-task" + "," + "dummyTask"}, common.ShootTaskDeployInfrastructure, false),
			Entry("task in list", map[string]string{common.ShootTasks: "some-task" + "," + common.ShootTaskDeployInfrastructure}, "some-task", true),
		)
	})

	var deletionTimestamp = metav1.Now()
	DescribeTable("#ReconcileOncePer24hDuration",
		func(objectMeta metav1.ObjectMeta, observedGeneration int64, lastOperation *gardencorev1beta1.LastOperation, expectedDuration time.Duration) {
			oldNow := Now
			defer func() { Now = oldNow }()
			Now = func() time.Time { return time.Date(1, 1, 2, 1, 0, 0, 0, time.UTC) }

			oldRandomDuration := RandomDuration
			defer func() { RandomDuration = oldRandomDuration }()
			RandomDuration = func(time.Duration) time.Duration { return time.Minute }

			Expect(ReconcileOncePer24hDuration(objectMeta, observedGeneration, lastOperation)).To(Equal(expectedDuration))
		},

		Entry("deletion timestamp set", metav1.ObjectMeta{DeletionTimestamp: &deletionTimestamp}, int64(0), nil, time.Duration(0)),
		Entry("generation not equal observed generation", metav1.ObjectMeta{Generation: int64(1)}, int64(0), nil, time.Duration(0)),
		Entry("last operation is nil", metav1.ObjectMeta{}, int64(0), nil, time.Duration(0)),
		Entry("last operation state is succeeded", metav1.ObjectMeta{}, int64(0), &gardencorev1beta1.LastOperation{State: gardencorev1beta1.LastOperationStateSucceeded}, time.Duration(0)),
		Entry("last operation type is not create or reconcile", metav1.ObjectMeta{}, int64(0), &gardencorev1beta1.LastOperation{State: gardencorev1beta1.LastOperationStateSucceeded, Type: gardencorev1beta1.LastOperationTypeRestore}, time.Duration(0)),
		Entry("last reconciliation was more than 24h ago", metav1.ObjectMeta{}, int64(0), &gardencorev1beta1.LastOperation{State: gardencorev1beta1.LastOperationStateSucceeded, Type: gardencorev1beta1.LastOperationTypeReconcile, LastUpdateTime: metav1.Time{Time: time.Date(1, 1, 1, 0, 30, 0, 0, time.UTC)}}, time.Duration(0)),
		Entry("last reconciliation was not more than 24h ago", metav1.ObjectMeta{}, int64(0), &gardencorev1beta1.LastOperation{State: gardencorev1beta1.LastOperationStateSucceeded, Type: gardencorev1beta1.LastOperationTypeReconcile, LastUpdateTime: metav1.Time{Time: time.Date(1, 1, 1, 1, 30, 0, 0, time.UTC)}}, time.Minute),
	)
})

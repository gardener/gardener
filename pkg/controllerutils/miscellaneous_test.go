// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controllerutils_test

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

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
			Entry("all occurrences of a task", map[string]string{v1beta1constants.ShootTasks: v1beta1constants.ShootTaskDeployInfrastructure + "," + v1beta1constants.ShootTaskDeployInfrastructure},
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
})

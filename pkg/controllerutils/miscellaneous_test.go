// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controllerutils_test

import (
	"strings"

	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/operation/common"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("controller", func() {
	Describe("utils", func() {
		DescribeTable("#GetTasks",
			func(existingTasks map[string]string, expectedResult []string) {
				result := controllerutils.GetTasks(existingTasks)
				Expect(result).To(Equal(expectedResult))
			},

			Entry("absent task annotation", map[string]string{}, nil),
			Entry("empty task list", map[string]string{common.ShootTasks: ""}, nil),
			Entry("task list", map[string]string{common.ShootTasks: "some-task" + "," + common.ShootTaskDeployInfrastructure},
				[]string{"some-task", common.ShootTaskDeployInfrastructure}),
		)

		DescribeTable("#AddTasks",
			func(existingTasks map[string]string, tasks []string, expectedTasks []string) {
				controllerutils.AddTasks(existingTasks, tasks...)

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
				controllerutils.RemoveTasks(existingTasks, tasks...)

				if expectedTasks == nil {
					Expect(existingTasks[common.ShootTasks]).To(BeEmpty())
				} else {
					Expect(strings.Split(existingTasks[common.ShootTasks], ",")).To(Equal(expectedTasks))
				}
			},

			Entry("task from absent annotation", map[string]string{},
				[]string{common.ShootTaskDeployInfrastructure}, nil),
			Entry("tasks from empty list", map[string]string{},
				[]string{common.ShootTaskDeployInfrastructure}, nil),
			Entry("task from empty list", map[string]string{"foo": "bar"},
				[]string{common.ShootTaskDeployInfrastructure}, nil),
			Entry("no task from empty list", map[string]string{},
				[]string{}, nil),
			Entry("no task from empty list", map[string]string{"foo": "bar"},
				[]string{}, nil),
			Entry("task from empty list twice", map[string]string{},
				[]string{common.ShootTaskDeployInfrastructure, common.ShootTaskDeployInfrastructure}, nil),
			Entry("non-existing tasks from filled list", map[string]string{common.ShootTasks: common.ShootTaskDeployInfrastructure},
				[]string{"some-task"}, []string{common.ShootTaskDeployInfrastructure}),
			Entry("existing task from filled list", map[string]string{common.ShootTasks: common.ShootTaskDeployInfrastructure + ",foo"},
				[]string{common.ShootTaskDeployInfrastructure}, []string{"foo"}),
			Entry("all existing tasks from filled list", map[string]string{common.ShootTasks: common.ShootTaskDeployInfrastructure + ",foo"},
				[]string{"foo", common.ShootTaskDeployInfrastructure}, nil),
		)

		DescribeTable("#HasTask",
			func(existingTasks map[string]string, task string, expectedResult bool) {
				result := controllerutils.HasTask(existingTasks, task)
				Expect(result).To(Equal(expectedResult))
			},

			Entry("absent task annotation", map[string]string{}, common.ShootTaskDeployInfrastructure, false),
			Entry("empty task list", map[string]string{common.ShootTasks: ""}, common.ShootTaskDeployInfrastructure, false),
			Entry("task not in list", map[string]string{common.ShootTasks: "some-task" + "," + "dummyTask"}, common.ShootTaskDeployInfrastructure, false),
			Entry("task in list", map[string]string{common.ShootTasks: "some-task" + "," + common.ShootTaskDeployInfrastructure}, "some-task", true),
		)
	})
})

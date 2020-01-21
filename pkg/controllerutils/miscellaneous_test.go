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

	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/operation/common"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("controller", func() {
	Describe("utils", func() {
		DescribeTable("#AddTask",
			func(existingTasks map[string]string, tasks []string, expectedTasks []string) {
				controllerutils.AddTasks(existingTasks, tasks...)
				shootTasks := existingTasks[common.ShootTasks]
				Expect(strings.Split(shootTasks, ",")).To(Equal(expectedTasks))
			},
			Entry("task to absent annotation", map[string]string{},
				[]string{common.ShootTaskDeployInfrastructure}, []string{common.ShootTaskDeployInfrastructure}),
			Entry("tasks to empty list", map[string]string{},
				[]string{common.ShootTaskDeployInfrastructure}, []string{common.ShootTaskDeployInfrastructure}),
			Entry("task to empty list", map[string]string{common.ShootTaskDeployInfrastructure: ""},
				[]string{common.ShootTaskDeployInfrastructure}, []string{common.ShootTaskDeployInfrastructure}),
			Entry("task to empty list twice", map[string]string{},
				[]string{common.ShootTaskDeployInfrastructure, common.ShootTaskDeployInfrastructure}, []string{common.ShootTaskDeployInfrastructure}),
			Entry("tasks to filled list", map[string]string{common.ShootTasks: common.ShootTaskDeployInfrastructure},
				[]string{"some-task"}, []string{common.ShootTaskDeployInfrastructure, "some-task"}),
			Entry("tasks already in list", map[string]string{common.ShootTasks: common.ShootTaskDeployInfrastructure},
				[]string{"some-task", common.ShootTaskDeployInfrastructure}, []string{common.ShootTaskDeployInfrastructure, "some-task"}),
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

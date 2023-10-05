// Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package helper

import (
	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
)

var _ = Describe("Helper Tests", func() {
	var (
		machineDeploymentName = "machine-deployment-1"
		machineClassName      = "test-machine-class"
		expectedMachineSet    = machinev1alpha1.MachineSet{Spec: machinev1alpha1.MachineSetSpec{
			Template: machinev1alpha1.MachineTemplateSpec{
				Spec: machinev1alpha1.MachineSpec{
					Class: machinev1alpha1.ClassSpec{
						Name: machineClassName,
					},
				},
			},
		}}

		ownerReferenceToMachineSet = map[string][]machinev1alpha1.MachineSet{
			machineDeploymentName: {expectedMachineSet}}
	)

	DescribeTable("#GetMachineSetWithMachineClass", func(machineDeploymentName, machineClassName string, ownerReferenceToMachineSet map[string][]machinev1alpha1.MachineSet, expected *machinev1alpha1.MachineSet) {
		result := GetMachineSetWithMachineClass(machineDeploymentName, machineClassName, ownerReferenceToMachineSet)
		Expect(result).To(Equal(expected))
	},
		Entry("should find expected machine set", machineDeploymentName, machineClassName, ownerReferenceToMachineSet, &expectedMachineSet),

		Entry("should not find machine set - unknown machine deployment name", "unknown-machine-deployment", machineClassName, ownerReferenceToMachineSet, nil),

		Entry("should not find machine set - unknown machine class", machineDeploymentName, "unknown-machine-class", ownerReferenceToMachineSet, nil),
	)

	DescribeTable("#ReportFailedMachines",
		func(status machinev1alpha1.MachineDeploymentStatus, matcher gomegatypes.GomegaMatcher) {
			err := ReportFailedMachines(status)
			Expect(err).To(matcher)
		},
		Entry("error should be nil because no failed machines in status", machinev1alpha1.MachineDeploymentStatus{}, Succeed()),
		Entry("error should have one name and one description",
			machinev1alpha1.MachineDeploymentStatus{
				FailedMachines: []*machinev1alpha1.MachineSummary{
					{
						Name:          "machine1",
						LastOperation: machinev1alpha1.LastOperation{Description: "foo"},
					},
				},
			},
			PointTo(MatchFields(IgnoreExtras, Fields{
				"Errors": ConsistOf(MatchError("\"machine1\": foo")),
			})),
		),
		Entry("error should have multiple names and one description",
			machinev1alpha1.MachineDeploymentStatus{
				FailedMachines: []*machinev1alpha1.MachineSummary{
					{
						Name:          "machine1",
						LastOperation: machinev1alpha1.LastOperation{Description: "foo"},
					},
					{
						Name:          "machine2",
						LastOperation: machinev1alpha1.LastOperation{Description: "foo"},
					},
					{
						Name:          "machine3",
						LastOperation: machinev1alpha1.LastOperation{Description: "foo"},
					},
				},
			},
			PointTo(MatchFields(IgnoreExtras, Fields{
				"Errors": ConsistOf(MatchError("\"machine1\", \"machine2\", \"machine3\": foo")),
			})),
		),
		Entry("error should have multiple names and multiple descriptions",
			machinev1alpha1.MachineDeploymentStatus{
				FailedMachines: []*machinev1alpha1.MachineSummary{
					{
						Name:          "machine1",
						LastOperation: machinev1alpha1.LastOperation{Description: "foo"},
					},
					{
						Name:          "machine2",
						LastOperation: machinev1alpha1.LastOperation{Description: "bar"},
					},
					{
						Name:          "machine3",
						LastOperation: machinev1alpha1.LastOperation{Description: "foo"},
					},
				},
			},
			PointTo(MatchFields(IgnoreExtras, Fields{
				"Errors": ConsistOf(
					MatchError("\"machine1\", \"machine3\": foo"),
					MatchError("\"machine2\": bar"),
				),
			})),
		),
	)
})

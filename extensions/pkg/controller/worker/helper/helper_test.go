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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Helper Tests", func() {
	machineSetName := "machine-set-1"
	machineDeploymentName := "machine-deployment-1"
	var (
		machineSetReference = machinev1alpha1.Machine{
			ObjectMeta: metav1.ObjectMeta{
				OwnerReferences: []metav1.OwnerReference{
					{
						Kind: MachineSetKind,
						Name: machineSetName,
					},
				},
			},
		}

		machineDeploymentReference = machinev1alpha1.Machine{
			ObjectMeta: metav1.ObjectMeta{
				OwnerReferences: []metav1.OwnerReference{
					{
						Kind: MachineDeploymentKind,
						Name: machineDeploymentName,
					},
				},
			},
		}

		machineLabelReference = machinev1alpha1.Machine{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					nameLabel: machineDeploymentName,
				},
			},
		}
	)

	DescribeTable("#BuildOwnerToMachinesMap", func(machines []machinev1alpha1.Machine, expected map[string][]machinev1alpha1.Machine) {
		result := BuildOwnerToMachinesMap(machines)
		Expect(result).To(Equal(expected))
	},
		Entry("should map using reference kind = `MachineSet`", []machinev1alpha1.Machine{machineSetReference, machineLabelReference}, map[string][]machinev1alpha1.Machine{
			machineSetName: {machineSetReference}, machineDeploymentName: {machineLabelReference},
		}),

		Entry("should map using label with key `name`", []machinev1alpha1.Machine{machineLabelReference}, map[string][]machinev1alpha1.Machine{
			machineDeploymentName: {machineLabelReference},
		}),

		Entry("should not consider machines with machine deployment reference", []machinev1alpha1.Machine{machineSetReference, machineDeploymentReference, machineLabelReference}, map[string][]machinev1alpha1.Machine{
			machineSetName: {machineSetReference}, machineDeploymentName: {machineLabelReference},
		}),
	)

	var (
		machineSetWithOwnerReference = machinev1alpha1.MachineSet{
			ObjectMeta: metav1.ObjectMeta{
				OwnerReferences: []metav1.OwnerReference{
					{
						Kind: MachineDeploymentKind,
						Name: machineDeploymentName,
					},
				},
			},
		}

		machineSetWithWrongOwnerReference = machinev1alpha1.MachineSet{
			ObjectMeta: metav1.ObjectMeta{
				OwnerReferences: []metav1.OwnerReference{
					{
						Kind: MachineSetKind,
						Name: machineDeploymentName,
					},
				},
			},
		}

		machineSetWithLabelReference = machinev1alpha1.MachineSet{
			ObjectMeta: metav1.ObjectMeta{
				Labels: map[string]string{
					nameLabel: machineDeploymentName,
				},
			},
		}
	)

	DescribeTable("#BuildOwnerToMachineSetsMap", func(machines []machinev1alpha1.MachineSet, expected map[string][]machinev1alpha1.MachineSet) {
		result := BuildOwnerToMachineSetsMap(machines)
		Expect(result).To(Equal(expected))
	},
		Entry("should map using reference kind = `MachineDeployment", []machinev1alpha1.MachineSet{machineSetWithOwnerReference}, map[string][]machinev1alpha1.MachineSet{
			machineDeploymentName: {machineSetWithOwnerReference},
		}),

		Entry("should map using label with key `name`", []machinev1alpha1.MachineSet{machineSetWithLabelReference}, map[string][]machinev1alpha1.MachineSet{
			machineDeploymentName: {machineSetWithLabelReference},
		}),

		Entry("should not consider machines with machine set reference", []machinev1alpha1.MachineSet{machineSetWithOwnerReference, machineSetWithLabelReference, machineSetWithWrongOwnerReference}, map[string][]machinev1alpha1.MachineSet{
			machineDeploymentName: {machineSetWithOwnerReference, machineSetWithLabelReference},
		}),
	)

	var (
		machineClassName   = "test-machine-class"
		expectedMachineSet = machinev1alpha1.MachineSet{Spec: machinev1alpha1.MachineSetSpec{
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

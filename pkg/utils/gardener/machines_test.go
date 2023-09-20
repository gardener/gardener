// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package gardener_test

import (
	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/gardener/gardener/pkg/utils/gardener"
)

var _ = Describe("Machines", func() {
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
					"name": machineDeploymentName,
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
					"name": machineDeploymentName,
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
})

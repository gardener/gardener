// Copyright 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package worker

import (
	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("health", func() {
	Describe("#checkMachineDeploymentsHealthy", func() {
		It("should  return true for nil", func() {
			isHealthy, err := checkMachineDeploymentsHealthy(nil)

			Expect(isHealthy).To(BeTrue())
			Expect(err).To(Succeed())
		})

		It("should  return true for an empty list", func() {
			isHealthy, err := checkMachineDeploymentsHealthy([]machinev1alpha1.MachineDeployment{})

			Expect(isHealthy).To(BeTrue())
			Expect(err).To(Succeed())
		})

		It("should  return true when all machine deployments healthy", func() {
			machineDeployments := []machinev1alpha1.MachineDeployment{
				{
					Status: machinev1alpha1.MachineDeploymentStatus{
						Conditions: []machinev1alpha1.MachineDeploymentCondition{
							{
								Type:   machinev1alpha1.MachineDeploymentAvailable,
								Status: machinev1alpha1.ConditionTrue,
							},
							{
								Type:   machinev1alpha1.MachineDeploymentProgressing,
								Status: machinev1alpha1.ConditionTrue,
							},
						},
					},
				},
			}

			isHealthy, err := checkMachineDeploymentsHealthy(machineDeployments)

			Expect(isHealthy).To(BeTrue())
			Expect(err).To(Succeed())
		})

		It("should return an error due to failed machines", func() {
			var (
				machineName        = "foo"
				machineDescription = "error"
				machineDeployments = []machinev1alpha1.MachineDeployment{
					{
						Status: machinev1alpha1.MachineDeploymentStatus{
							FailedMachines: []*machinev1alpha1.MachineSummary{
								{
									Name:          machineName,
									LastOperation: machinev1alpha1.LastOperation{Description: machineDescription},
								},
							},
						},
					},
				}
			)

			isHealthy, err := checkMachineDeploymentsHealthy(machineDeployments)

			Expect(isHealthy).To(BeFalse())
			Expect(err).ToNot(Succeed())
		})

		It("should return an error because machine deployment is not available", func() {
			machineDeployments := []machinev1alpha1.MachineDeployment{
				{
					Status: machinev1alpha1.MachineDeploymentStatus{
						Conditions: []machinev1alpha1.MachineDeploymentCondition{
							{
								Type:   machinev1alpha1.MachineDeploymentAvailable,
								Status: machinev1alpha1.ConditionFalse,
							},
						},
					},
				},
			}

			isHealthy, err := checkMachineDeploymentsHealthy(machineDeployments)

			Expect(isHealthy).To(BeFalse())
			Expect(err).ToNot(Succeed())
		})
	})
})

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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
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

	Describe("#checkNodesScalingUp", func() {
		It("should return true if number of ready nodes equal number of desired machines", func() {
			status, err := checkNodesScalingUp(nil, 1, 1)

			Expect(status).To(Equal(gardencorev1beta1.ConditionTrue))
			Expect(err).ToNot(HaveOccurred())
		})

		It("should return an error if not enough machine objects as desired were created", func() {
			status, err := checkNodesScalingUp(&machinev1alpha1.MachineList{}, 0, 1)

			Expect(status).To(Equal(gardencorev1beta1.ConditionFalse))
			Expect(err).To(HaveOccurred())
		})

		It("should return an error when detecting erroneous machines", func() {
			machineList := &machinev1alpha1.MachineList{
				Items: []machinev1alpha1.Machine{
					{
						Status: machinev1alpha1.MachineStatus{
							CurrentStatus: machinev1alpha1.CurrentStatus{Phase: machinev1alpha1.MachineUnknown},
						},
					},
				},
			}

			status, err := checkNodesScalingUp(machineList, 0, 1)

			Expect(status).To(Equal(gardencorev1beta1.ConditionFalse))
			Expect(err).To(HaveOccurred())
		})

		It("should return an error when not detecting erroneous machines", func() {
			machineList := &machinev1alpha1.MachineList{
				Items: []machinev1alpha1.Machine{
					{
						Status: machinev1alpha1.MachineStatus{
							CurrentStatus: machinev1alpha1.CurrentStatus{Phase: machinev1alpha1.MachineRunning},
						},
					},
				},
			}

			status, err := checkNodesScalingUp(machineList, 0, 1)

			Expect(status).To(Equal(gardencorev1beta1.ConditionFalse))
			Expect(err).To(HaveOccurred())
		})

		It("should return progressing when detecting a regular scale up (pending status)", func() {
			machineList := &machinev1alpha1.MachineList{
				Items: []machinev1alpha1.Machine{
					{
						Status: machinev1alpha1.MachineStatus{
							CurrentStatus: machinev1alpha1.CurrentStatus{Phase: machinev1alpha1.MachinePending},
						},
					},
				},
			}

			status, err := checkNodesScalingUp(machineList, 0, 1)

			Expect(status).To(Equal(gardencorev1beta1.ConditionProgressing))
			Expect(err).To(HaveOccurred())
		})

		It("should return progressing when detecting a regular scale up (no status)", func() {
			machineList := &machinev1alpha1.MachineList{
				Items: []machinev1alpha1.Machine{
					{},
				},
			}

			status, err := checkNodesScalingUp(machineList, 0, 1)

			Expect(status).To(Equal(gardencorev1beta1.ConditionProgressing))
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("#checkNodesScalingDown", func() {
		It("should return true if number of registered nodes equal number of desired machines", func() {
			status, err := checkNodesScalingDown(nil, nil, 1, 1)

			Expect(status).To(Equal(gardencorev1beta1.ConditionTrue))
			Expect(err).ToNot(HaveOccurred())
		})

		It("should return an error if the machine for a cordoned node is not found", func() {
			nodeList := &corev1.NodeList{
				Items: []corev1.Node{
					{Spec: corev1.NodeSpec{Unschedulable: true}},
				},
			}

			status, err := checkNodesScalingDown(&machinev1alpha1.MachineList{}, nodeList, 2, 1)

			Expect(status).To(Equal(gardencorev1beta1.ConditionFalse))
			Expect(err).To(HaveOccurred())
		})

		It("should return an error if the machine for a cordoned node is not deleted", func() {
			var (
				nodeName = "foo"

				machineList = &machinev1alpha1.MachineList{
					Items: []machinev1alpha1.Machine{
						{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"node": nodeName}}},
					},
				}
				nodeList = &corev1.NodeList{
					Items: []corev1.Node{
						{
							ObjectMeta: metav1.ObjectMeta{Name: nodeName},
							Spec:       corev1.NodeSpec{Unschedulable: true},
						},
					},
				}
			)

			status, err := checkNodesScalingDown(machineList, nodeList, 2, 1)

			Expect(status).To(Equal(gardencorev1beta1.ConditionFalse))
			Expect(err).To(HaveOccurred())
		})

		It("should return an error if there are more nodes then machines", func() {
			status, err := checkNodesScalingDown(&machinev1alpha1.MachineList{}, &corev1.NodeList{Items: []corev1.Node{{}}}, 2, 1)

			Expect(status).To(Equal(gardencorev1beta1.ConditionFalse))
			Expect(err).To(HaveOccurred())
		})

		It("should return progressing for a regular scale down", func() {
			var (
				nodeName          = "foo"
				deletionTimestamp = metav1.Now()

				machineList = &machinev1alpha1.MachineList{
					Items: []machinev1alpha1.Machine{
						{ObjectMeta: metav1.ObjectMeta{DeletionTimestamp: &deletionTimestamp, Labels: map[string]string{"node": nodeName}}},
					},
				}
				nodeList = &corev1.NodeList{
					Items: []corev1.Node{
						{
							ObjectMeta: metav1.ObjectMeta{Name: nodeName},
							Spec:       corev1.NodeSpec{Unschedulable: true},
						},
					},
				}
			)

			status, err := checkNodesScalingDown(machineList, nodeList, 2, 1)

			Expect(status).To(Equal(gardencorev1beta1.ConditionProgressing))
			Expect(err).To(HaveOccurred())
		})

		It("should ignore node not managed by MCM and return progressing for a regular scale down", func() {
			var (
				nodeName          = "foo"
				deletionTimestamp = metav1.Now()

				machineList = &machinev1alpha1.MachineList{
					Items: []machinev1alpha1.Machine{
						{ObjectMeta: metav1.ObjectMeta{DeletionTimestamp: &deletionTimestamp, Labels: map[string]string{"node": nodeName}}},
					},
				}
				nodeList = &corev1.NodeList{
					Items: []corev1.Node{
						{
							ObjectMeta: metav1.ObjectMeta{Name: nodeName},
							Spec:       corev1.NodeSpec{Unschedulable: true},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:        "bar",
								Annotations: map[string]string{AnnotationKeyNotManagedByMCM: "1"},
							},
						},
					},
				}
			)

			status, err := checkNodesScalingDown(machineList, nodeList, 2, 1)

			Expect(status).To(Equal(gardencorev1beta1.ConditionProgressing))
			Expect(err).To(HaveOccurred())
		})
	})
})

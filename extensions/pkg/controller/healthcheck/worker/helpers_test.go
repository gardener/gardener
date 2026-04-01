// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package worker

import (
	"time"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

		It("should return error for the first failed machine in the first unhealthy deployment", func() {
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
				{
					Status: machinev1alpha1.MachineDeploymentStatus{
						FailedMachines: []*machinev1alpha1.MachineSummary{
							{
								Name:          "failed-machine-1",
								LastOperation: machinev1alpha1.LastOperation{Description: "Out of memory"},
							},
						},
					},
				},
			}

			isHealthy, err := checkMachineDeploymentsHealthy(machineDeployments)

			Expect(isHealthy).To(BeFalse())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed-machine-1"))
			Expect(err.Error()).To(ContainSubstring("Out of memory"))
		})
	})

	Describe("#getDesiredMachineCount", func() {
		It("should return 0 for nil input", func() {
			Expect(getDesiredMachineCount(nil)).To(Equal(0))
		})

		It("should return 0 for an empty list", func() {
			Expect(getDesiredMachineCount([]machinev1alpha1.MachineDeployment{})).To(Equal(0))
		})

		It("should return the sum of replicas for all machine deployments", func() {
			machineDeployments := []machinev1alpha1.MachineDeployment{
				{
					Spec: machinev1alpha1.MachineDeploymentSpec{
						Replicas: 3,
					},
				},
				{
					Spec: machinev1alpha1.MachineDeploymentSpec{
						Replicas: 2,
					},
				},
			}

			Expect(getDesiredMachineCount(machineDeployments)).To(Equal(5))
		})

		It("should skip machine deployments with a deletion timestamp", func() {
			now := metav1.NewTime(time.Now())
			machineDeployments := []machinev1alpha1.MachineDeployment{
				{
					Spec: machinev1alpha1.MachineDeploymentSpec{
						Replicas: 3,
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						DeletionTimestamp: &now,
					},
					Spec: machinev1alpha1.MachineDeploymentSpec{
						Replicas: 2,
					},
				},
			}

			Expect(getDesiredMachineCount(machineDeployments)).To(Equal(3))
		})

		It("should return 0 when all machine deployments are being deleted", func() {
			now := metav1.NewTime(time.Now())
			machineDeployments := []machinev1alpha1.MachineDeployment{
				{
					ObjectMeta: metav1.ObjectMeta{
						DeletionTimestamp: &now,
					},
					Spec: machinev1alpha1.MachineDeploymentSpec{
						Replicas: 3,
					},
				},
			}

			Expect(getDesiredMachineCount(machineDeployments)).To(Equal(0))
		})

		It("should handle machine deployments with 0 replicas", func() {
			machineDeployments := []machinev1alpha1.MachineDeployment{
				{
					Spec: machinev1alpha1.MachineDeploymentSpec{
						Replicas: 0,
					},
				},
				{
					Spec: machinev1alpha1.MachineDeploymentSpec{
						Replicas: 3,
					},
				},
			}

			Expect(getDesiredMachineCount(machineDeployments)).To(Equal(3))
		})
	})
})

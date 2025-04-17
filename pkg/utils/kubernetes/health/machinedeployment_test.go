// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package health_test

import (
	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
)

var _ = Describe("MachineDeployment", func() {
	DescribeTable("#CheckMachineDeployment",
		func(machineDeployment *machinev1alpha1.MachineDeployment, matcher gomegatypes.GomegaMatcher) {
			Expect(health.CheckMachineDeployment(machineDeployment)).To(matcher)
		},

		Entry("healthy", &machinev1alpha1.MachineDeployment{
			Status: machinev1alpha1.MachineDeploymentStatus{Conditions: []machinev1alpha1.MachineDeploymentCondition{
				{
					Type:   machinev1alpha1.MachineDeploymentAvailable,
					Status: machinev1alpha1.ConditionTrue,
				},
				{
					Type:   machinev1alpha1.MachineDeploymentProgressing,
					Status: machinev1alpha1.ConditionTrue,
				},
			}},
		}, BeNil()),
		Entry("healthy for manual in-place update with UpdatedReplicas != Replicas", &machinev1alpha1.MachineDeployment{
			Spec: machinev1alpha1.MachineDeploymentSpec{
				Strategy: machinev1alpha1.MachineDeploymentStrategy{
					Type: machinev1alpha1.InPlaceUpdateMachineDeploymentStrategyType,
					InPlaceUpdate: &machinev1alpha1.InPlaceUpdateMachineDeployment{
						OrchestrationType: machinev1alpha1.OrchestrationTypeManual,
					}},
			},
			Status: machinev1alpha1.MachineDeploymentStatus{
				UpdatedReplicas: 1,
				Replicas:        2,
				Conditions: []machinev1alpha1.MachineDeploymentCondition{
					{
						Type:   machinev1alpha1.MachineDeploymentAvailable,
						Status: machinev1alpha1.ConditionTrue,
					},
					{
						Type:   machinev1alpha1.MachineDeploymentProgressing,
						Status: machinev1alpha1.ConditionTrue,
					},
				}},
		}, BeNil()),
		Entry("healthy without progressing", &machinev1alpha1.MachineDeployment{
			Status: machinev1alpha1.MachineDeploymentStatus{Conditions: []machinev1alpha1.MachineDeploymentCondition{
				{
					Type:   machinev1alpha1.MachineDeploymentAvailable,
					Status: machinev1alpha1.ConditionTrue,
				},
			}},
		}, BeNil()),
		Entry("unhealthy without available", &machinev1alpha1.MachineDeployment{}, HaveOccurred()),
		Entry("unhealthy with false available", &machinev1alpha1.MachineDeployment{
			Status: machinev1alpha1.MachineDeploymentStatus{Conditions: []machinev1alpha1.MachineDeploymentCondition{
				{
					Type:   machinev1alpha1.MachineDeploymentAvailable,
					Status: machinev1alpha1.ConditionFalse,
				},
				{
					Type:   machinev1alpha1.MachineDeploymentProgressing,
					Status: machinev1alpha1.ConditionTrue,
				},
			}},
		}, HaveOccurred()),
		Entry("unhealthy with false progressing", &machinev1alpha1.MachineDeployment{
			Status: machinev1alpha1.MachineDeploymentStatus{Conditions: []machinev1alpha1.MachineDeploymentCondition{
				{
					Type:   machinev1alpha1.MachineDeploymentAvailable,
					Status: machinev1alpha1.ConditionTrue,
				},
				{
					Type:   machinev1alpha1.MachineDeploymentProgressing,
					Status: machinev1alpha1.ConditionFalse,
				},
			}},
		}, HaveOccurred()),
		Entry("unhealthy with false progressing for manual in-place update", &machinev1alpha1.MachineDeployment{
			Spec: machinev1alpha1.MachineDeploymentSpec{
				Strategy: machinev1alpha1.MachineDeploymentStrategy{
					Type: machinev1alpha1.InPlaceUpdateMachineDeploymentStrategyType,
					InPlaceUpdate: &machinev1alpha1.InPlaceUpdateMachineDeployment{
						OrchestrationType: machinev1alpha1.OrchestrationTypeManual,
					}},
			},
			Status: machinev1alpha1.MachineDeploymentStatus{
				Conditions: []machinev1alpha1.MachineDeploymentCondition{
					{
						Type:   machinev1alpha1.MachineDeploymentAvailable,
						Status: machinev1alpha1.ConditionTrue,
					},
					{
						Type:   machinev1alpha1.MachineDeploymentProgressing,
						Status: machinev1alpha1.ConditionFalse,
					},
				}},
		}, HaveOccurred()),
		Entry("unhealthy with bad condition", &machinev1alpha1.MachineDeployment{
			Status: machinev1alpha1.MachineDeploymentStatus{Conditions: []machinev1alpha1.MachineDeploymentCondition{
				{
					Type:   machinev1alpha1.MachineDeploymentAvailable,
					Status: machinev1alpha1.ConditionTrue,
				},
				{
					Type:   machinev1alpha1.MachineDeploymentProgressing,
					Status: machinev1alpha1.ConditionFalse,
				},
				{
					Type:   machinev1alpha1.MachineDeploymentReplicaFailure,
					Status: machinev1alpha1.ConditionTrue,
				},
			}},
		}, HaveOccurred()),
		Entry("not observed at latest version", &machinev1alpha1.MachineDeployment{
			ObjectMeta: metav1.ObjectMeta{Generation: 1},
		}, HaveOccurred()),
		Entry("not enough updated replicas", &machinev1alpha1.MachineDeployment{
			Spec: machinev1alpha1.MachineDeploymentSpec{Replicas: 1},
		}, HaveOccurred()),
	)
})

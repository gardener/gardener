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

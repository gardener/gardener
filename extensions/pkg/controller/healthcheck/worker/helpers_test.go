// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"testing"

	gardenv1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestHealth(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Health Suite")
}

var _ = Describe("health", func() {
	Context("CheckMachineDeployment", func() {
		DescribeTable("machine deployments",
			func(machineDeployment *gardenv1alpha1.MachineDeployment, matcher types.GomegaMatcher) {
				err := CheckMachineDeployment(machineDeployment)
				Expect(err).To(matcher)
			},
			Entry("healthy", &gardenv1alpha1.MachineDeployment{
				Status: gardenv1alpha1.MachineDeploymentStatus{Conditions: []gardenv1alpha1.MachineDeploymentCondition{
					{
						Type:   gardenv1alpha1.MachineDeploymentAvailable,
						Status: gardenv1alpha1.ConditionTrue,
					},
					{
						Type:   gardenv1alpha1.MachineDeploymentProgressing,
						Status: gardenv1alpha1.ConditionTrue,
					},
				}},
			}, BeNil()),
			Entry("healthy without progressing", &gardenv1alpha1.MachineDeployment{
				Status: gardenv1alpha1.MachineDeploymentStatus{Conditions: []gardenv1alpha1.MachineDeploymentCondition{
					{
						Type:   gardenv1alpha1.MachineDeploymentAvailable,
						Status: gardenv1alpha1.ConditionTrue,
					},
				}},
			}, BeNil()),
			Entry("unhealthy without available", &gardenv1alpha1.MachineDeployment{}, HaveOccurred()),
			Entry("unhealthy with false available", &gardenv1alpha1.MachineDeployment{
				Status: gardenv1alpha1.MachineDeploymentStatus{Conditions: []gardenv1alpha1.MachineDeploymentCondition{
					{
						Type:   gardenv1alpha1.MachineDeploymentAvailable,
						Status: gardenv1alpha1.ConditionFalse,
					},
					{
						Type:   gardenv1alpha1.MachineDeploymentProgressing,
						Status: gardenv1alpha1.ConditionTrue,
					},
				}},
			}, HaveOccurred()),
			Entry("unhealthy with false progressing", &gardenv1alpha1.MachineDeployment{
				Status: gardenv1alpha1.MachineDeploymentStatus{Conditions: []gardenv1alpha1.MachineDeploymentCondition{
					{
						Type:   gardenv1alpha1.MachineDeploymentAvailable,
						Status: gardenv1alpha1.ConditionTrue,
					},
					{
						Type:   gardenv1alpha1.MachineDeploymentProgressing,
						Status: gardenv1alpha1.ConditionFalse,
					},
				}},
			}, HaveOccurred()),
			Entry("unhealthy with bad condition", &gardenv1alpha1.MachineDeployment{
				Status: gardenv1alpha1.MachineDeploymentStatus{Conditions: []gardenv1alpha1.MachineDeploymentCondition{
					{
						Type:   gardenv1alpha1.MachineDeploymentAvailable,
						Status: gardenv1alpha1.ConditionTrue,
					},
					{
						Type:   gardenv1alpha1.MachineDeploymentProgressing,
						Status: gardenv1alpha1.ConditionFalse,
					},
					{
						Type:   gardenv1alpha1.MachineDeploymentReplicaFailure,
						Status: gardenv1alpha1.ConditionTrue,
					},
				}},
			}, HaveOccurred()),
			Entry("not observed at latest version", &gardenv1alpha1.MachineDeployment{
				ObjectMeta: metav1.ObjectMeta{Generation: 1},
			}, HaveOccurred()),
			Entry("not enough updated replicas", &gardenv1alpha1.MachineDeployment{
				Spec: gardenv1alpha1.MachineDeploymentSpec{Replicas: 1},
			}, HaveOccurred()),
		)
	})
})

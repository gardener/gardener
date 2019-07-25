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

package health_test

import (
	"testing"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"

	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	gardenv1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func replicas(i int32) *int32 {
	return &i
}

func TestHealth(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Health Suite")
}

var _ = Describe("health", func() {
	Context("CheckDeployment", func() {
		DescribeTable("deployments",
			func(deployment *appsv1.Deployment, matcher types.GomegaMatcher) {
				err := health.CheckDeployment(deployment)
				Expect(err).To(matcher)
			},
			Entry("healthy", &appsv1.Deployment{
				Status: appsv1.DeploymentStatus{Conditions: []appsv1.DeploymentCondition{
					{
						Type:   appsv1.DeploymentAvailable,
						Status: corev1.ConditionTrue,
					},
				}},
			}, BeNil()),
			Entry("healthy with progressing", &appsv1.Deployment{
				Status: appsv1.DeploymentStatus{Conditions: []appsv1.DeploymentCondition{
					{
						Type:   appsv1.DeploymentAvailable,
						Status: corev1.ConditionTrue,
					},
					{
						Type:   appsv1.DeploymentProgressing,
						Status: corev1.ConditionTrue,
					},
				}},
			}, BeNil()),
			Entry("not observed at latest version", &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Generation: 1},
			}, HaveOccurred()),
			Entry("not available", &appsv1.Deployment{
				Status: appsv1.DeploymentStatus{Conditions: []appsv1.DeploymentCondition{
					{
						Type:   appsv1.DeploymentAvailable,
						Status: corev1.ConditionFalse,
					},
					{
						Type:   appsv1.DeploymentProgressing,
						Status: corev1.ConditionTrue,
					},
				}},
			}, HaveOccurred()),
			Entry("not progressing", &appsv1.Deployment{
				Status: appsv1.DeploymentStatus{Conditions: []appsv1.DeploymentCondition{
					{
						Type:   appsv1.DeploymentAvailable,
						Status: corev1.ConditionTrue,
					},
					{
						Type:   appsv1.DeploymentProgressing,
						Status: corev1.ConditionFalse,
					},
				}},
			}, HaveOccurred()),
			Entry("available | progressing missing", &appsv1.Deployment{}, HaveOccurred()),
		)
	})

	Context("CheckStatefulSet", func() {
		DescribeTable("statefulsets",
			func(statefulSet *appsv1.StatefulSet, matcher types.GomegaMatcher) {
				err := health.CheckStatefulSet(statefulSet)
				Expect(err).To(matcher)
			},
			Entry("healthy", &appsv1.StatefulSet{
				Spec:   appsv1.StatefulSetSpec{Replicas: replicas(1)},
				Status: appsv1.StatefulSetStatus{CurrentReplicas: 1, ReadyReplicas: 1},
			}, BeNil()),
			Entry("healthy with nil replicas", &appsv1.StatefulSet{
				Status: appsv1.StatefulSetStatus{ReadyReplicas: 1},
			}, BeNil()),
			Entry("not observed at latest version", &appsv1.StatefulSet{
				ObjectMeta: metav1.ObjectMeta{Generation: 1},
			}, HaveOccurred()),
			Entry("not enough ready replicas", &appsv1.StatefulSet{
				Spec:   appsv1.StatefulSetSpec{Replicas: replicas(2)},
				Status: appsv1.StatefulSetStatus{ReadyReplicas: 1},
			}, HaveOccurred()),
		)
	})

	Context("CheckDaemonSet", func() {
		oneUnavailable := intstr.FromInt(1)
		DescribeTable("daemonsets",
			func(daemonSet *appsv1.DaemonSet, matcher types.GomegaMatcher) {
				err := health.CheckDaemonSet(daemonSet)
				Expect(err).To(matcher)
			},
			Entry("healthy", &appsv1.DaemonSet{}, BeNil()),
			Entry("healthy with one unavailable", &appsv1.DaemonSet{
				Spec: appsv1.DaemonSetSpec{UpdateStrategy: appsv1.DaemonSetUpdateStrategy{
					Type: appsv1.RollingUpdateDaemonSetStrategyType,
					RollingUpdate: &appsv1.RollingUpdateDaemonSet{
						MaxUnavailable: &oneUnavailable,
					},
				}},
				Status: appsv1.DaemonSetStatus{
					DesiredNumberScheduled: 2,
					CurrentNumberScheduled: 1,
				},
			}, BeNil()),
			Entry("not observed at latest version", &appsv1.DaemonSet{
				ObjectMeta: metav1.ObjectMeta{Generation: 1},
			}, HaveOccurred()),
			Entry("not enough updated scheduled", &appsv1.DaemonSet{
				Status: appsv1.DaemonSetStatus{DesiredNumberScheduled: 1},
			}, HaveOccurred()),
		)
	})

	Context("CheckNode", func() {
		DescribeTable("nodes",
			func(node *corev1.Node, matcher types.GomegaMatcher) {
				err := health.CheckNode(node)
				Expect(err).To(matcher)
			},
			Entry("healthy", &corev1.Node{
				Status: corev1.NodeStatus{Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}}},
			}, BeNil()),
			Entry("no ready condition", &corev1.Node{}, HaveOccurred()),
			Entry("ready condition not indicating true", &corev1.Node{
				Status: corev1.NodeStatus{Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionFalse}}},
			}, HaveOccurred()),
		)
	})

	Context("CheckMachineDeployment", func() {
		DescribeTable("machine deployments",
			func(machineDeployment *gardenv1alpha1.MachineDeployment, matcher types.GomegaMatcher) {
				err := health.CheckMachineDeployment(machineDeployment)
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

	Context("CheckSeed", func() {
		DescribeTable("seeds",
			func(seed *gardenv1beta1.Seed, identity *gardenv1beta1.Gardener, matcher types.GomegaMatcher) {
				Expect(health.CheckSeed(seed, identity)).To(matcher)
			},
			Entry("healthy", &gardenv1beta1.Seed{
				Status: gardenv1beta1.SeedStatus{
					Conditions: []gardencorev1alpha1.Condition{
						{Type: gardenv1beta1.SeedAvailable, Status: gardencorev1alpha1.ConditionTrue},
					},
				},
			}, &gardenv1beta1.Gardener{}, Succeed()),
			Entry("healthy with non-default identity", &gardenv1beta1.Seed{
				Status: gardenv1beta1.SeedStatus{
					Gardener: gardenv1beta1.Gardener{ID: "thegardener"},
					Conditions: []gardencorev1alpha1.Condition{
						{Type: gardenv1beta1.SeedAvailable, Status: gardencorev1alpha1.ConditionTrue},
					},
				},
			}, &gardenv1beta1.Gardener{ID: "thegardener"}, Succeed()),
			Entry("unhealthy available condition", &gardenv1beta1.Seed{
				Status: gardenv1beta1.SeedStatus{
					Conditions: []gardencorev1alpha1.Condition{
						{Type: gardenv1beta1.SeedAvailable, Status: gardencorev1alpha1.ConditionFalse},
					},
				},
			}, &gardenv1beta1.Gardener{}, HaveOccurred()),
			Entry("unhealthy due to missing available condition", &gardenv1beta1.Seed{}, &gardenv1beta1.Gardener{}, HaveOccurred()),
			Entry("unhealthy due to non-matching identity", &gardenv1beta1.Seed{
				Status: gardenv1beta1.SeedStatus{
					Gardener: gardenv1beta1.Gardener{ID: "thegardener"},
					Conditions: []gardencorev1alpha1.Condition{
						{Type: gardenv1beta1.SeedAvailable, Status: gardencorev1alpha1.ConditionTrue},
					},
				},
			}, &gardenv1beta1.Gardener{}, HaveOccurred()),
			Entry("not observed at latest generation", &gardenv1beta1.Seed{
				ObjectMeta: metav1.ObjectMeta{
					Generation: 1,
				},
				Status: gardenv1beta1.SeedStatus{
					Conditions: []gardencorev1alpha1.Condition{
						{Type: gardenv1beta1.SeedAvailable, Status: gardencorev1alpha1.ConditionTrue},
					},
				},
			}, &gardenv1beta1.Gardener{}, HaveOccurred()),
		)
	})

	Context("CheckExtensionObject", func() {
		DescribeTable("extension objects",
			func(obj extensionsv1alpha1.Object, match types.GomegaMatcher) {
				Expect(health.CheckExtensionObject(obj)).To(match)
			},
			Entry("healthy",
				&extensionsv1alpha1.Infrastructure{
					Status: extensionsv1alpha1.InfrastructureStatus{
						DefaultStatus: extensionsv1alpha1.DefaultStatus{
							LastOperation: &gardencorev1alpha1.LastOperation{
								State: gardencorev1alpha1.LastOperationStateSucceeded,
							},
						},
					},
				},
				Succeed()),
			Entry("generation outdated",
				&extensionsv1alpha1.Infrastructure{
					ObjectMeta: metav1.ObjectMeta{
						Generation: 1,
					},
					Status: extensionsv1alpha1.InfrastructureStatus{
						DefaultStatus: extensionsv1alpha1.DefaultStatus{
							LastOperation: &gardencorev1alpha1.LastOperation{
								State: gardencorev1alpha1.LastOperationStateSucceeded,
							},
						},
					},
				},
				HaveOccurred()),
			Entry("gardener operation ongoing",
				&extensionsv1alpha1.Infrastructure{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							gardencorev1alpha1.GardenerOperation: gardencorev1alpha1.GardenerOperationReconcile,
						},
					},
					Status: extensionsv1alpha1.InfrastructureStatus{
						DefaultStatus: extensionsv1alpha1.DefaultStatus{
							LastOperation: &gardencorev1alpha1.LastOperation{
								State: gardencorev1alpha1.LastOperationStateSucceeded,
							},
						},
					},
				},
				HaveOccurred()),
			Entry("last error non-nil",
				&extensionsv1alpha1.Infrastructure{
					Status: extensionsv1alpha1.InfrastructureStatus{
						DefaultStatus: extensionsv1alpha1.DefaultStatus{
							LastError: &gardencorev1alpha1.LastError{
								Description: "something happened",
							},
							LastOperation: &gardencorev1alpha1.LastOperation{
								State: gardencorev1alpha1.LastOperationStateSucceeded,
							},
						},
					},
				},
				HaveOccurred()),
			Entry("no last operation",
				&extensionsv1alpha1.Infrastructure{},
				HaveOccurred()),
			Entry("last operation not succeeded",
				&extensionsv1alpha1.Infrastructure{
					Status: extensionsv1alpha1.InfrastructureStatus{
						DefaultStatus: extensionsv1alpha1.DefaultStatus{
							LastOperation: &gardencorev1alpha1.LastOperation{
								State: gardencorev1alpha1.LastOperationStateError,
							},
						},
					},
				},
				HaveOccurred()),
		)
	})
})

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

package botanist_test

import (
	"testing"
	"time"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/apis/garden/v1beta1/helper"
	"github.com/gardener/gardener/pkg/operation/botanist"
	"github.com/gardener/gardener/pkg/operation/common"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/onsi/gomega/types"
)

func TestBotanist(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Botanist Suite")
}

var (
	zeroTime     time.Time
	zeroMetaTime metav1.Time
)

func roleOf(obj metav1.Object) string {
	return obj.GetLabels()[common.GardenRole]
}

func constDeploymentLister(deployments []*appsv1.Deployment) kutil.DeploymentLister {
	return kutil.NewDeploymentLister(func() ([]*appsv1.Deployment, error) {
		return deployments, nil
	})
}

func constStatefulSetLister(statefulSets []*appsv1.StatefulSet) kutil.StatefulSetLister {
	return kutil.NewStatefulSetLister(func() ([]*appsv1.StatefulSet, error) {
		return statefulSets, nil
	})
}

func constDaemonSetLister(daemonSets []*appsv1.DaemonSet) kutil.DaemonSetLister {
	return kutil.NewDaemonSetLister(func() ([]*appsv1.DaemonSet, error) {
		return daemonSets, nil
	})
}

func constNodeLister(nodes []*corev1.Node) kutil.NodeLister {
	return kutil.NewNodeLister(func() ([]*corev1.Node, error) {
		return nodes, nil
	})
}

func constMachineDeploymentLister(machineDeployments []*machinev1alpha1.MachineDeployment) kutil.MachineDeploymentLister {
	return kutil.NewMachineDeploymentLister(func() ([]*machinev1alpha1.MachineDeployment, error) {
		return machineDeployments, nil
	})
}

func roleLabels(role string) map[string]string {
	return map[string]string{common.GardenRole: role}
}

func newDeployment(namespace, name, role string, healthy bool) *appsv1.Deployment {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
			Labels:    roleLabels(role),
		},
	}
	if healthy {
		deployment.Status = appsv1.DeploymentStatus{Conditions: []appsv1.DeploymentCondition{{
			Type:   appsv1.DeploymentAvailable,
			Status: corev1.ConditionTrue,
		}}}
	}
	return deployment
}

func newStatefulSet(namespace, name, role string, healthy bool) *appsv1.StatefulSet {
	statefulSet := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
			Labels:    roleLabels(role),
		},
	}
	if healthy {
		statefulSet.Status.ReadyReplicas = 1
	}

	return statefulSet
}

func newDaemonSet(namespace, name, role string, healthy bool) *appsv1.DaemonSet {
	daemonSet := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
			Labels:    roleLabels(role),
		},
	}
	if !healthy {
		daemonSet.Status.DesiredNumberScheduled = 1
	}

	return daemonSet
}

func newNode(name string, healthy bool) *corev1.Node {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}

	if healthy {
		node.Status.Conditions = []corev1.NodeCondition{
			{
				Type:   corev1.NodeReady,
				Status: corev1.ConditionTrue,
			},
		}
	}

	return node
}

func newMachineDeployment(namespace, name string, replicas int32, healthy bool) *machinev1alpha1.MachineDeployment {
	machineDeployment := &machinev1alpha1.MachineDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Spec: machinev1alpha1.MachineDeploymentSpec{
			Replicas: replicas,
		},
	}

	if healthy {
		machineDeployment.Status.Conditions = []machinev1alpha1.MachineDeploymentCondition{
			{
				Type:   machinev1alpha1.MachineDeploymentAvailable,
				Status: machinev1alpha1.ConditionTrue,
			},
		}
	}

	return machineDeployment
}

func beConditionWithStatus(status gardencorev1alpha1.ConditionStatus) types.GomegaMatcher {
	return PointTo(MatchFields(IgnoreExtras, Fields{
		"Status": Equal(status),
	}))
}

var _ = Describe("health check", func() {
	var (
		condition = gardencorev1alpha1.Condition{
			Type: gardencorev1alpha1.ConditionType("test"),
		}
		gcpShoot = &gardenv1beta1.Shoot{
			Spec: gardenv1beta1.ShootSpec{
				Cloud: gardenv1beta1.Cloud{
					GCP: &gardenv1beta1.GCPCloud{},
				},
			},
		}
		gcpShootWithAutoscaler = &gardenv1beta1.Shoot{
			Spec: gardenv1beta1.ShootSpec{
				Cloud: gardenv1beta1.Cloud{
					GCP: &gardenv1beta1.GCPCloud{
						Workers: []gardenv1beta1.GCPWorker{
							{Worker: gardenv1beta1.Worker{
								Name:          "foo",
								AutoScalerMin: 1,
								AutoScalerMax: 2,
							}},
						},
					},
				},
			},
		}

		seedNamespace  = "shoot--foo--bar"
		shootNamespace = metav1.NamespaceSystem

		// control plane deployments
		cloudControllerManagerDeployment   = newDeployment(seedNamespace, common.CloudControllerManagerDeploymentName, common.GardenRoleControlPlane, true)
		kubeAddonManagerDeployment         = newDeployment(seedNamespace, common.KubeAddonManagerDeploymentName, common.GardenRoleControlPlane, true)
		kubeAPIServerDeployment            = newDeployment(seedNamespace, common.KubeAPIServerDeploymentName, common.GardenRoleControlPlane, true)
		kubeControllerManagerDeployment    = newDeployment(seedNamespace, common.KubeControllerManagerDeploymentName, common.GardenRoleControlPlane, true)
		kubeSchedulerDeployment            = newDeployment(seedNamespace, common.KubeSchedulerDeploymentName, common.GardenRoleControlPlane, true)
		machineControllerManagerDeployment = newDeployment(seedNamespace, common.MachineControllerManagerDeploymentName, common.GardenRoleControlPlane, true)
		awsLBReadvertiserDeployment        = newDeployment(seedNamespace, common.AWSLBReadvertiserDeploymentName, common.GardenRoleControlPlane, true)
		clusterAutoscalerDeployment        = newDeployment(seedNamespace, gardencorev1alpha1.DeploymentNameClusterAutoscaler, common.GardenRoleControlPlane, true)

		requiredControlPlaneDeployments = []*appsv1.Deployment{
			cloudControllerManagerDeployment,
			kubeAddonManagerDeployment,
			kubeAPIServerDeployment,
			kubeControllerManagerDeployment,
			kubeSchedulerDeployment,
			machineControllerManagerDeployment,
			awsLBReadvertiserDeployment,
			clusterAutoscalerDeployment,
		}

		// control plane stateful sets
		etcdMainStatefulSet   = newStatefulSet(seedNamespace, common.ETCDMainStatefulSetName, common.GardenRoleControlPlane, true)
		etcdEventsStatefulSet = newStatefulSet(seedNamespace, common.ETCDEventsStatefulSetName, common.GardenRoleControlPlane, true)

		requiredControlPlaneStatefulSets = []*appsv1.StatefulSet{
			etcdMainStatefulSet,
			etcdEventsStatefulSet,
		}

		// system component deployments
		calicoTyphaDeployment   = newDeployment(shootNamespace, common.CalicoTyphaDeploymentName, common.GardenRoleSystemComponent, true)
		coreDNSDeployment       = newDeployment(shootNamespace, common.CoreDNSDeploymentName, common.GardenRoleSystemComponent, true)
		vpnShootDeployment      = newDeployment(shootNamespace, common.VPNShootDeploymentName, common.GardenRoleSystemComponent, true)
		metricsServerDeployment = newDeployment(shootNamespace, common.MetricsServerDeploymentName, common.GardenRoleSystemComponent, true)

		requiredSystemComponentDeployments = []*appsv1.Deployment{
			calicoTyphaDeployment,
			coreDNSDeployment,
			vpnShootDeployment,
			metricsServerDeployment,
		}

		// system component daemon sets
		calicoNodeDaemonSet = newDaemonSet(shootNamespace, common.CalicoNodeDaemonSetName, common.GardenRoleSystemComponent, true)
		kubeProxyDaemonSet  = newDaemonSet(shootNamespace, common.KubeProxyDaemonSetName, common.GardenRoleSystemComponent, true)

		requiredSystemComponentDaemonSets = []*appsv1.DaemonSet{
			calicoNodeDaemonSet,
			kubeProxyDaemonSet,
		}

		nodeExporterDaemonSet = newDaemonSet(shootNamespace, common.NodeExporterDaemonSetName, common.GardenRoleMonitoring, true)

		requiredMonitoringSystemComponentDaemonSets = []*appsv1.DaemonSet{
			nodeExporterDaemonSet,
		}

		grafanaDeployment               = newDeployment(seedNamespace, common.GrafanaDeploymentName, common.GardenRoleMonitoring, true)
		kubeStateMetricsSeedDeployment  = newDeployment(seedNamespace, common.KubeStateMetricsSeedDeploymentName, common.GardenRoleMonitoring, true)
		kubeStateMetricsShootDeployment = newDeployment(seedNamespace, common.KubeStateMetricsShootDeploymentName, common.GardenRoleMonitoring, true)

		requiredMonitoringControlPlaneDeployments = []*appsv1.Deployment{
			grafanaDeployment,
			kubeStateMetricsSeedDeployment,
			kubeStateMetricsShootDeployment,
		}

		alertManagerStatefulSet = newStatefulSet(seedNamespace, common.AlertManagerStatefulSetName, common.GardenRoleMonitoring, true)
		prometheusStatefulSet   = newStatefulSet(seedNamespace, common.PrometheusStatefulSetName, common.GardenRoleMonitoring, true)

		requiredMonitoringControlPlaneStatefulSets = []*appsv1.StatefulSet{
			alertManagerStatefulSet,
			prometheusStatefulSet,
		}

		kibanaDeployment = newDeployment(seedNamespace, common.KibanaDeploymentName, common.GardenRoleLogging, true)

		requiredLoggingControlPlaneDeployments = []*appsv1.Deployment{
			kibanaDeployment,
		}

		elasticSearchStatefulSet = newStatefulSet(seedNamespace, common.ElasticSearchStatefulSetName, common.GardenRoleLogging, true)

		requiredLoggingControlPlaneStatefulSets = []*appsv1.StatefulSet{
			elasticSearchStatefulSet,
		}
	)

	DescribeTable("#CheckControlPlane",
		func(shoot *gardenv1beta1.Shoot, cloudProvider gardenv1beta1.CloudProvider, deployments []*appsv1.Deployment, statefulSets []*appsv1.StatefulSet, machineDeployments []*machinev1alpha1.MachineDeployment, conditionMatcher types.GomegaMatcher) {
			var (
				deploymentLister        = constDeploymentLister(deployments)
				statefulSetLister       = constStatefulSetLister(statefulSets)
				machineDeploymentLister = constMachineDeploymentLister(machineDeployments)
				checker                 = botanist.NewHealthChecker(map[gardencorev1alpha1.ConditionType]time.Duration{})
			)

			exitCondition, err := checker.CheckControlPlane(shoot, seedNamespace, cloudProvider, condition, deploymentLister, statefulSetLister, machineDeploymentLister)
			Expect(err).NotTo(HaveOccurred())
			Expect(exitCondition).To(conditionMatcher)
		},
		Entry("all healthy",
			gcpShoot,
			gardenv1beta1.CloudProviderGCP,
			requiredControlPlaneDeployments,
			requiredControlPlaneStatefulSets,
			nil,
			BeNil()),
		Entry("all healthy (AWS)",
			gcpShoot,
			gardenv1beta1.CloudProviderAWS,
			[]*appsv1.Deployment{
				cloudControllerManagerDeployment,
				kubeAddonManagerDeployment,
				kubeAPIServerDeployment,
				kubeControllerManagerDeployment,
				kubeSchedulerDeployment,
				machineControllerManagerDeployment,
				awsLBReadvertiserDeployment,
			},
			requiredControlPlaneStatefulSets,
			nil,
			BeNil()),
		Entry("all healthy (with autoscaler)",
			gcpShootWithAutoscaler,
			gardenv1beta1.CloudProviderGCP,
			[]*appsv1.Deployment{
				cloudControllerManagerDeployment,
				kubeAddonManagerDeployment,
				kubeAPIServerDeployment,
				kubeControllerManagerDeployment,
				kubeSchedulerDeployment,
				machineControllerManagerDeployment,
				clusterAutoscalerDeployment,
			},
			requiredControlPlaneStatefulSets,
			nil,
			BeNil()),
		Entry("missing required deployment",
			gcpShoot,
			gardenv1beta1.CloudProviderGCP,
			[]*appsv1.Deployment{
				kubeAddonManagerDeployment,
				kubeAPIServerDeployment,
				kubeControllerManagerDeployment,
				kubeSchedulerDeployment,
				machineControllerManagerDeployment,
			},
			requiredControlPlaneStatefulSets,
			nil,
			beConditionWithStatus(gardencorev1alpha1.ConditionFalse)),
		Entry("required deployment unhealthy",
			gcpShoot,
			gardenv1beta1.CloudProviderGCP,
			[]*appsv1.Deployment{
				cloudControllerManagerDeployment,
				newDeployment(kubeAddonManagerDeployment.Namespace, kubeAddonManagerDeployment.Name, roleOf(kubeAddonManagerDeployment), false),
				kubeAPIServerDeployment,
				kubeControllerManagerDeployment,
				kubeSchedulerDeployment,
				machineControllerManagerDeployment,
			},
			requiredControlPlaneStatefulSets,
			nil,
			beConditionWithStatus(gardencorev1alpha1.ConditionFalse)),
		Entry("missing required stateful set",
			gcpShoot,
			gardenv1beta1.CloudProviderGCP,
			requiredControlPlaneDeployments,
			[]*appsv1.StatefulSet{
				etcdEventsStatefulSet,
			},
			nil,
			beConditionWithStatus(gardencorev1alpha1.ConditionFalse)),
		Entry("required stateful set unhealthy",
			gcpShoot,
			gardenv1beta1.CloudProviderGCP,
			requiredControlPlaneDeployments,
			[]*appsv1.StatefulSet{
				newStatefulSet(etcdMainStatefulSet.Namespace, etcdMainStatefulSet.Name, roleOf(etcdMainStatefulSet), false),
				etcdEventsStatefulSet,
			},
			nil,
			beConditionWithStatus(gardencorev1alpha1.ConditionFalse)),
		Entry("rolling update ongoing (with autoscaler)",
			gcpShootWithAutoscaler,
			gardenv1beta1.CloudProviderGCP,
			[]*appsv1.Deployment{
				cloudControllerManagerDeployment,
				kubeAddonManagerDeployment,
				kubeAPIServerDeployment,
				kubeControllerManagerDeployment,
				kubeSchedulerDeployment,
				machineControllerManagerDeployment,
			},
			requiredControlPlaneStatefulSets,
			[]*machinev1alpha1.MachineDeployment{
				{Status: machinev1alpha1.MachineDeploymentStatus{Replicas: 2, UpdatedReplicas: 1}},
			},
			BeNil()),
	)

	DescribeTable("#CheckSystemComponents",
		func(deployments []*appsv1.Deployment, daemonSets []*appsv1.DaemonSet, conditionMatcher types.GomegaMatcher) {
			var (
				deploymentLister = constDeploymentLister(deployments)
				daemonSetLister  = constDaemonSetLister(daemonSets)
				checker          = botanist.NewHealthChecker(map[gardencorev1alpha1.ConditionType]time.Duration{})
			)

			exitCondition, err := checker.CheckSystemComponents(shootNamespace, condition, deploymentLister, daemonSetLister)
			Expect(err).NotTo(HaveOccurred())
			Expect(exitCondition).To(conditionMatcher)
		},
		Entry("all healthy",
			requiredSystemComponentDeployments,
			requiredSystemComponentDaemonSets,
			BeNil()),
		Entry("missing required deployment",
			[]*appsv1.Deployment{
				calicoTyphaDeployment,
				coreDNSDeployment,
				vpnShootDeployment,
			},
			requiredSystemComponentDaemonSets,
			beConditionWithStatus(gardencorev1alpha1.ConditionFalse)),
		Entry("missing required daemon set",
			requiredSystemComponentDeployments,
			[]*appsv1.DaemonSet{
				calicoNodeDaemonSet,
			},
			beConditionWithStatus(gardencorev1alpha1.ConditionFalse)),
		Entry("required deployment not healthy",
			[]*appsv1.Deployment{
				newDeployment(calicoTyphaDeployment.Namespace, calicoTyphaDeployment.Name, roleOf(calicoTyphaDeployment), false),
				coreDNSDeployment,
				vpnShootDeployment,
				metricsServerDeployment,
			},
			requiredSystemComponentDaemonSets,
			beConditionWithStatus(gardencorev1alpha1.ConditionFalse)),
		Entry("required daemon set not healthy",
			requiredSystemComponentDeployments,
			[]*appsv1.DaemonSet{
				newDaemonSet(kubeProxyDaemonSet.Namespace, kubeProxyDaemonSet.Name, roleOf(kubeProxyDaemonSet), false),
				calicoNodeDaemonSet,
			},
			beConditionWithStatus(gardencorev1alpha1.ConditionFalse)),
	)

	DescribeTable("#CheckClusterNodes",
		func(nodes []*corev1.Node, machineDeployments []*machinev1alpha1.MachineDeployment, conditionMatcher types.GomegaMatcher) {
			var (
				nodeLister              = constNodeLister(nodes)
				machineDeploymentLister = constMachineDeploymentLister(machineDeployments)
				checker                 = botanist.NewHealthChecker(map[gardencorev1alpha1.ConditionType]time.Duration{})
			)

			exitCondition, err := checker.CheckClusterNodes(seedNamespace, condition, nodeLister, machineDeploymentLister)
			Expect(err).NotTo(HaveOccurred())
			Expect(exitCondition).To(conditionMatcher)
		},
		Entry("all healthy",
			[]*corev1.Node{
				newNode("node1", true),
			},
			[]*machinev1alpha1.MachineDeployment{
				newMachineDeployment(seedNamespace, "machinedeployment", 1, true),
			},
			BeNil()),
		Entry("node not healthy",
			[]*corev1.Node{
				newNode("node1", false),
			},
			[]*machinev1alpha1.MachineDeployment{
				newMachineDeployment(seedNamespace, "machinedeployment", 1, true),
			},
			beConditionWithStatus(gardencorev1alpha1.ConditionFalse)),
		Entry("machine deployment not healthy",
			[]*corev1.Node{
				newNode("node1", true),
			},
			[]*machinev1alpha1.MachineDeployment{
				newMachineDeployment(seedNamespace, "machinedeployment", 1, false),
			},
			beConditionWithStatus(gardencorev1alpha1.ConditionFalse)),
		Entry("not enough nodes",
			[]*corev1.Node{},
			[]*machinev1alpha1.MachineDeployment{
				newMachineDeployment(seedNamespace, "machinedeployment", 1, true),
			},
			beConditionWithStatus(gardencorev1alpha1.ConditionFalse)),
	)

	DescribeTable("#CheckMonitoringSystemComponents",
		func(daemonSets []*appsv1.DaemonSet, conditionMatcher types.GomegaMatcher) {
			var (
				daemonSetLister = constDaemonSetLister(daemonSets)
				checker         = botanist.NewHealthChecker(map[gardencorev1alpha1.ConditionType]time.Duration{})
			)

			exitCondition, err := checker.CheckMonitoringSystemComponents(shootNamespace, condition, daemonSetLister)
			Expect(err).NotTo(HaveOccurred())
			Expect(exitCondition).To(conditionMatcher)
		},
		Entry("all healthy",
			requiredMonitoringSystemComponentDaemonSets,
			BeNil()),
		Entry("required daemon set missing",
			[]*appsv1.DaemonSet{},
			beConditionWithStatus(gardencorev1alpha1.ConditionFalse)),
		Entry("daemon set unhealthy",
			[]*appsv1.DaemonSet{newDaemonSet(nodeExporterDaemonSet.Namespace, nodeExporterDaemonSet.Name, roleOf(nodeExporterDaemonSet), false)},
			beConditionWithStatus(gardencorev1alpha1.ConditionFalse)),
	)

	DescribeTable("#CheckMonitoringControlPlane",
		func(deployments []*appsv1.Deployment, statefulSets []*appsv1.StatefulSet, wantsAlertmanager bool, conditionMatcher types.GomegaMatcher) {
			var (
				deploymentLister  = constDeploymentLister(deployments)
				statefulSetLister = constStatefulSetLister(statefulSets)
				checker           = botanist.NewHealthChecker(map[gardencorev1alpha1.ConditionType]time.Duration{})
			)

			exitCondition, err := checker.CheckMonitoringControlPlane(seedNamespace, wantsAlertmanager, condition, deploymentLister, statefulSetLister)
			Expect(err).NotTo(HaveOccurred())
			Expect(exitCondition).To(conditionMatcher)
		},
		Entry("all healthy",
			requiredMonitoringControlPlaneDeployments,
			requiredMonitoringControlPlaneStatefulSets,
			true,
			BeNil()),
		Entry("required deployment set missing",
			[]*appsv1.Deployment{
				kubeStateMetricsSeedDeployment,
				kubeStateMetricsShootDeployment,
			},
			requiredMonitoringControlPlaneStatefulSets,
			true,
			beConditionWithStatus(gardencorev1alpha1.ConditionFalse)),
		Entry("required stateful set set missing",
			requiredMonitoringControlPlaneDeployments,
			[]*appsv1.StatefulSet{
				prometheusStatefulSet,
			},
			true,
			beConditionWithStatus(gardencorev1alpha1.ConditionFalse)),
		Entry("deployment unhealthy",
			[]*appsv1.Deployment{
				newDeployment(grafanaDeployment.Namespace, grafanaDeployment.Name, roleOf(grafanaDeployment), false),
				kubeStateMetricsSeedDeployment,
				kubeStateMetricsShootDeployment,
			},
			requiredMonitoringControlPlaneStatefulSets,
			true, beConditionWithStatus(gardencorev1alpha1.ConditionFalse)),
		Entry("stateful set unhealthy",
			requiredMonitoringControlPlaneDeployments,
			[]*appsv1.StatefulSet{
				newStatefulSet(alertManagerStatefulSet.Namespace, alertManagerStatefulSet.Name, roleOf(alertManagerStatefulSet), false),
				prometheusStatefulSet,
			},
			true,
			beConditionWithStatus(gardencorev1alpha1.ConditionFalse)),
	)

	DescribeTable("#CheckOptionalAddonsSystemComponents",
		func(deployments []*appsv1.Deployment, daemonSets []*appsv1.DaemonSet, conditionMatcher types.GomegaMatcher) {
			var (
				deploymentLister = constDeploymentLister(deployments)
				daemonSetLister  = constDaemonSetLister(daemonSets)
				checker          = botanist.NewHealthChecker(map[gardencorev1alpha1.ConditionType]time.Duration{})
			)

			exitCondition, err := checker.CheckOptionalAddonsSystemComponents(shootNamespace, condition, deploymentLister, daemonSetLister)
			Expect(err).NotTo(HaveOccurred())
			Expect(exitCondition).To(conditionMatcher)
		},
		Entry("all healthy",
			nil,
			nil,
			BeNil()),
		Entry("deployment unhealthy",
			[]*appsv1.Deployment{newDeployment(shootNamespace, "addon", common.GardenRoleOptionalAddon, false)},
			nil,
			beConditionWithStatus(gardencorev1alpha1.ConditionFalse)),
		Entry("deployment unhealthy",
			nil,
			[]*appsv1.DaemonSet{newDaemonSet(shootNamespace, "addon", common.GardenRoleOptionalAddon, false)},
			beConditionWithStatus(gardencorev1alpha1.ConditionFalse)),
	)

	DescribeTable("#CheckLoggingControlPlane",
		func(deployments []*appsv1.Deployment, statefulSets []*appsv1.StatefulSet, conditionMatcher types.GomegaMatcher) {
			var (
				deploymentLister  = constDeploymentLister(deployments)
				statefulSetLister = constStatefulSetLister(statefulSets)
				checker           = botanist.NewHealthChecker(map[gardencorev1alpha1.ConditionType]time.Duration{})
			)

			exitCondition, err := checker.CheckLoggingControlPlane(seedNamespace, condition, deploymentLister, statefulSetLister)
			Expect(err).NotTo(HaveOccurred())
			Expect(exitCondition).To(conditionMatcher)
		},
		Entry("all healthy",
			requiredLoggingControlPlaneDeployments,
			requiredLoggingControlPlaneStatefulSets,
			BeNil()),
		Entry("required deployment missing",
			nil,
			requiredLoggingControlPlaneStatefulSets,
			beConditionWithStatus(gardencorev1alpha1.ConditionFalse)),
		Entry("required stateful set missing",
			requiredLoggingControlPlaneDeployments,
			nil,
			beConditionWithStatus(gardencorev1alpha1.ConditionFalse)),
		Entry("deployment unhealthy",
			[]*appsv1.Deployment{newDeployment(kibanaDeployment.Namespace, kibanaDeployment.Name, roleOf(kibanaDeployment), false)},
			requiredLoggingControlPlaneStatefulSets,
			beConditionWithStatus(gardencorev1alpha1.ConditionFalse)),
		Entry("stateful set unhealthy",
			requiredLoggingControlPlaneDeployments,
			[]*appsv1.StatefulSet{
				newStatefulSet(elasticSearchStatefulSet.Namespace, elasticSearchStatefulSet.Name, roleOf(elasticSearchStatefulSet), false),
			},
			beConditionWithStatus(gardencorev1alpha1.ConditionFalse)),
	)

	DescribeTable("#FailedCondition",
		func(thresholds map[gardencorev1alpha1.ConditionType]time.Duration, transitionTime metav1.Time, now time.Time, condition gardencorev1alpha1.Condition, expected types.GomegaMatcher) {
			checker := botanist.NewHealthChecker(thresholds)
			tmp1, tmp2 := botanist.Now, helper.Now
			defer func() {
				botanist.Now, helper.Now = tmp1, tmp2
			}()
			botanist.Now, helper.Now = func() time.Time {
				return now
			}, func() metav1.Time {
				return transitionTime
			}

			Expect(checker.FailedCondition(condition, "", "")).To(expected)
		},
		Entry("true condition with threshold",
			map[gardencorev1alpha1.ConditionType]time.Duration{
				gardenv1beta1.ShootControlPlaneHealthy: time.Minute,
			},
			zeroMetaTime,
			zeroTime,
			gardencorev1alpha1.Condition{
				Type:   gardenv1beta1.ShootControlPlaneHealthy,
				Status: gardencorev1alpha1.ConditionTrue,
			},
			MatchFields(IgnoreExtras, Fields{
				"Status": Equal(gardencorev1alpha1.ConditionProgressing),
			})),
		Entry("true condition without threshold",
			map[gardencorev1alpha1.ConditionType]time.Duration{},
			zeroMetaTime,
			zeroTime,
			gardencorev1alpha1.Condition{
				Type:   gardenv1beta1.ShootControlPlaneHealthy,
				Status: gardencorev1alpha1.ConditionTrue,
			},
			MatchFields(IgnoreExtras, Fields{
				"Status": Equal(gardencorev1alpha1.ConditionFalse),
			})),
		Entry("progressing condition within threshold",
			map[gardencorev1alpha1.ConditionType]time.Duration{
				gardenv1beta1.ShootControlPlaneHealthy: time.Minute,
			},
			zeroMetaTime,
			zeroTime,
			gardencorev1alpha1.Condition{
				Type:   gardenv1beta1.ShootControlPlaneHealthy,
				Status: gardencorev1alpha1.ConditionProgressing,
			},
			MatchFields(IgnoreExtras, Fields{
				"Status": Equal(gardencorev1alpha1.ConditionProgressing),
			})),
		Entry("progressing condition outside threshold",
			map[gardencorev1alpha1.ConditionType]time.Duration{
				gardenv1beta1.ShootControlPlaneHealthy: time.Minute,
			},
			zeroMetaTime,
			zeroTime.Add(time.Minute+time.Second),
			gardencorev1alpha1.Condition{
				Type:   gardenv1beta1.ShootControlPlaneHealthy,
				Status: gardencorev1alpha1.ConditionProgressing,
			},
			MatchFields(IgnoreExtras, Fields{
				"Status": Equal(gardencorev1alpha1.ConditionFalse),
			})),
		Entry("failed condition",
			map[gardencorev1alpha1.ConditionType]time.Duration{},
			zeroMetaTime,
			zeroTime,
			gardencorev1alpha1.Condition{
				Type:   gardenv1beta1.ShootControlPlaneHealthy,
				Status: gardencorev1alpha1.ConditionFalse,
			},
			MatchFields(IgnoreExtras, Fields{
				"Status": Equal(gardencorev1alpha1.ConditionFalse),
			})),
	)
})

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
	v1alpha1constants "github.com/gardener/gardener/pkg/apis/core/v1alpha1/constants"
	"github.com/gardener/gardener/pkg/apis/garden/v1beta1/helper"
	"github.com/gardener/gardener/pkg/operation/botanist"
	"github.com/gardener/gardener/pkg/operation/common"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/onsi/gomega/types"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	return obj.GetLabels()[v1alpha1constants.DeprecatedGardenRole]
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
	return map[string]string{v1alpha1constants.DeprecatedGardenRole: role}
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
		gcpShoot               = &gardencorev1alpha1.Shoot{}
		gcpShootWithAutoscaler = &gardencorev1alpha1.Shoot{
			Spec: gardencorev1alpha1.ShootSpec{
				Provider: gardencorev1alpha1.Provider{
					Workers: []gardencorev1alpha1.Worker{
						{
							Name:    "foo",
							Minimum: 1,
							Maximum: 2,
						},
					},
				},
			},
		}

		seedNamespace  = "shoot--foo--bar"
		shootNamespace = metav1.NamespaceSystem

		// control plane deployments
		gardenerResourceManagerDeployment  = newDeployment(seedNamespace, v1alpha1constants.DeploymentNameGardenerResourceManager, v1alpha1constants.GardenRoleControlPlane, true)
		kubeAPIServerDeployment            = newDeployment(seedNamespace, v1alpha1constants.DeploymentNameKubeAPIServer, v1alpha1constants.GardenRoleControlPlane, true)
		kubeControllerManagerDeployment    = newDeployment(seedNamespace, v1alpha1constants.DeploymentNameKubeControllerManager, v1alpha1constants.GardenRoleControlPlane, true)
		kubeSchedulerDeployment            = newDeployment(seedNamespace, v1alpha1constants.DeploymentNameKubeScheduler, v1alpha1constants.GardenRoleControlPlane, true)
		machineControllerManagerDeployment = newDeployment(seedNamespace, common.MachineControllerManagerDeploymentName, v1alpha1constants.GardenRoleControlPlane, true)
		clusterAutoscalerDeployment        = newDeployment(seedNamespace, v1alpha1constants.DeploymentNameClusterAutoscaler, v1alpha1constants.GardenRoleControlPlane, true)

		requiredControlPlaneDeployments = []*appsv1.Deployment{
			gardenerResourceManagerDeployment,
			kubeAPIServerDeployment,
			kubeControllerManagerDeployment,
			kubeSchedulerDeployment,
			machineControllerManagerDeployment,
			clusterAutoscalerDeployment,
		}

		// control plane stateful sets
		etcdMainStatefulSet   = newStatefulSet(seedNamespace, v1alpha1constants.StatefulSetNameETCDMain, v1alpha1constants.GardenRoleControlPlane, true)
		etcdEventsStatefulSet = newStatefulSet(seedNamespace, v1alpha1constants.StatefulSetNameETCDEvents, v1alpha1constants.GardenRoleControlPlane, true)

		requiredControlPlaneStatefulSets = []*appsv1.StatefulSet{
			etcdMainStatefulSet,
			etcdEventsStatefulSet,
		}

		// system component deployments
		calicoKubeControllersDeployment = newDeployment(shootNamespace, common.CalicoKubeControllersDeploymentName, v1alpha1constants.GardenRoleSystemComponent, true)
		coreDNSDeployment               = newDeployment(shootNamespace, common.CoreDNSDeploymentName, v1alpha1constants.GardenRoleSystemComponent, true)
		vpnShootDeployment              = newDeployment(shootNamespace, common.VPNShootDeploymentName, v1alpha1constants.GardenRoleSystemComponent, true)
		metricsServerDeployment         = newDeployment(shootNamespace, common.MetricsServerDeploymentName, v1alpha1constants.GardenRoleSystemComponent, true)

		requiredSystemComponentDeployments = []*appsv1.Deployment{
			calicoKubeControllersDeployment,
			coreDNSDeployment,
			vpnShootDeployment,
			metricsServerDeployment,
		}

		// system component daemon sets
		calicoNodeDaemonSet          = newDaemonSet(shootNamespace, common.CalicoNodeDaemonSetName, v1alpha1constants.GardenRoleSystemComponent, true)
		kubeProxyDaemonSet           = newDaemonSet(shootNamespace, common.KubeProxyDaemonSetName, v1alpha1constants.GardenRoleSystemComponent, true)
		nodeProblemDetectorDaemonSet = newDaemonSet(shootNamespace, common.NodeProblemDetectorDaemonSetName, v1alpha1constants.GardenRoleSystemComponent, true)

		requiredSystemComponentDaemonSets = []*appsv1.DaemonSet{
			calicoNodeDaemonSet,
			kubeProxyDaemonSet,
			nodeProblemDetectorDaemonSet,
		}

		nodeExporterDaemonSet = newDaemonSet(shootNamespace, common.NodeExporterDaemonSetName, v1alpha1constants.GardenRoleMonitoring, true)

		requiredMonitoringSystemComponentDaemonSets = []*appsv1.DaemonSet{
			nodeExporterDaemonSet,
		}

		grafanaDeployment               = newDeployment(seedNamespace, v1alpha1constants.DeploymentNameGrafanaOperators, v1alpha1constants.GardenRoleMonitoring, true)
		grafanaDeploymentUsers          = newDeployment(seedNamespace, v1alpha1constants.DeploymentNameGrafanaUsers, v1alpha1constants.GardenRoleMonitoring, true)
		kubeStateMetricsSeedDeployment  = newDeployment(seedNamespace, v1alpha1constants.DeploymentNameKubeStateMetricsSeed, v1alpha1constants.GardenRoleMonitoring, true)
		kubeStateMetricsShootDeployment = newDeployment(seedNamespace, v1alpha1constants.DeploymentNameKubeStateMetricsShoot, v1alpha1constants.GardenRoleMonitoring, true)

		requiredMonitoringControlPlaneDeployments = []*appsv1.Deployment{
			grafanaDeployment,
			grafanaDeploymentUsers,
			kubeStateMetricsSeedDeployment,
			kubeStateMetricsShootDeployment,
		}

		alertManagerStatefulSet = newStatefulSet(seedNamespace, v1alpha1constants.StatefulSetNameAlertManager, v1alpha1constants.GardenRoleMonitoring, true)
		prometheusStatefulSet   = newStatefulSet(seedNamespace, v1alpha1constants.StatefulSetNamePrometheus, v1alpha1constants.GardenRoleMonitoring, true)

		requiredMonitoringControlPlaneStatefulSets = []*appsv1.StatefulSet{
			alertManagerStatefulSet,
			prometheusStatefulSet,
		}

		kibanaDeployment = newDeployment(seedNamespace, v1alpha1constants.DeploymentNameKibana, v1alpha1constants.GardenRoleLogging, true)

		requiredLoggingControlPlaneDeployments = []*appsv1.Deployment{
			kibanaDeployment,
		}

		elasticSearchStatefulSet = newStatefulSet(seedNamespace, v1alpha1constants.StatefulSetNameElasticSearch, v1alpha1constants.GardenRoleLogging, true)

		requiredLoggingControlPlaneStatefulSets = []*appsv1.StatefulSet{
			elasticSearchStatefulSet,
		}
	)

	DescribeTable("#CheckControlPlane",
		func(shoot *gardencorev1alpha1.Shoot, cloudProvider string, deployments []*appsv1.Deployment, statefulSets []*appsv1.StatefulSet, machineDeployments []*machinev1alpha1.MachineDeployment, conditionMatcher types.GomegaMatcher) {
			var (
				deploymentLister        = constDeploymentLister(deployments)
				statefulSetLister       = constStatefulSetLister(statefulSets)
				machineDeploymentLister = constMachineDeploymentLister(machineDeployments)
				checker                 = botanist.NewHealthChecker(map[gardencorev1alpha1.ConditionType]time.Duration{})
			)

			exitCondition, err := checker.CheckControlPlane(shoot, seedNamespace, condition, deploymentLister, statefulSetLister, machineDeploymentLister)
			Expect(err).NotTo(HaveOccurred())
			Expect(exitCondition).To(conditionMatcher)
		},
		Entry("all healthy",
			gcpShoot,
			"gcp",
			requiredControlPlaneDeployments,
			requiredControlPlaneStatefulSets,
			nil,
			BeNil()),
		Entry("all healthy (AWS)",
			gcpShoot,
			"aws",
			[]*appsv1.Deployment{
				gardenerResourceManagerDeployment,
				kubeAPIServerDeployment,
				kubeControllerManagerDeployment,
				kubeSchedulerDeployment,
				machineControllerManagerDeployment,
			},
			requiredControlPlaneStatefulSets,
			nil,
			BeNil()),
		Entry("all healthy (with autoscaler)",
			gcpShootWithAutoscaler,
			"gcp",
			[]*appsv1.Deployment{
				gardenerResourceManagerDeployment,
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
			"gcp",
			[]*appsv1.Deployment{
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
			"gcp",
			[]*appsv1.Deployment{
				newDeployment(gardenerResourceManagerDeployment.Namespace, gardenerResourceManagerDeployment.Name, roleOf(gardenerResourceManagerDeployment), false),
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
			"gcp",
			requiredControlPlaneDeployments,
			[]*appsv1.StatefulSet{
				etcdEventsStatefulSet,
			},
			nil,
			beConditionWithStatus(gardencorev1alpha1.ConditionFalse)),
		Entry("required stateful set unhealthy",
			gcpShoot,
			"gcp",
			requiredControlPlaneDeployments,
			[]*appsv1.StatefulSet{
				newStatefulSet(etcdMainStatefulSet.Namespace, etcdMainStatefulSet.Name, roleOf(etcdMainStatefulSet), false),
				etcdEventsStatefulSet,
			},
			nil,
			beConditionWithStatus(gardencorev1alpha1.ConditionFalse)),
		Entry("rolling update ongoing (with autoscaler)",
			gcpShootWithAutoscaler,
			"gcp",
			[]*appsv1.Deployment{
				gardenerResourceManagerDeployment,
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
		func(gardenerVersion string, deployments []*appsv1.Deployment, daemonSets []*appsv1.DaemonSet, conditionMatcher types.GomegaMatcher) {
			var (
				deploymentLister = constDeploymentLister(deployments)
				daemonSetLister  = constDaemonSetLister(daemonSets)
				checker          = botanist.NewHealthChecker(map[gardencorev1alpha1.ConditionType]time.Duration{})
			)

			exitCondition, err := checker.CheckSystemComponents(gardenerVersion, shootNamespace, condition, deploymentLister, daemonSetLister)
			Expect(err).NotTo(HaveOccurred())
			Expect(exitCondition).To(conditionMatcher)
		},
		Entry("all healthy",
			"0.100.200",
			requiredSystemComponentDeployments,
			requiredSystemComponentDaemonSets,
			BeNil()),
		Entry("missing required deployment",
			"0.100.200",
			[]*appsv1.Deployment{
				calicoKubeControllersDeployment,
				coreDNSDeployment,
				vpnShootDeployment,
			},
			requiredSystemComponentDaemonSets,
			beConditionWithStatus(gardencorev1alpha1.ConditionFalse)),
		Entry("missing required daemon set",
			"0.100.200",
			requiredSystemComponentDeployments,
			[]*appsv1.DaemonSet{
				calicoNodeDaemonSet,
			},
			beConditionWithStatus(gardencorev1alpha1.ConditionFalse)),
		Entry("required deployment not healthy",
			"0.100.200",
			[]*appsv1.Deployment{
				calicoKubeControllersDeployment,
				newDeployment(coreDNSDeployment.Namespace, coreDNSDeployment.Name, roleOf(coreDNSDeployment), false),
				vpnShootDeployment,
				metricsServerDeployment,
			},
			requiredSystemComponentDaemonSets,
			beConditionWithStatus(gardencorev1alpha1.ConditionFalse)),
		Entry("required daemon set not healthy",
			"0.100.200",
			requiredSystemComponentDeployments,
			[]*appsv1.DaemonSet{
				newDaemonSet(kubeProxyDaemonSet.Namespace, kubeProxyDaemonSet.Name, roleOf(kubeProxyDaemonSet), false),
				calicoNodeDaemonSet,
			},
			beConditionWithStatus(gardencorev1alpha1.ConditionFalse)),
		Entry("node-problem-detector missing but still all healthy (gardener < 0.31)",
			"0.30.5",
			requiredSystemComponentDeployments,
			[]*appsv1.DaemonSet{calicoNodeDaemonSet, kubeProxyDaemonSet},
			BeNil()),
		Entry("node-problem-detector missing and condition fails (gardener >= 0.31)",
			"0.31.2",
			requiredSystemComponentDeployments,
			[]*appsv1.DaemonSet{calicoNodeDaemonSet, kubeProxyDaemonSet},
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
				grafanaDeploymentUsers,
				kubeStateMetricsSeedDeployment,
				kubeStateMetricsShootDeployment,
			},
			requiredMonitoringControlPlaneStatefulSets,
			true,
			beConditionWithStatus(gardencorev1alpha1.ConditionFalse)),
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
			[]*appsv1.Deployment{newDeployment(shootNamespace, "addon", v1alpha1constants.GardenRoleOptionalAddon, false)},
			nil,
			beConditionWithStatus(gardencorev1alpha1.ConditionFalse)),
		Entry("deployment unhealthy",
			nil,
			[]*appsv1.DaemonSet{newDaemonSet(shootNamespace, "addon", v1alpha1constants.GardenRoleOptionalAddon, false)},
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
				gardencorev1alpha1.ShootControlPlaneHealthy: time.Minute,
			},
			zeroMetaTime,
			zeroTime,
			gardencorev1alpha1.Condition{
				Type:   gardencorev1alpha1.ShootControlPlaneHealthy,
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
				Type:   gardencorev1alpha1.ShootControlPlaneHealthy,
				Status: gardencorev1alpha1.ConditionTrue,
			},
			MatchFields(IgnoreExtras, Fields{
				"Status": Equal(gardencorev1alpha1.ConditionFalse),
			})),
		Entry("progressing condition within threshold",
			map[gardencorev1alpha1.ConditionType]time.Duration{
				gardencorev1alpha1.ShootControlPlaneHealthy: time.Minute,
			},
			zeroMetaTime,
			zeroTime,
			gardencorev1alpha1.Condition{
				Type:   gardencorev1alpha1.ShootControlPlaneHealthy,
				Status: gardencorev1alpha1.ConditionProgressing,
			},
			MatchFields(IgnoreExtras, Fields{
				"Status": Equal(gardencorev1alpha1.ConditionProgressing),
			})),
		Entry("progressing condition outside threshold",
			map[gardencorev1alpha1.ConditionType]time.Duration{
				gardencorev1alpha1.ShootControlPlaneHealthy: time.Minute,
			},
			zeroMetaTime,
			zeroTime.Add(time.Minute+time.Second),
			gardencorev1alpha1.Condition{
				Type:   gardencorev1alpha1.ShootControlPlaneHealthy,
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
				Type:   gardencorev1alpha1.ShootControlPlaneHealthy,
				Status: gardencorev1alpha1.ConditionFalse,
			},
			MatchFields(IgnoreExtras, Fields{
				"Status": Equal(gardencorev1alpha1.ConditionFalse),
			})),
	)
})

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
	"fmt"
	"time"

	druidv1alpha1 "github.com/gardener/etcd-druid/api/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/operation/botanist"
	"github.com/gardener/gardener/pkg/operation/common"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/onsi/gomega/types"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/utils/pointer"
)

var (
	zeroTime     time.Time
	zeroMetaTime metav1.Time
)

func roleOf(obj metav1.Object) string {
	return obj.GetLabels()[v1beta1constants.DeprecatedGardenRole]
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

func constWorkerLister(workers []*extensionsv1alpha1.Worker) kutil.WorkerLister {
	return kutil.NewWorkerLister(func() ([]*extensionsv1alpha1.Worker, error) {
		return workers, nil
	})
}

func constEtcdLister(etcds []*druidv1alpha1.Etcd) kutil.EtcdLister {
	return kutil.NewEtcdLister(func() ([]*druidv1alpha1.Etcd, error) {
		return etcds, nil
	})
}

func roleLabels(role string) map[string]string {
	return map[string]string{v1beta1constants.DeprecatedGardenRole: role}
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

func newEtcd(namespace, name, role string, healthy bool, lastError *string) *druidv1alpha1.Etcd {
	etcd := &druidv1alpha1.Etcd{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
			Labels:    roleLabels(role),
		},
	}
	if healthy {
		etcd.Status.Ready = pointer.BoolPtr(true)
	} else {
		etcd.Status.Ready = pointer.BoolPtr(false)
		etcd.Status.LastError = lastError
	}

	return etcd
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

func newNode(name string, healthy bool, set labels.Set) *corev1.Node {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: set,
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

func beConditionWithStatus(status gardencorev1beta1.ConditionStatus) types.GomegaMatcher {
	return PointTo(MatchFields(IgnoreExtras, Fields{
		"Status": Equal(status),
	}))
}

func beConditionWithStatusAndCodes(status gardencorev1beta1.ConditionStatus, codes ...gardencorev1beta1.ErrorCode) types.GomegaMatcher {
	return PointTo(MatchFields(IgnoreExtras, Fields{
		"Status": Equal(status),
		"Codes":  Equal(codes),
	}))
}

func beConditionWithStatusAndMsg(status gardencorev1beta1.ConditionStatus, reason, message string) types.GomegaMatcher {
	return PointTo(MatchFields(IgnoreExtras, Fields{
		"Status":  Equal(status),
		"Reason":  Equal(reason),
		"Message": ContainSubstring(message),
	}))
}

var _ = Describe("health check", func() {
	var (
		condition = gardencorev1beta1.Condition{
			Type: gardencorev1beta1.ConditionType("test"),
		}
		gcpShoot                    = &gardencorev1beta1.Shoot{}
		gcpShootThatNeedsAutoscaler = &gardencorev1beta1.Shoot{
			Spec: gardencorev1beta1.ShootSpec{
				Provider: gardencorev1beta1.Provider{
					Workers: []gardencorev1beta1.Worker{
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
		gardenerResourceManagerDeployment = newDeployment(seedNamespace, v1beta1constants.DeploymentNameGardenerResourceManager, v1beta1constants.GardenRoleControlPlane, true)
		kubeAPIServerDeployment           = newDeployment(seedNamespace, v1beta1constants.DeploymentNameKubeAPIServer, v1beta1constants.GardenRoleControlPlane, true)
		kubeControllerManagerDeployment   = newDeployment(seedNamespace, v1beta1constants.DeploymentNameKubeControllerManager, v1beta1constants.GardenRoleControlPlane, true)
		kubeSchedulerDeployment           = newDeployment(seedNamespace, v1beta1constants.DeploymentNameKubeScheduler, v1beta1constants.GardenRoleControlPlane, true)
		clusterAutoscalerDeployment       = newDeployment(seedNamespace, v1beta1constants.DeploymentNameClusterAutoscaler, v1beta1constants.GardenRoleControlPlane, true)

		requiredControlPlaneDeployments = []*appsv1.Deployment{
			gardenerResourceManagerDeployment,
			kubeAPIServerDeployment,
			kubeControllerManagerDeployment,
			kubeSchedulerDeployment,
			clusterAutoscalerDeployment,
		}

		// control plane etcds
		etcdMain   = newEtcd(seedNamespace, v1beta1constants.ETCDMain, v1beta1constants.GardenRoleControlPlane, true, nil)
		etcdEvents = newEtcd(seedNamespace, v1beta1constants.ETCDEvents, v1beta1constants.GardenRoleControlPlane, true, nil)

		requiredControlPlaneEtcds = []*druidv1alpha1.Etcd{
			etcdMain,
			etcdEvents,
		}

		// system component deployments
		coreDNSDeployment       = newDeployment(shootNamespace, common.CoreDNSDeploymentName, v1beta1constants.GardenRoleSystemComponent, true)
		vpnShootDeployment      = newDeployment(shootNamespace, common.VPNShootDeploymentName, v1beta1constants.GardenRoleSystemComponent, true)
		metricsServerDeployment = newDeployment(shootNamespace, common.MetricsServerDeploymentName, v1beta1constants.GardenRoleSystemComponent, true)

		requiredSystemComponentDeployments = []*appsv1.Deployment{
			coreDNSDeployment,
			vpnShootDeployment,
			metricsServerDeployment,
		}

		// system component daemon sets
		kubeProxyDaemonSet           = newDaemonSet(shootNamespace, common.KubeProxyDaemonSetName, v1beta1constants.GardenRoleSystemComponent, true)
		nodeProblemDetectorDaemonSet = newDaemonSet(shootNamespace, common.NodeProblemDetectorDaemonSetName, v1beta1constants.GardenRoleSystemComponent, true)

		requiredSystemComponentDaemonSets = []*appsv1.DaemonSet{
			kubeProxyDaemonSet,
			nodeProblemDetectorDaemonSet,
		}

		blackboxExporterDeployment = newDeployment(shootNamespace, common.BlackboxExporterDeploymentName, v1beta1constants.GardenRoleMonitoring, true)

		requiredMonitoringSystemComponentDeployments = []*appsv1.Deployment{
			blackboxExporterDeployment,
		}

		nodeExporterDaemonSet = newDaemonSet(shootNamespace, common.NodeExporterDaemonSetName, v1beta1constants.GardenRoleMonitoring, true)

		requiredMonitoringSystemComponentDaemonSets = []*appsv1.DaemonSet{
			nodeExporterDaemonSet,
		}

		grafanaDeploymentOperators      = newDeployment(seedNamespace, v1beta1constants.DeploymentNameGrafanaOperators, v1beta1constants.GardenRoleMonitoring, true)
		grafanaDeploymentUsers          = newDeployment(seedNamespace, v1beta1constants.DeploymentNameGrafanaUsers, v1beta1constants.GardenRoleMonitoring, true)
		kubeStateMetricsSeedDeployment  = newDeployment(seedNamespace, v1beta1constants.DeploymentNameKubeStateMetricsSeed, v1beta1constants.GardenRoleMonitoring, true)
		kubeStateMetricsShootDeployment = newDeployment(seedNamespace, v1beta1constants.DeploymentNameKubeStateMetricsShoot, v1beta1constants.GardenRoleMonitoring, true)

		requiredMonitoringControlPlaneDeployments = []*appsv1.Deployment{
			grafanaDeploymentOperators,
			grafanaDeploymentUsers,
			kubeStateMetricsSeedDeployment,
			kubeStateMetricsShootDeployment,
		}

		alertManagerStatefulSet = newStatefulSet(seedNamespace, v1beta1constants.StatefulSetNameAlertManager, v1beta1constants.GardenRoleMonitoring, true)
		prometheusStatefulSet   = newStatefulSet(seedNamespace, v1beta1constants.StatefulSetNamePrometheus, v1beta1constants.GardenRoleMonitoring, true)

		requiredMonitoringControlPlaneStatefulSets = []*appsv1.StatefulSet{
			alertManagerStatefulSet,
			prometheusStatefulSet,
		}

		kibanaDeployment = newDeployment(seedNamespace, v1beta1constants.DeploymentNameKibana, v1beta1constants.GardenRoleLogging, true)

		requiredLoggingControlPlaneDeployments = []*appsv1.Deployment{
			kibanaDeployment,
		}

		elasticSearchStatefulSet = newStatefulSet(seedNamespace, v1beta1constants.StatefulSetNameElasticSearch, v1beta1constants.GardenRoleLogging, true)

		requiredLoggingControlPlaneStatefulSets = []*appsv1.StatefulSet{
			elasticSearchStatefulSet,
		}
	)

	DescribeTable("#CheckControlPlane",
		func(shoot *gardencorev1beta1.Shoot, cloudProvider string, deployments []*appsv1.Deployment, etcds []*druidv1alpha1.Etcd, workers []*extensionsv1alpha1.Worker, conditionMatcher types.GomegaMatcher) {
			var (
				deploymentLister = constDeploymentLister(deployments)
				etcdLister       = constEtcdLister(etcds)
				workerLister     = constWorkerLister(workers)
				checker          = botanist.NewHealthChecker(map[gardencorev1beta1.ConditionType]time.Duration{}, nil)
			)

			exitCondition, err := checker.CheckControlPlane(shoot, seedNamespace, condition, deploymentLister, etcdLister, workerLister)
			Expect(err).NotTo(HaveOccurred())
			Expect(exitCondition).To(conditionMatcher)
		},
		Entry("all healthy",
			gcpShoot,
			"gcp",
			requiredControlPlaneDeployments,
			requiredControlPlaneEtcds,
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
			},
			requiredControlPlaneEtcds,
			nil,
			BeNil()),
		Entry("all healthy (needs autoscaler)",
			gcpShootThatNeedsAutoscaler,
			"gcp",
			[]*appsv1.Deployment{
				gardenerResourceManagerDeployment,
				kubeAPIServerDeployment,
				kubeControllerManagerDeployment,
				kubeSchedulerDeployment,
				clusterAutoscalerDeployment,
			},
			requiredControlPlaneEtcds,
			[]*extensionsv1alpha1.Worker{
				{Status: extensionsv1alpha1.WorkerStatus{DefaultStatus: extensionsv1alpha1.DefaultStatus{
					LastOperation: &gardencorev1beta1.LastOperation{
						State: gardencorev1beta1.LastOperationStateSucceeded}}}},
			},
			BeNil()),
		Entry("missing required deployment",
			gcpShoot,
			"gcp",
			[]*appsv1.Deployment{
				kubeAPIServerDeployment,
				kubeControllerManagerDeployment,
				kubeSchedulerDeployment,
			},
			requiredControlPlaneEtcds,
			nil,
			beConditionWithStatus(gardencorev1beta1.ConditionFalse)),
		Entry("required deployment unhealthy",
			gcpShoot,
			"gcp",
			[]*appsv1.Deployment{
				newDeployment(gardenerResourceManagerDeployment.Namespace, gardenerResourceManagerDeployment.Name, roleOf(gardenerResourceManagerDeployment), false),
				kubeAPIServerDeployment,
				kubeControllerManagerDeployment,
				kubeSchedulerDeployment,
			},
			requiredControlPlaneEtcds,
			nil,
			beConditionWithStatus(gardencorev1beta1.ConditionFalse)),
		Entry("missing required etcd",
			gcpShoot,
			"gcp",
			requiredControlPlaneDeployments,
			[]*druidv1alpha1.Etcd{
				etcdEvents,
			},
			nil,
			beConditionWithStatus(gardencorev1beta1.ConditionFalse)),
		Entry("required etcd unready",
			gcpShoot,
			"gcp",
			requiredControlPlaneDeployments,
			[]*druidv1alpha1.Etcd{
				newEtcd(etcdMain.Namespace, etcdMain.Name, roleOf(etcdMain), false, nil),
				etcdEvents,
			},
			nil,
			beConditionWithStatus(gardencorev1beta1.ConditionFalse)),
		Entry("required etcd unhealthy with error code message",
			gcpShoot,
			"gcp",
			requiredControlPlaneDeployments,
			[]*druidv1alpha1.Etcd{
				newEtcd(etcdMain.Namespace, etcdMain.Name, roleOf(etcdMain), false, pointer.StringPtr("some error that maps to an error code, e.g. unauthorized")),
				etcdEvents,
			},
			nil,
			beConditionWithStatusAndCodes(gardencorev1beta1.ConditionFalse, gardencorev1beta1.ErrorInfraUnauthorized)),
		Entry("possibly rolling update ongoing (with autoscaler)",
			gcpShootThatNeedsAutoscaler,
			"gcp",
			[]*appsv1.Deployment{
				gardenerResourceManagerDeployment,
				kubeAPIServerDeployment,
				kubeControllerManagerDeployment,
				kubeSchedulerDeployment,
			},
			requiredControlPlaneEtcds,
			[]*extensionsv1alpha1.Worker{
				{Status: extensionsv1alpha1.WorkerStatus{DefaultStatus: extensionsv1alpha1.DefaultStatus{
					LastOperation: &gardencorev1beta1.LastOperation{
						State: gardencorev1beta1.LastOperationStateProcessing}}}},
			},
			BeNil()),
	)

	DescribeTable("#CheckSystemComponents",
		func(deployments []*appsv1.Deployment, daemonSets []*appsv1.DaemonSet, conditionMatcher types.GomegaMatcher) {
			var (
				deploymentLister = constDeploymentLister(deployments)
				daemonSetLister  = constDaemonSetLister(daemonSets)
				checker          = botanist.NewHealthChecker(map[gardencorev1beta1.ConditionType]time.Duration{}, nil)
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
				coreDNSDeployment,
				vpnShootDeployment,
			},
			requiredSystemComponentDaemonSets,
			beConditionWithStatus(gardencorev1beta1.ConditionFalse)),
		Entry("missing required daemon set",
			requiredSystemComponentDeployments,
			[]*appsv1.DaemonSet{
				kubeProxyDaemonSet,
			},
			beConditionWithStatus(gardencorev1beta1.ConditionFalse)),
		Entry("required deployment not healthy",
			[]*appsv1.Deployment{
				newDeployment(coreDNSDeployment.Namespace, coreDNSDeployment.Name, roleOf(coreDNSDeployment), false),
				vpnShootDeployment,
				metricsServerDeployment,
			},
			requiredSystemComponentDaemonSets,
			beConditionWithStatus(gardencorev1beta1.ConditionFalse)),
		Entry("required daemon set not healthy",
			requiredSystemComponentDeployments,
			[]*appsv1.DaemonSet{
				newDaemonSet(kubeProxyDaemonSet.Namespace, kubeProxyDaemonSet.Name, roleOf(kubeProxyDaemonSet), false),
				kubeProxyDaemonSet,
			},
			beConditionWithStatus(gardencorev1beta1.ConditionFalse)),
	)

	var (
		workerPoolName1 = "cpu-worker-1"
		workerPoolName2 = "cpu-worker-2"
		nodeName        = "node1"
	)

	DescribeTable("#CheckClusterNodes",
		func(nodes []*corev1.Node, workerPools []gardencorev1beta1.Worker, conditionMatcher types.GomegaMatcher) {
			var (
				nodeLister = constNodeLister(nodes)
				checker    = botanist.NewHealthChecker(map[gardencorev1beta1.ConditionType]time.Duration{}, nil)
			)

			exitCondition, err := checker.CheckClusterNodes(workerPools, condition, nodeLister)
			Expect(err).NotTo(HaveOccurred())
			Expect(exitCondition).To(conditionMatcher)
		},
		Entry("all healthy",
			[]*corev1.Node{
				newNode(nodeName, true, labels.Set{"worker.gardener.cloud/pool": workerPoolName1}),
			},
			[]gardencorev1beta1.Worker{
				{
					Name:    workerPoolName1,
					Maximum: 10,
					Minimum: 1,
				},
			},
			BeNil()),
		Entry("node not healthy",
			[]*corev1.Node{
				newNode(nodeName, false, labels.Set{"worker.gardener.cloud/pool": workerPoolName1}),
			},
			[]gardencorev1beta1.Worker{
				{
					Name:    workerPoolName1,
					Maximum: 10,
					Minimum: 1,
				},
			},
			beConditionWithStatusAndMsg(gardencorev1beta1.ConditionFalse, "NodeUnhealthy", fmt.Sprintf("Node '%s' in worker group '%s' is unhealthy", nodeName, workerPoolName1))),
		Entry("node not healthy with error codes",
			[]*corev1.Node{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:   nodeName,
						Labels: labels.Set{"worker.gardener.cloud/pool": workerPoolName1},
					},
					Status: corev1.NodeStatus{
						Conditions: []corev1.NodeCondition{
							{
								Type:   corev1.NodeReady,
								Status: corev1.ConditionTrue,
							},
							{
								Type:   corev1.NodeDiskPressure,
								Status: corev1.ConditionTrue,
								Reason: "KubeletHasDiskPressure",
							},
						},
					},
				},
			},
			[]gardencorev1beta1.Worker{
				{
					Name:    workerPoolName1,
					Maximum: 10,
					Minimum: 1,
				},
			},
			beConditionWithStatusAndCodes(gardencorev1beta1.ConditionFalse, gardencorev1beta1.ErrorConfigurationProblem)),
		Entry("not enough nodes in worker pool",
			[]*corev1.Node{
				newNode(nodeName, true, labels.Set{"worker.gardener.cloud/pool": workerPoolName1}),
			},
			[]gardencorev1beta1.Worker{
				{
					Name:    workerPoolName1,
					Maximum: 10,
					Minimum: 1,
				},
				{
					Name:    workerPoolName2,
					Maximum: 2,
					Minimum: 1,
				},
			},
			beConditionWithStatusAndMsg(gardencorev1beta1.ConditionFalse, "MissingNodes", fmt.Sprintf("Not enough worker nodes registered in worker pool '%s' to meet minimum desired machine count. (%d/%d).", workerPoolName2, 0, 1))),
		Entry("not enough nodes in worker pool",
			[]*corev1.Node{
				newNode(nodeName, true, labels.Set{"worker.gardener.cloud/pool": workerPoolName1}),
			},
			[]gardencorev1beta1.Worker{
				{
					Name:    workerPoolName1,
					Maximum: 10,
					Minimum: 1,
				},
				{
					Name:    workerPoolName2,
					Maximum: 2,
					Minimum: 1,
				},
			},
			beConditionWithStatusAndMsg(gardencorev1beta1.ConditionFalse, "MissingNodes", fmt.Sprintf("Not enough worker nodes registered in worker pool '%s' to meet minimum desired machine count. (%d/%d).", workerPoolName2, 0, 1))),
	)

	DescribeTable("#CheckMonitoringSystemComponents",
		func(deployments []*appsv1.Deployment, daemonSets []*appsv1.DaemonSet, isTestingShoot bool, conditionMatcher types.GomegaMatcher) {
			var (
				deploymentLister = constDeploymentLister(deployments)
				daemonSetLister  = constDaemonSetLister(daemonSets)
				checker          = botanist.NewHealthChecker(map[gardencorev1beta1.ConditionType]time.Duration{}, nil)
			)

			exitCondition, err := checker.CheckMonitoringSystemComponents(shootNamespace, isTestingShoot, condition, deploymentLister, daemonSetLister)
			Expect(err).NotTo(HaveOccurred())
			Expect(exitCondition).To(conditionMatcher)
		},
		Entry("all healthy",
			requiredMonitoringSystemComponentDeployments,
			requiredMonitoringSystemComponentDaemonSets,
			false,
			BeNil()),
		Entry("required deployment missing",
			[]*appsv1.Deployment{},
			requiredMonitoringSystemComponentDaemonSets,
			false,
			beConditionWithStatus(gardencorev1beta1.ConditionFalse)),
		Entry("required daemon set missing",
			requiredMonitoringSystemComponentDeployments,
			[]*appsv1.DaemonSet{},
			false,
			beConditionWithStatus(gardencorev1beta1.ConditionFalse)),
		Entry("deployment unhealthy",
			[]*appsv1.Deployment{newDeployment(blackboxExporterDeployment.Namespace, blackboxExporterDeployment.Name, roleOf(blackboxExporterDeployment), false)},
			requiredMonitoringSystemComponentDaemonSets,
			false,
			beConditionWithStatus(gardencorev1beta1.ConditionFalse)),
		Entry("daemon set unhealthy",
			requiredMonitoringSystemComponentDeployments,
			[]*appsv1.DaemonSet{newDaemonSet(nodeExporterDaemonSet.Namespace, nodeExporterDaemonSet.Name, roleOf(nodeExporterDaemonSet), false)},
			false,
			beConditionWithStatus(gardencorev1beta1.ConditionFalse)),
		Entry("shoot purpose is testing, omit all checks",
			[]*appsv1.Deployment{},
			[]*appsv1.DaemonSet{},
			true,
			BeNil()),
	)

	DescribeTable("#CheckMonitoringControlPlane",
		func(deployments []*appsv1.Deployment, statefulSets []*appsv1.StatefulSet, isTestingShoot, wantsAlertmanager bool, conditionMatcher types.GomegaMatcher) {
			var (
				deploymentLister  = constDeploymentLister(deployments)
				statefulSetLister = constStatefulSetLister(statefulSets)
				checker           = botanist.NewHealthChecker(map[gardencorev1beta1.ConditionType]time.Duration{}, nil)
			)

			exitCondition, err := checker.CheckMonitoringControlPlane(seedNamespace, isTestingShoot, wantsAlertmanager, condition, deploymentLister, statefulSetLister)
			Expect(err).NotTo(HaveOccurred())
			Expect(exitCondition).To(conditionMatcher)
		},
		Entry("all healthy",
			requiredMonitoringControlPlaneDeployments,
			requiredMonitoringControlPlaneStatefulSets,
			false,
			true,
			BeNil()),
		Entry("required deployment set missing",
			[]*appsv1.Deployment{
				kubeStateMetricsSeedDeployment,
				kubeStateMetricsShootDeployment,
			},
			requiredMonitoringControlPlaneStatefulSets,
			false,
			true,
			beConditionWithStatus(gardencorev1beta1.ConditionFalse)),
		Entry("required stateful set set missing",
			requiredMonitoringControlPlaneDeployments,
			[]*appsv1.StatefulSet{
				prometheusStatefulSet,
			},
			false,
			true,
			beConditionWithStatus(gardencorev1beta1.ConditionFalse)),
		Entry("deployment unhealthy",
			[]*appsv1.Deployment{
				newDeployment(grafanaDeploymentOperators.Namespace, grafanaDeploymentOperators.Name, roleOf(grafanaDeploymentOperators), false),
				grafanaDeploymentUsers,
				kubeStateMetricsSeedDeployment,
				kubeStateMetricsShootDeployment,
			},
			requiredMonitoringControlPlaneStatefulSets,
			false,
			true,
			beConditionWithStatus(gardencorev1beta1.ConditionFalse)),
		Entry("stateful set unhealthy",
			requiredMonitoringControlPlaneDeployments,
			[]*appsv1.StatefulSet{
				newStatefulSet(alertManagerStatefulSet.Namespace, alertManagerStatefulSet.Name, roleOf(alertManagerStatefulSet), false),
				prometheusStatefulSet,
			},
			false,
			true,
			beConditionWithStatus(gardencorev1beta1.ConditionFalse)),
		Entry("shoot purpose is testing, omit all checks",
			[]*appsv1.Deployment{},
			[]*appsv1.StatefulSet{},
			true,
			true,
			BeNil()),
	)

	DescribeTable("#CheckOptionalAddonsSystemComponents",
		func(deployments []*appsv1.Deployment, daemonSets []*appsv1.DaemonSet, conditionMatcher types.GomegaMatcher) {
			var (
				deploymentLister = constDeploymentLister(deployments)
				daemonSetLister  = constDaemonSetLister(daemonSets)
				checker          = botanist.NewHealthChecker(map[gardencorev1beta1.ConditionType]time.Duration{}, nil)
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
			[]*appsv1.Deployment{newDeployment(shootNamespace, "addon", v1beta1constants.GardenRoleOptionalAddon, false)},
			nil,
			beConditionWithStatus(gardencorev1beta1.ConditionFalse)),
		Entry("deployment unhealthy",
			nil,
			[]*appsv1.DaemonSet{newDaemonSet(shootNamespace, "addon", v1beta1constants.GardenRoleOptionalAddon, false)},
			beConditionWithStatus(gardencorev1beta1.ConditionFalse)),
	)

	DescribeTable("#CheckLoggingControlPlane",
		func(deployments []*appsv1.Deployment, statefulSets []*appsv1.StatefulSet, isTestingShoot bool, conditionMatcher types.GomegaMatcher) {
			var (
				deploymentLister  = constDeploymentLister(deployments)
				statefulSetLister = constStatefulSetLister(statefulSets)
				checker           = botanist.NewHealthChecker(map[gardencorev1beta1.ConditionType]time.Duration{}, nil)
			)

			exitCondition, err := checker.CheckLoggingControlPlane(seedNamespace, isTestingShoot, condition, deploymentLister, statefulSetLister)
			Expect(err).NotTo(HaveOccurred())
			Expect(exitCondition).To(conditionMatcher)
		},
		Entry("all healthy",
			requiredLoggingControlPlaneDeployments,
			requiredLoggingControlPlaneStatefulSets,
			false,
			BeNil()),
		Entry("required deployment missing",
			nil,
			requiredLoggingControlPlaneStatefulSets,
			false,
			beConditionWithStatus(gardencorev1beta1.ConditionFalse)),
		Entry("required stateful set missing",
			requiredLoggingControlPlaneDeployments,
			nil,
			false,
			beConditionWithStatus(gardencorev1beta1.ConditionFalse)),
		Entry("deployment unhealthy",
			[]*appsv1.Deployment{newDeployment(kibanaDeployment.Namespace, kibanaDeployment.Name, roleOf(kibanaDeployment), false)},
			requiredLoggingControlPlaneStatefulSets,
			false,
			beConditionWithStatus(gardencorev1beta1.ConditionFalse)),
		Entry("stateful set unhealthy",
			requiredLoggingControlPlaneDeployments,
			[]*appsv1.StatefulSet{
				newStatefulSet(elasticSearchStatefulSet.Namespace, elasticSearchStatefulSet.Name, roleOf(elasticSearchStatefulSet), false),
			},
			false,
			beConditionWithStatus(gardencorev1beta1.ConditionFalse)),
		Entry("shoot purpose is testing, omit all checks",
			[]*appsv1.Deployment{},
			[]*appsv1.StatefulSet{},
			true,
			BeNil()),
	)

	DescribeTable("#FailedCondition",
		func(thresholds map[gardencorev1beta1.ConditionType]time.Duration, transitionTime metav1.Time, now time.Time, condition gardencorev1beta1.Condition, expected types.GomegaMatcher) {
			checker := botanist.NewHealthChecker(thresholds, nil)
			tmp1, tmp2 := botanist.Now, gardencorev1beta1helper.Now
			defer func() {
				botanist.Now, gardencorev1beta1helper.Now = tmp1, tmp2
			}()
			botanist.Now, gardencorev1beta1helper.Now = func() time.Time {
				return now
			}, func() metav1.Time {
				return transitionTime
			}

			Expect(checker.FailedCondition(condition, "", "")).To(expected)
		},
		Entry("true condition with threshold",
			map[gardencorev1beta1.ConditionType]time.Duration{
				gardencorev1beta1.ShootControlPlaneHealthy: time.Minute,
			},
			zeroMetaTime,
			zeroTime,
			gardencorev1beta1.Condition{
				Type:   gardencorev1beta1.ShootControlPlaneHealthy,
				Status: gardencorev1beta1.ConditionTrue,
			},
			MatchFields(IgnoreExtras, Fields{
				"Status": Equal(gardencorev1beta1.ConditionProgressing),
			})),
		Entry("true condition without threshold",
			map[gardencorev1beta1.ConditionType]time.Duration{},
			zeroMetaTime,
			zeroTime,
			gardencorev1beta1.Condition{
				Type:   gardencorev1beta1.ShootControlPlaneHealthy,
				Status: gardencorev1beta1.ConditionTrue,
			},
			MatchFields(IgnoreExtras, Fields{
				"Status": Equal(gardencorev1beta1.ConditionFalse),
			})),
		Entry("progressing condition within threshold",
			map[gardencorev1beta1.ConditionType]time.Duration{
				gardencorev1beta1.ShootControlPlaneHealthy: time.Minute,
			},
			zeroMetaTime,
			zeroTime,
			gardencorev1beta1.Condition{
				Type:   gardencorev1beta1.ShootControlPlaneHealthy,
				Status: gardencorev1beta1.ConditionProgressing,
			},
			MatchFields(IgnoreExtras, Fields{
				"Status": Equal(gardencorev1beta1.ConditionProgressing),
			})),
		Entry("progressing condition outside threshold",
			map[gardencorev1beta1.ConditionType]time.Duration{
				gardencorev1beta1.ShootControlPlaneHealthy: time.Minute,
			},
			zeroMetaTime,
			zeroTime.Add(time.Minute+time.Second),
			gardencorev1beta1.Condition{
				Type:   gardencorev1beta1.ShootControlPlaneHealthy,
				Status: gardencorev1beta1.ConditionProgressing,
			},
			MatchFields(IgnoreExtras, Fields{
				"Status": Equal(gardencorev1beta1.ConditionFalse),
			})),
		Entry("failed condition",
			map[gardencorev1beta1.ConditionType]time.Duration{},
			zeroMetaTime,
			zeroTime,
			gardencorev1beta1.Condition{
				Type:   gardencorev1beta1.ShootControlPlaneHealthy,
				Status: gardencorev1beta1.ConditionFalse,
			},
			MatchFields(IgnoreExtras, Fields{
				"Status": Equal(gardencorev1beta1.ConditionFalse),
			})),
	)

	// CheckExtensionCondition
	DescribeTable("#CheckExtensionCondition - HealthCheckReport",
		func(healthCheckOutdatedThreshold *metav1.Duration, condition gardencorev1beta1.Condition, extensionsConditions []botanist.ExtensionCondition, expected types.GomegaMatcher) {
			checker := botanist.NewHealthChecker(nil, healthCheckOutdatedThreshold)
			updatedCondition := checker.CheckExtensionCondition(condition, extensionsConditions)
			if expected == nil {
				Expect(updatedCondition).To(BeNil())
				return
			}
			Expect(*updatedCondition).To(expected)
		},

		Entry("health check report is not outdated - threshold not configured in Gardenlet config",
			nil,
			gardencorev1beta1.Condition{},
			[]botanist.ExtensionCondition{
				{
					Condition: gardencorev1beta1.Condition{
						Type:           gardencorev1beta1.ShootControlPlaneHealthy,
						Status:         gardencorev1beta1.ConditionTrue,
						LastUpdateTime: metav1.Time{Time: time.Now().Add(time.Second * -30)},
					},
				},
			},
			nil,
		),
		Entry("health check report is not outdated",
			// 2 minute threshold for outdated health check reports
			&metav1.Duration{Duration: time.Minute * 2},
			gardencorev1beta1.Condition{},
			[]botanist.ExtensionCondition{
				{
					Condition: gardencorev1beta1.Condition{
						Type:   gardencorev1beta1.ShootControlPlaneHealthy,
						Status: gardencorev1beta1.ConditionTrue,
						// health check result is only 30 seconds old so < than the staleExtensionHealthCheckThreshold
						LastUpdateTime: metav1.Time{Time: time.Now().Add(time.Second * -30)},
					},
				},
			},
			nil,
		),
		Entry("should determine that health check report is outdated",
			// 2 minute threshold for outdated health check reports
			&metav1.Duration{Duration: time.Minute * 2},
			gardencorev1beta1.Condition{
				Type:   gardencorev1beta1.ShootControlPlaneHealthy,
				Status: gardencorev1beta1.ConditionTrue,
			},
			[]botanist.ExtensionCondition{
				{
					Condition: gardencorev1beta1.Condition{
						Type:   gardencorev1beta1.ShootControlPlaneHealthy,
						Status: gardencorev1beta1.ConditionTrue,
						// health check result is already 3 minutes old
						LastUpdateTime: metav1.Time{Time: time.Now().Add(time.Minute * -3)},
					},
					ExtensionType:      "Worker",
					ExtensionName:      "worker-ubuntu",
					ExtensionNamespace: "shoot-namespace-in-seed",
				},
			},
			MatchFields(IgnoreExtras, Fields{
				"Status": Equal(gardencorev1beta1.ConditionUnknown),
			}),
		),
		Entry("health check reports status progressing",
			nil,
			gardencorev1beta1.Condition{},
			[]botanist.ExtensionCondition{
				{
					ExtensionType: "Foo",
					Condition: gardencorev1beta1.Condition{
						Type:           gardencorev1beta1.ShootControlPlaneHealthy,
						Status:         gardencorev1beta1.ConditionProgressing,
						Reason:         "Bar",
						Message:        "Baz",
						LastUpdateTime: metav1.Time{Time: time.Now()},
					},
				},
			},
			MatchFields(IgnoreExtras, Fields{
				"Status":  Equal(gardencorev1beta1.ConditionProgressing),
				"Reason":  Equal("FooBar"),
				"Message": Equal("Baz"),
			}),
		),
		Entry("health check reports status false",
			nil,
			gardencorev1beta1.Condition{},
			[]botanist.ExtensionCondition{
				{
					ExtensionType: "Foo",
					Condition: gardencorev1beta1.Condition{
						Type:           gardencorev1beta1.ShootControlPlaneHealthy,
						Status:         gardencorev1beta1.ConditionFalse,
						LastUpdateTime: metav1.Time{Time: time.Now()},
					},
				},
			},
			MatchFields(IgnoreExtras, Fields{
				"Status":  Equal(gardencorev1beta1.ConditionFalse),
				"Reason":  Equal("FooUnhealthyReport"),
				"Message": ContainSubstring("failing health check"),
			}),
		),
		Entry("health check reports status unknown",
			nil,
			gardencorev1beta1.Condition{},
			[]botanist.ExtensionCondition{
				{
					ExtensionType: "Foo",
					Condition: gardencorev1beta1.Condition{
						Type:           gardencorev1beta1.ShootControlPlaneHealthy,
						Status:         gardencorev1beta1.ConditionUnknown,
						LastUpdateTime: metav1.Time{Time: time.Now()},
					},
				},
			},
			MatchFields(IgnoreExtras, Fields{
				"Status":  Equal(gardencorev1beta1.ConditionFalse),
				"Reason":  Equal("FooUnhealthyReport"),
				"Message": ContainSubstring("failing health check"),
			}),
		),
	)

	DescribeTable("#PardonCondition",
		func(lastOp *gardencorev1beta1.LastOperation, condition gardencorev1beta1.Condition, expected types.GomegaMatcher) {
			updatedCondition := botanist.PardonCondition(lastOp, condition)
			Expect(&updatedCondition).To(expected)
		},
		Entry("should pardon false ConditionStatus when the last operation is nil",
			nil,
			gardencorev1beta1.Condition{
				Type:   gardencorev1beta1.ShootAPIServerAvailable,
				Status: gardencorev1beta1.ConditionFalse,
			},
			beConditionWithStatus(gardencorev1beta1.ConditionProgressing)),
		Entry("should pardon false ConditionStatus when the last operation is create processing",
			&gardencorev1beta1.LastOperation{
				Type:  gardencorev1beta1.LastOperationTypeCreate,
				State: gardencorev1beta1.LastOperationStateProcessing,
			},
			gardencorev1beta1.Condition{
				Type:   gardencorev1beta1.ShootAPIServerAvailable,
				Status: gardencorev1beta1.ConditionFalse,
			},
			beConditionWithStatus(gardencorev1beta1.ConditionProgressing)),
		Entry("should pardon false ConditionStatus when the last operation is delete processing",
			&gardencorev1beta1.LastOperation{
				Type:  gardencorev1beta1.LastOperationTypeDelete,
				State: gardencorev1beta1.LastOperationStateProcessing,
			},
			gardencorev1beta1.Condition{
				Type:   gardencorev1beta1.ShootAPIServerAvailable,
				Status: gardencorev1beta1.ConditionFalse,
			},
			beConditionWithStatus(gardencorev1beta1.ConditionProgressing)),
		Entry("should not pardon false ConditionStatus when the last operation is create succeeded",
			&gardencorev1beta1.LastOperation{
				Type:  gardencorev1beta1.LastOperationTypeCreate,
				State: gardencorev1beta1.LastOperationStateSucceeded,
			},
			gardencorev1beta1.Condition{
				Type:   gardencorev1beta1.ShootAPIServerAvailable,
				Status: gardencorev1beta1.ConditionFalse,
			},
			beConditionWithStatus(gardencorev1beta1.ConditionFalse)),
	)
})

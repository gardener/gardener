// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package care_test

import (
	"context"
	"fmt"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/operatingsystemconfig/downloader"
	"github.com/gardener/gardener/pkg/operation/botanist/component/extensions/operatingsystemconfig/executor"
	"github.com/gardener/gardener/pkg/operation/care"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"github.com/Masterminds/semver"
	druidv1alpha1 "github.com/gardener/etcd-druid/api/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/onsi/gomega/types"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	zeroTime     time.Time
	zeroMetaTime metav1.Time
)

func roleOf(obj metav1.Object) string {
	return obj.GetLabels()[v1beta1constants.GardenRole]
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
	return map[string]string{v1beta1constants.GardenRole: role}
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
		etcd.Status.Ready = pointer.Bool(true)
	} else {
		etcd.Status.Ready = pointer.Bool(false)
		etcd.Status.LastError = lastError
	}

	return etcd
}

func newNode(name string, healthy bool, labels labels.Set, annotations map[string]string, kubeletVersion string) corev1.Node {
	node := corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Labels:      labels,
			Annotations: annotations,
		},
		Status: corev1.NodeStatus{
			NodeInfo: corev1.NodeSystemInfo{
				KubeletVersion: kubeletVersion,
			},
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
	return MatchFields(IgnoreExtras, Fields{
		"Status": Equal(status),
	})
}

func beConditionWithMissingRequiredDeployment(deployments []*appsv1.Deployment) types.GomegaMatcher {
	var names = make([]string, 0, len(deployments))
	for _, deploy := range deployments {
		names = append(names, deploy.Name)
	}
	return MatchFields(IgnoreExtras, Fields{
		"Status":  Equal(gardencorev1beta1.ConditionFalse),
		"Message": ContainSubstring("%s", names),
	})
}

func beConditionWithStatusAndCodes(status gardencorev1beta1.ConditionStatus, codes ...gardencorev1beta1.ErrorCode) types.GomegaMatcher {
	return MatchFields(IgnoreExtras, Fields{
		"Status": Equal(status),
		"Codes":  Equal(codes),
	})
}

func beConditionWithStatusAndMsg(status gardencorev1beta1.ConditionStatus, reason, message string) types.GomegaMatcher {
	return MatchFields(IgnoreExtras, Fields{
		"Status":  Equal(status),
		"Reason":  Equal(reason),
		"Message": ContainSubstring(message),
	})
}

var _ = Describe("health check", func() {
	var (
		condition = gardencorev1beta1.Condition{
			Type: gardencorev1beta1.ConditionType("test"),
		}
		shoot                    = &gardencorev1beta1.Shoot{}
		shootThatNeedsAutoscaler = &gardencorev1beta1.Shoot{
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

		shootWantsVPA = &gardencorev1beta1.Shoot{
			Spec: gardencorev1beta1.ShootSpec{
				Kubernetes: gardencorev1beta1.Kubernetes{
					VerticalPodAutoscaler: &gardencorev1beta1.VerticalPodAutoscaler{
						Enabled: true,
					},
				},
			},
		}

		seedNamespace     = "shoot--foo--bar"
		kubernetesVersion = semver.MustParse("1.19.3")
		gardenerVersion   = semver.MustParse("1.30.0")

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

		withVpaDeployments = func(deploys ...*appsv1.Deployment) []*appsv1.Deployment {
			var deployments = make([]*appsv1.Deployment, 0, len(deploys))
			deployments = append(deployments, deploys...)
			for _, deploymentName := range v1beta1constants.GetShootVPADeploymentNames() {
				deployments = append(deployments, newDeployment(seedNamespace, deploymentName, v1beta1constants.GardenRoleControlPlane, true))
			}
			return deployments
		}

		// control plane etcds
		etcdMain   = newEtcd(seedNamespace, v1beta1constants.ETCDMain, v1beta1constants.GardenRoleControlPlane, true, nil)
		etcdEvents = newEtcd(seedNamespace, v1beta1constants.ETCDEvents, v1beta1constants.GardenRoleControlPlane, true, nil)

		requiredControlPlaneEtcds = []*druidv1alpha1.Etcd{
			etcdMain,
			etcdEvents,
		}

		grafanaDeploymentOperators      = newDeployment(seedNamespace, v1beta1constants.DeploymentNameGrafanaOperators, v1beta1constants.GardenRoleMonitoring, true)
		grafanaDeploymentUsers          = newDeployment(seedNamespace, v1beta1constants.DeploymentNameGrafanaUsers, v1beta1constants.GardenRoleMonitoring, true)
		kubeStateMetricsShootDeployment = newDeployment(seedNamespace, v1beta1constants.DeploymentNameKubeStateMetrics, v1beta1constants.GardenRoleMonitoring, true)

		requiredMonitoringControlPlaneDeployments = []*appsv1.Deployment{
			grafanaDeploymentOperators,
			grafanaDeploymentUsers,
			kubeStateMetricsShootDeployment,
		}

		alertManagerStatefulSet = newStatefulSet(seedNamespace, v1beta1constants.StatefulSetNameAlertManager, v1beta1constants.GardenRoleMonitoring, true)
		prometheusStatefulSet   = newStatefulSet(seedNamespace, v1beta1constants.StatefulSetNamePrometheus, v1beta1constants.GardenRoleMonitoring, true)

		requiredMonitoringControlPlaneStatefulSets = []*appsv1.StatefulSet{
			alertManagerStatefulSet,
			prometheusStatefulSet,
		}

		lokiStatefulSet = newStatefulSet(seedNamespace, v1beta1constants.StatefulSetNameLoki, v1beta1constants.GardenRoleLogging, true)

		requiredLoggingControlPlaneStatefulSets = []*appsv1.StatefulSet{
			lokiStatefulSet,
		}
	)

	DescribeTable("#CheckControlPlane",
		func(shoot *gardencorev1beta1.Shoot, deployments []*appsv1.Deployment, etcds []*druidv1alpha1.Etcd, workers []*extensionsv1alpha1.Worker, conditionMatcher types.GomegaMatcher) {
			var (
				deploymentLister = constDeploymentLister(deployments)
				etcdLister       = constEtcdLister(etcds)
				workerLister     = constWorkerLister(workers)
				checker          = care.NewHealthChecker(map[gardencorev1beta1.ConditionType]time.Duration{}, nil, nil, kubernetesVersion, gardenerVersion)
			)

			exitCondition, err := checker.CheckControlPlane(shoot, seedNamespace, condition, deploymentLister, etcdLister, workerLister)
			Expect(err).NotTo(HaveOccurred())
			Expect(exitCondition).To(conditionMatcher)
		},
		Entry("all healthy",
			shoot,
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
			shootThatNeedsAutoscaler,
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
		Entry("all healthy (needs VPA)",
			shootWantsVPA,
			withVpaDeployments(
				gardenerResourceManagerDeployment,
				kubeAPIServerDeployment,
				kubeControllerManagerDeployment,
				kubeSchedulerDeployment,
			),
			requiredControlPlaneEtcds,
			[]*extensionsv1alpha1.Worker{
				{Status: extensionsv1alpha1.WorkerStatus{DefaultStatus: extensionsv1alpha1.DefaultStatus{
					LastOperation: &gardencorev1beta1.LastOperation{
						State: gardencorev1beta1.LastOperationStateSucceeded}}}},
			},
			BeNil()),
		Entry("missing required deployments",
			shootWantsVPA,
			[]*appsv1.Deployment{
				kubeAPIServerDeployment,
				kubeControllerManagerDeployment,
				kubeSchedulerDeployment,
			},
			requiredControlPlaneEtcds,
			nil,
			PointTo(beConditionWithMissingRequiredDeployment(withVpaDeployments(gardenerResourceManagerDeployment)))),
		Entry("required deployment unhealthy",
			shoot,
			[]*appsv1.Deployment{
				newDeployment(gardenerResourceManagerDeployment.Namespace, gardenerResourceManagerDeployment.Name, roleOf(gardenerResourceManagerDeployment), false),
				kubeAPIServerDeployment,
				kubeControllerManagerDeployment,
				kubeSchedulerDeployment,
			},
			requiredControlPlaneEtcds,
			nil,
			PointTo(beConditionWithStatus(gardencorev1beta1.ConditionFalse))),
		Entry("missing required etcd",
			shoot,
			requiredControlPlaneDeployments,
			[]*druidv1alpha1.Etcd{
				etcdEvents,
			},
			nil,
			PointTo(beConditionWithStatus(gardencorev1beta1.ConditionFalse))),
		Entry("required etcd unready",
			shoot,
			requiredControlPlaneDeployments,
			[]*druidv1alpha1.Etcd{
				newEtcd(etcdMain.Namespace, etcdMain.Name, roleOf(etcdMain), false, nil),
				etcdEvents,
			},
			nil,
			PointTo(beConditionWithStatus(gardencorev1beta1.ConditionFalse))),
		Entry("required etcd unhealthy with error code message",
			shoot,
			requiredControlPlaneDeployments,
			[]*druidv1alpha1.Etcd{
				newEtcd(etcdMain.Namespace, etcdMain.Name, roleOf(etcdMain), false, pointer.String("some error that maps to an error code, e.g. unauthorized")),
				etcdEvents,
			},
			nil,
			PointTo(beConditionWithStatusAndCodes(gardencorev1beta1.ConditionFalse, gardencorev1beta1.ErrorInfraUnauthorized))),
		Entry("possibly rolling update ongoing (with autoscaler)",
			shootThatNeedsAutoscaler,
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

	DescribeTable("#CheckManagedResource",
		func(conditions []gardencorev1beta1.Condition, upToDate bool, conditionMatcher types.GomegaMatcher) {
			var (
				mr      = new(resourcesv1alpha1.ManagedResource)
				checker = care.NewHealthChecker(map[gardencorev1beta1.ConditionType]time.Duration{}, nil, nil, kubernetesVersion, gardenerVersion)
			)

			if !upToDate {
				mr.Generation++
			}

			mr.Status.Conditions = conditions

			exitCondition := checker.CheckManagedResource(condition, mr)
			Expect(exitCondition).To(conditionMatcher)
		},
		Entry("no conditions",
			nil,
			true,
			PointTo(beConditionWithStatusAndMsg(gardencorev1beta1.ConditionFalse, gardencorev1beta1.ManagedResourceMissingConditionError, ""))),
		Entry("one true condition, one missing",
			[]gardencorev1beta1.Condition{
				{
					Type:   resourcesv1alpha1.ResourcesApplied,
					Status: gardencorev1beta1.ConditionTrue,
				},
			},
			true,
			PointTo(beConditionWithStatusAndMsg(gardencorev1beta1.ConditionFalse, gardencorev1beta1.ManagedResourceMissingConditionError, string(resourcesv1alpha1.ResourcesHealthy)))),
		Entry("multiple true conditions",
			[]gardencorev1beta1.Condition{
				{
					Status: gardencorev1beta1.ConditionTrue,
				},
				{
					Type:   resourcesv1alpha1.ResourcesHealthy,
					Status: gardencorev1beta1.ConditionTrue,
				},
				{
					Type:   resourcesv1alpha1.ResourcesApplied,
					Status: gardencorev1beta1.ConditionTrue,
				},
			},
			true,
			BeNil()),
		Entry("one false condition ResourcesApplied",
			[]gardencorev1beta1.Condition{
				{
					Type:   resourcesv1alpha1.ResourcesApplied,
					Status: gardencorev1beta1.ConditionFalse,
				},
				{
					Type:   resourcesv1alpha1.ResourcesHealthy,
					Status: gardencorev1beta1.ConditionTrue,
				},
			},
			true,
			PointTo(beConditionWithStatus(gardencorev1beta1.ConditionFalse))),
		Entry("one false condition ResourcesHealthy",
			[]gardencorev1beta1.Condition{
				{
					Type:   resourcesv1alpha1.ResourcesApplied,
					Status: gardencorev1beta1.ConditionTrue,
				},
				{
					Type:   resourcesv1alpha1.ResourcesHealthy,
					Status: gardencorev1beta1.ConditionFalse,
				},
			},
			true,
			PointTo(beConditionWithStatus(gardencorev1beta1.ConditionFalse))),
		Entry("multiple false conditions with reason & message",
			[]gardencorev1beta1.Condition{
				{
					Type:    resourcesv1alpha1.ResourcesApplied,
					Status:  gardencorev1beta1.ConditionFalse,
					Reason:  "fooFailed",
					Message: "foo is unhealthy",
				},
				{
					Type:    resourcesv1alpha1.ResourcesHealthy,
					Status:  gardencorev1beta1.ConditionFalse,
					Reason:  "barFailed",
					Message: "bar is unhealthy",
				},
			},
			true,
			PointTo(beConditionWithStatusAndMsg(gardencorev1beta1.ConditionFalse, "fooFailed", "foo is unhealthy"))),
		Entry("outdated managed resource",
			[]gardencorev1beta1.Condition{
				{
					Type:    resourcesv1alpha1.ResourcesApplied,
					Status:  gardencorev1beta1.ConditionFalse,
					Reason:  "fooFailed",
					Message: "foo is unhealthy",
				},
				{
					Type:    resourcesv1alpha1.ResourcesHealthy,
					Status:  gardencorev1beta1.ConditionFalse,
					Reason:  "barFailed",
					Message: "bar is unhealthy",
				},
			},
			false,
			PointTo(beConditionWithStatusAndMsg(gardencorev1beta1.ConditionFalse, gardencorev1beta1.OutdatedStatusError, "outdated"))),
		Entry("unknown condition status with reason and message",
			[]gardencorev1beta1.Condition{
				{
					Type:   resourcesv1alpha1.ResourcesApplied,
					Status: gardencorev1beta1.ConditionTrue,
				},
				{
					Type:    resourcesv1alpha1.ResourcesHealthy,
					Status:  gardencorev1beta1.ConditionUnknown,
					Reason:  "Unknown",
					Message: "bar is unknown",
				},
			},
			true,
			PointTo(beConditionWithStatusAndMsg(gardencorev1beta1.ConditionFalse, "Unknown", "bar is unknown"))),
	)

	Describe("#CheckClusterNodes", func() {
		var (
			ctrl *gomock.Controller
			c    *mockclient.MockClient

			ctx                        = context.TODO()
			workerPoolName1            = "cpu-worker-1"
			workerPoolName2            = "cpu-worker-2"
			cloudConfigSecretChecksum1 = "foo"
			cloudConfigSecretChecksum2 = "foo"
			nodeName                   = "node1"
			cloudConfigSecretChecksums = map[string]string{
				workerPoolName1: cloudConfigSecretChecksum1,
				workerPoolName2: cloudConfigSecretChecksum2,
			}
		)

		BeforeEach(func() {
			ctrl = gomock.NewController(GinkgoT())
			c = mockclient.NewMockClient(ctrl)
		})

		AfterEach(func() {
			ctrl.Finish()
		})

		DescribeTable("#CheckClusterNodes",
			func(nodes []corev1.Node, workerPools []gardencorev1beta1.Worker, cloudConfigSecretChecksums map[string]string, conditionMatcher types.GomegaMatcher) {
				c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&corev1.NodeList{})).DoAndReturn(func(_ context.Context, list *corev1.NodeList, _ ...client.ListOption) error {
					*list = corev1.NodeList{Items: nodes}
					return nil
				})
				c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&corev1.SecretList{}), gomock.AssignableToTypeOf(client.MatchingLabels{})).DoAndReturn(func(_ context.Context, list *corev1.SecretList, _ ...client.ListOption) error {
					*list = corev1.SecretList{}
					for pool, checksum := range cloudConfigSecretChecksums {
						list.Items = append(list.Items, corev1.Secret{
							ObjectMeta: metav1.ObjectMeta{
								Labels:      map[string]string{v1beta1constants.LabelWorkerPool: pool},
								Annotations: map[string]string{downloader.AnnotationKeyChecksum: checksum},
							},
						})
					}
					return nil
				})

				checker := care.NewHealthChecker(map[gardencorev1beta1.ConditionType]time.Duration{}, nil, nil, kubernetesVersion, gardenerVersion)

				exitCondition, err := checker.CheckClusterNodes(ctx, c, workerPools, condition)
				Expect(err).NotTo(HaveOccurred())
				Expect(exitCondition).To(conditionMatcher)
			},
			Entry("all healthy",
				[]corev1.Node{
					newNode(nodeName, true, labels.Set{"worker.gardener.cloud/pool": workerPoolName1}, nil, kubernetesVersion.Original()),
				},
				[]gardencorev1beta1.Worker{
					{
						Name:    workerPoolName1,
						Maximum: 10,
						Minimum: 1,
					},
				},
				cloudConfigSecretChecksums,
				BeNil()),
			Entry("node not healthy",
				[]corev1.Node{
					newNode(nodeName, false, labels.Set{"worker.gardener.cloud/pool": workerPoolName1}, nil, kubernetesVersion.Original()),
				},
				[]gardencorev1beta1.Worker{
					{
						Name:    workerPoolName1,
						Maximum: 10,
						Minimum: 1,
					},
				},
				cloudConfigSecretChecksums,
				PointTo(beConditionWithStatusAndMsg(gardencorev1beta1.ConditionFalse, "NodeUnhealthy", fmt.Sprintf("Node %q in worker group %q is unhealthy", nodeName, workerPoolName1)))),
			Entry("node not healthy with error codes",
				[]corev1.Node{
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
				cloudConfigSecretChecksums,
				PointTo(beConditionWithStatusAndCodes(gardencorev1beta1.ConditionFalse, gardencorev1beta1.ErrorConfigurationProblem))),
			Entry("not enough nodes in worker pool",
				[]corev1.Node{
					newNode(nodeName, true, labels.Set{"worker.gardener.cloud/pool": workerPoolName1}, nil, kubernetesVersion.Original()),
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
				cloudConfigSecretChecksums,
				PointTo(beConditionWithStatusAndMsg(gardencorev1beta1.ConditionFalse, "MissingNodes", fmt.Sprintf("Not enough worker nodes registered in worker pool %q to meet minimum desired machine count. (%d/%d).", workerPoolName2, 0, 1)))),
			Entry("not enough nodes in worker pool",
				[]corev1.Node{
					newNode(nodeName, true, labels.Set{"worker.gardener.cloud/pool": workerPoolName1}, nil, kubernetesVersion.Original()),
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
				cloudConfigSecretChecksums,
				PointTo(beConditionWithStatusAndMsg(gardencorev1beta1.ConditionFalse, "MissingNodes", fmt.Sprintf("Not enough worker nodes registered in worker pool %q to meet minimum desired machine count. (%d/%d).", workerPoolName2, 0, 1)))),
			Entry("too old Kubernetes patch version",
				[]corev1.Node{
					newNode(nodeName, true, labels.Set{"worker.gardener.cloud/pool": workerPoolName1}, nil, "v1.19.2"),
				},
				[]gardencorev1beta1.Worker{
					{
						Name:    workerPoolName1,
						Maximum: 10,
						Minimum: 1,
					},
				},
				cloudConfigSecretChecksums,
				PointTo(beConditionWithStatusAndMsg(gardencorev1beta1.ConditionFalse, "KubeletVersionMismatch", fmt.Sprintf("The kubelet version for node %q (v1.19.2) does not match the desired Kubernetes version (v%s)", nodeName, kubernetesVersion.Original())))),
			Entry("same Kubernetes patch version",
				[]corev1.Node{
					newNode(nodeName, true, labels.Set{"worker.gardener.cloud/pool": workerPoolName1}, nil, "v1.19.3"),
				},
				[]gardencorev1beta1.Worker{
					{
						Name:    workerPoolName1,
						Maximum: 10,
						Minimum: 1,
					},
				},
				cloudConfigSecretChecksums,
				BeNil()),
			Entry("too old Kubernetes patch version with pool version overwrite",
				[]corev1.Node{
					newNode(nodeName, true, labels.Set{"worker.gardener.cloud/pool": workerPoolName1}, nil, "v1.18.2"),
				},
				[]gardencorev1beta1.Worker{
					{
						Name:    workerPoolName1,
						Maximum: 10,
						Minimum: 1,
						Kubernetes: &gardencorev1beta1.WorkerKubernetes{
							Version: pointer.String("1.18.3"),
						},
					},
				},
				cloudConfigSecretChecksums,
				PointTo(beConditionWithStatusAndMsg(gardencorev1beta1.ConditionFalse, "KubeletVersionMismatch", fmt.Sprintf("The kubelet version for node %q (v1.18.2) does not match the desired Kubernetes version (v1.18.3)", nodeName)))),
			Entry("different Kubernetes minor version (all healthy)",
				[]corev1.Node{
					newNode(nodeName, true, labels.Set{"worker.gardener.cloud/pool": workerPoolName1}, nil, "v1.18.2"),
				},
				[]gardencorev1beta1.Worker{
					{
						Name:    workerPoolName1,
						Maximum: 10,
						Minimum: 1,
					},
				},
				cloudConfigSecretChecksums,
				BeNil()),
			Entry("missing cloud-config secret checksum for a worker pool",
				[]corev1.Node{
					newNode(nodeName, true, labels.Set{"worker.gardener.cloud/pool": workerPoolName1}, nil, "v1.18.2"),
				},
				[]gardencorev1beta1.Worker{
					{
						Name:    workerPoolName1,
						Maximum: 10,
						Minimum: 1,
					},
				},
				nil,
				BeNil()),
			Entry("no cloud-config node checksum for a worker pool",
				[]corev1.Node{
					newNode(nodeName, true, labels.Set{"worker.gardener.cloud/pool": workerPoolName1}, nil, "v1.18.2"),
				},
				[]gardencorev1beta1.Worker{
					{
						Name:    workerPoolName1,
						Maximum: 10,
						Minimum: 1,
					},
				},
				cloudConfigSecretChecksums,
				BeNil()),
			Entry("outdated cloud-config secret checksum for a worker pool",
				[]corev1.Node{
					newNode(nodeName, true, labels.Set{"worker.gardener.cloud/pool": workerPoolName1}, map[string]string{executor.AnnotationKeyChecksum: "outdated"}, "v1.18.2"),
				},
				[]gardencorev1beta1.Worker{
					{
						Name:    workerPoolName1,
						Maximum: 10,
						Minimum: 1,
					},
				},
				map[string]string{
					workerPoolName1: cloudConfigSecretChecksum1,
				},
				PointTo(beConditionWithStatusAndMsg(gardencorev1beta1.ConditionFalse, "CloudConfigOutdated", fmt.Sprintf("the last successfully applied cloud config on node %q is outdated", nodeName)))),
		)
	})

	DescribeTable("#CheckMonitoringControlPlane",
		func(deployments []*appsv1.Deployment, statefulSets []*appsv1.StatefulSet, wantsShootMonitoring, wantsAlertmanager bool, conditionMatcher types.GomegaMatcher) {
			var (
				deploymentLister  = constDeploymentLister(deployments)
				statefulSetLister = constStatefulSetLister(statefulSets)
				checker           = care.NewHealthChecker(map[gardencorev1beta1.ConditionType]time.Duration{}, nil, nil, kubernetesVersion, gardenerVersion)
			)

			exitCondition, err := checker.CheckMonitoringControlPlane(seedNamespace, wantsShootMonitoring, wantsAlertmanager, condition, deploymentLister, statefulSetLister)
			Expect(err).NotTo(HaveOccurred())
			Expect(exitCondition).To(conditionMatcher)
		},
		Entry("all healthy",
			requiredMonitoringControlPlaneDeployments,
			requiredMonitoringControlPlaneStatefulSets,
			true,
			true,
			BeNil()),
		Entry("required deployment set missing",
			[]*appsv1.Deployment{
				kubeStateMetricsShootDeployment,
			},
			requiredMonitoringControlPlaneStatefulSets,
			true,
			true,
			PointTo(beConditionWithStatus(gardencorev1beta1.ConditionFalse))),
		Entry("required stateful set set missing",
			requiredMonitoringControlPlaneDeployments,
			[]*appsv1.StatefulSet{
				prometheusStatefulSet,
			},
			true,
			true,
			PointTo(beConditionWithStatus(gardencorev1beta1.ConditionFalse))),
		Entry("deployment unhealthy",
			[]*appsv1.Deployment{
				newDeployment(grafanaDeploymentOperators.Namespace, grafanaDeploymentOperators.Name, roleOf(grafanaDeploymentOperators), false),
				grafanaDeploymentUsers,
				kubeStateMetricsShootDeployment,
			},
			requiredMonitoringControlPlaneStatefulSets,
			true,
			true,
			PointTo(beConditionWithStatus(gardencorev1beta1.ConditionFalse))),
		Entry("stateful set unhealthy",
			requiredMonitoringControlPlaneDeployments,
			[]*appsv1.StatefulSet{
				newStatefulSet(alertManagerStatefulSet.Namespace, alertManagerStatefulSet.Name, roleOf(alertManagerStatefulSet), false),
				prometheusStatefulSet,
			},
			true,
			true,
			PointTo(beConditionWithStatus(gardencorev1beta1.ConditionFalse))),
		Entry("shoot has monitoring disabled, omit all checks",
			[]*appsv1.Deployment{},
			[]*appsv1.StatefulSet{},
			false,
			true,
			BeNil()),
	)

	DescribeTable("#CheckLoggingControlPlane",
		func(statefulSets []*appsv1.StatefulSet, isTestingShoot bool, lokiEnabled bool, conditionMatcher types.GomegaMatcher) {
			var (
				statefulSetLister = constStatefulSetLister(statefulSets)
				checker           = care.NewHealthChecker(map[gardencorev1beta1.ConditionType]time.Duration{}, nil, nil, kubernetesVersion, gardenerVersion)
			)

			exitCondition, err := checker.CheckLoggingControlPlane(seedNamespace, isTestingShoot, lokiEnabled, condition, statefulSetLister)
			Expect(err).NotTo(HaveOccurred())
			Expect(exitCondition).To(conditionMatcher)
		},
		Entry("all healthy",
			requiredLoggingControlPlaneStatefulSets,
			false,
			true,
			BeNil()),
		Entry("required stateful set missing",
			nil,
			false,
			true,
			PointTo(beConditionWithStatus(gardencorev1beta1.ConditionFalse))),
		Entry("stateful set unhealthy",
			[]*appsv1.StatefulSet{
				newStatefulSet(lokiStatefulSet.Namespace, lokiStatefulSet.Name, roleOf(lokiStatefulSet), false),
			},
			false,
			true,
			PointTo(beConditionWithStatus(gardencorev1beta1.ConditionFalse))),
		Entry("shoot purpose is testing, omit all checks",
			[]*appsv1.StatefulSet{},
			true,
			true,
			BeNil()),
		Entry("loki is disabled in gardenlet config, omit all checks",
			[]*appsv1.StatefulSet{},
			false,
			false,
			BeNil()),
	)

	DescribeTable("#FailedCondition",
		func(thresholds map[gardencorev1beta1.ConditionType]time.Duration, lastOperation *gardencorev1beta1.LastOperation, transitionTime metav1.Time, now time.Time, condition gardencorev1beta1.Condition, reason, message string, expected types.GomegaMatcher) {
			checker := care.NewHealthChecker(thresholds, nil, lastOperation, kubernetesVersion, gardenerVersion)
			tmp1, tmp2 := care.Now, gardencorev1beta1helper.Now
			defer func() {
				care.Now, gardencorev1beta1helper.Now = tmp1, tmp2
			}()
			care.Now, gardencorev1beta1helper.Now = func() time.Time {
				return now
			}, func() metav1.Time {
				return transitionTime
			}

			Expect(checker.FailedCondition(condition, reason, message)).To(expected)
		},
		Entry("true condition with threshold",
			map[gardencorev1beta1.ConditionType]time.Duration{
				gardencorev1beta1.ShootControlPlaneHealthy: time.Minute,
			},
			nil,
			zeroMetaTime,
			zeroTime,
			gardencorev1beta1.Condition{
				Type:   gardencorev1beta1.ShootControlPlaneHealthy,
				Status: gardencorev1beta1.ConditionTrue,
			},
			"",
			"",
			beConditionWithStatus(gardencorev1beta1.ConditionProgressing)),
		Entry("true condition without condition threshold",
			map[gardencorev1beta1.ConditionType]time.Duration{},
			nil,
			zeroMetaTime,
			zeroTime,
			gardencorev1beta1.Condition{
				Type:   gardencorev1beta1.ShootControlPlaneHealthy,
				Status: gardencorev1beta1.ConditionTrue,
			},
			"",
			"",
			beConditionWithStatus(gardencorev1beta1.ConditionFalse)),
		Entry("progressing condition within last operation update time threshold",
			map[gardencorev1beta1.ConditionType]time.Duration{
				gardencorev1beta1.ShootControlPlaneHealthy: time.Minute,
			},
			&gardencorev1beta1.LastOperation{
				State:          gardencorev1beta1.LastOperationStateSucceeded,
				LastUpdateTime: zeroMetaTime,
			},
			zeroMetaTime,
			zeroTime,
			gardencorev1beta1.Condition{
				Type:   gardencorev1beta1.ShootControlPlaneHealthy,
				Status: gardencorev1beta1.ConditionProgressing,
			},
			"",
			"",
			beConditionWithStatus(gardencorev1beta1.ConditionProgressing)),
		Entry("progressing condition outside last operation update time threshold but within last transition time threshold",
			map[gardencorev1beta1.ConditionType]time.Duration{
				gardencorev1beta1.ShootControlPlaneHealthy: time.Minute,
			},
			&gardencorev1beta1.LastOperation{
				State:          gardencorev1beta1.LastOperationStateSucceeded,
				LastUpdateTime: zeroMetaTime,
			},
			zeroMetaTime,
			zeroTime.Add(time.Minute+time.Second),
			gardencorev1beta1.Condition{
				Type:               gardencorev1beta1.ShootControlPlaneHealthy,
				Status:             gardencorev1beta1.ConditionProgressing,
				LastTransitionTime: metav1.Time{Time: zeroMetaTime.Add(time.Minute)},
			},
			"",
			"",
			beConditionWithStatus(gardencorev1beta1.ConditionProgressing)),
		Entry("progressing condition outside last operation update time threshold and last transition time threshold",
			map[gardencorev1beta1.ConditionType]time.Duration{
				gardencorev1beta1.ShootControlPlaneHealthy: time.Minute,
			},
			&gardencorev1beta1.LastOperation{
				State:          gardencorev1beta1.LastOperationStateSucceeded,
				LastUpdateTime: zeroMetaTime,
			},
			zeroMetaTime,
			zeroTime.Add(time.Minute+time.Second),
			gardencorev1beta1.Condition{
				Type:   gardencorev1beta1.ShootControlPlaneHealthy,
				Status: gardencorev1beta1.ConditionProgressing,
			},
			"",
			"",
			beConditionWithStatus(gardencorev1beta1.ConditionFalse)),
		Entry("failed condition within last operation update time threshold",
			map[gardencorev1beta1.ConditionType]time.Duration{
				gardencorev1beta1.ShootControlPlaneHealthy: time.Minute,
			},
			&gardencorev1beta1.LastOperation{
				State:          gardencorev1beta1.LastOperationStateSucceeded,
				LastUpdateTime: zeroMetaTime,
			},
			zeroMetaTime,
			zeroTime.Add(time.Minute-time.Second),
			gardencorev1beta1.Condition{
				Type:   gardencorev1beta1.ShootControlPlaneHealthy,
				Status: gardencorev1beta1.ConditionFalse,
			},
			"",
			"",
			beConditionWithStatus(gardencorev1beta1.ConditionProgressing)),
		Entry("failed condition outside of last operation update time threshold with same reason",
			map[gardencorev1beta1.ConditionType]time.Duration{
				gardencorev1beta1.ShootControlPlaneHealthy: time.Minute,
			},
			&gardencorev1beta1.LastOperation{
				State:          gardencorev1beta1.LastOperationStateSucceeded,
				LastUpdateTime: zeroMetaTime,
			},
			zeroMetaTime,
			zeroTime.Add(time.Minute+time.Second),
			gardencorev1beta1.Condition{
				Type:   gardencorev1beta1.ShootControlPlaneHealthy,
				Status: gardencorev1beta1.ConditionFalse,
				Reason: "Reason",
			},
			"Reason",
			"",
			beConditionWithStatus(gardencorev1beta1.ConditionFalse)),
		Entry("failed condition outside of last operation update time threshold with a different reason",
			map[gardencorev1beta1.ConditionType]time.Duration{
				gardencorev1beta1.ShootControlPlaneHealthy: time.Minute,
			},
			&gardencorev1beta1.LastOperation{
				State:          gardencorev1beta1.LastOperationStateSucceeded,
				LastUpdateTime: zeroMetaTime,
			},
			zeroMetaTime,
			zeroTime.Add(time.Minute+time.Second),
			gardencorev1beta1.Condition{
				Type:   gardencorev1beta1.ShootControlPlaneHealthy,
				Status: gardencorev1beta1.ConditionFalse,
				Reason: "foo",
			},
			"bar",
			"",
			beConditionWithStatus(gardencorev1beta1.ConditionProgressing)),
		Entry("failed condition outside of last operation update time threshold with a different message",
			map[gardencorev1beta1.ConditionType]time.Duration{
				gardencorev1beta1.ShootControlPlaneHealthy: time.Minute,
			},
			&gardencorev1beta1.LastOperation{
				State:          gardencorev1beta1.LastOperationStateSucceeded,
				LastUpdateTime: zeroMetaTime,
			},
			zeroMetaTime,
			zeroTime.Add(time.Minute+time.Second),
			gardencorev1beta1.Condition{
				Type:    gardencorev1beta1.ShootControlPlaneHealthy,
				Status:  gardencorev1beta1.ConditionFalse,
				Message: "foo",
			},
			"",
			"bar",
			beConditionWithStatus(gardencorev1beta1.ConditionFalse)),
		Entry("failed condition without thresholds",
			map[gardencorev1beta1.ConditionType]time.Duration{},
			nil,
			zeroMetaTime,
			zeroTime,
			gardencorev1beta1.Condition{
				Type:   gardencorev1beta1.ShootControlPlaneHealthy,
				Status: gardencorev1beta1.ConditionFalse,
			},
			"",
			"",
			beConditionWithStatus(gardencorev1beta1.ConditionFalse)),
	)

	// CheckExtensionCondition
	DescribeTable("#CheckExtensionCondition - HealthCheckReport",
		func(healthCheckOutdatedThreshold *metav1.Duration, condition gardencorev1beta1.Condition, extensionsConditions []care.ExtensionCondition, expected types.GomegaMatcher) {
			checker := care.NewHealthChecker(nil, healthCheckOutdatedThreshold, nil, kubernetesVersion, gardenerVersion)
			updatedCondition := checker.CheckExtensionCondition(condition, extensionsConditions)
			if expected == nil {
				Expect(updatedCondition).To(BeNil())
				return
			}
			Expect(updatedCondition).To(expected)
		},

		Entry("health check report is not outdated - threshold not configured in Gardenlet config",
			nil,
			gardencorev1beta1.Condition{Type: "type"},
			[]care.ExtensionCondition{
				{
					Condition: gardencorev1beta1.Condition{
						Type:           gardencorev1beta1.ShootControlPlaneHealthy,
						Status:         gardencorev1beta1.ConditionTrue,
						LastUpdateTime: metav1.Time{Time: time.Now().Add(time.Second * -30)},
					},
				},
			},
			BeNil(),
		),
		Entry("health check report is not outdated",
			// 2 minute threshold for outdated health check reports
			&metav1.Duration{Duration: time.Minute * 2},
			gardencorev1beta1.Condition{Type: "type"},
			[]care.ExtensionCondition{
				{
					Condition: gardencorev1beta1.Condition{
						Type:   gardencorev1beta1.ShootControlPlaneHealthy,
						Status: gardencorev1beta1.ConditionTrue,
						// health check result is only 30 seconds old so < than the staleExtensionHealthCheckThreshold
						LastUpdateTime: metav1.Time{Time: time.Now().Add(time.Second * -30)},
					},
				},
			},
			BeNil(),
		),
		Entry("should determine that health check report is outdated",
			// 2 minute threshold for outdated health check reports
			&metav1.Duration{Duration: time.Minute * 2},
			gardencorev1beta1.Condition{
				Type:   gardencorev1beta1.ShootControlPlaneHealthy,
				Status: gardencorev1beta1.ConditionTrue,
			},
			[]care.ExtensionCondition{
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
			PointTo(MatchFields(IgnoreExtras, Fields{
				"Status": Equal(gardencorev1beta1.ConditionUnknown),
			})),
		),
		Entry("health check reports status progressing",
			nil,
			gardencorev1beta1.Condition{Type: "type"},
			[]care.ExtensionCondition{
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
			PointTo(MatchFields(IgnoreExtras, Fields{
				"Status":  Equal(gardencorev1beta1.ConditionProgressing),
				"Reason":  Equal("FooBar"),
				"Message": Equal("Baz"),
			})),
		),
		Entry("health check reports status false",
			nil,
			gardencorev1beta1.Condition{Type: "type"},
			[]care.ExtensionCondition{
				{
					ExtensionType: "Foo",
					Condition: gardencorev1beta1.Condition{
						Type:           gardencorev1beta1.ShootControlPlaneHealthy,
						Status:         gardencorev1beta1.ConditionFalse,
						LastUpdateTime: metav1.Time{Time: time.Now()},
					},
				},
			},
			PointTo(MatchFields(IgnoreExtras, Fields{
				"Status":  Equal(gardencorev1beta1.ConditionFalse),
				"Reason":  Equal("FooUnhealthyReport"),
				"Message": ContainSubstring("failing health check"),
			})),
		),
		Entry("health check reports status unknown",
			nil,
			gardencorev1beta1.Condition{Type: "type"},
			[]care.ExtensionCondition{
				{
					ExtensionType: "Foo",
					Condition: gardencorev1beta1.Condition{
						Type:           gardencorev1beta1.ShootControlPlaneHealthy,
						Status:         gardencorev1beta1.ConditionUnknown,
						LastUpdateTime: metav1.Time{Time: time.Now()},
					},
				},
			},
			PointTo(MatchFields(IgnoreExtras, Fields{
				"Status":  Equal(gardencorev1beta1.ConditionFalse),
				"Reason":  Equal("FooUnhealthyReport"),
				"Message": ContainSubstring("failing health check"),
			})),
		),
	)

	DescribeTable("#PardonCondition",
		func(condition gardencorev1beta1.Condition, lastOp *gardencorev1beta1.LastOperation, lastErrors []gardencorev1beta1.LastError, expected types.GomegaMatcher) {
			conditions := []gardencorev1beta1.Condition{condition}
			updatedConditions := care.PardonConditions(conditions, lastOp, lastErrors)
			Expect(updatedConditions).To(expected)
		},
		Entry("should pardon false ConditionStatus when the last operation is nil",
			gardencorev1beta1.Condition{
				Type:   gardencorev1beta1.ShootAPIServerAvailable,
				Status: gardencorev1beta1.ConditionFalse,
			},
			nil,
			nil,
			ConsistOf(beConditionWithStatus(gardencorev1beta1.ConditionProgressing))),
		Entry("should pardon false ConditionStatus when the last operation is create processing",
			gardencorev1beta1.Condition{
				Type:   gardencorev1beta1.ShootAPIServerAvailable,
				Status: gardencorev1beta1.ConditionFalse,
			},
			&gardencorev1beta1.LastOperation{
				Type:  gardencorev1beta1.LastOperationTypeCreate,
				State: gardencorev1beta1.LastOperationStateProcessing,
			},
			nil,
			ConsistOf(beConditionWithStatus(gardencorev1beta1.ConditionProgressing))),
		Entry("should pardon false ConditionStatus when the last operation is delete processing",
			gardencorev1beta1.Condition{
				Type:   gardencorev1beta1.ShootAPIServerAvailable,
				Status: gardencorev1beta1.ConditionFalse,
			},
			&gardencorev1beta1.LastOperation{
				Type:  gardencorev1beta1.LastOperationTypeDelete,
				State: gardencorev1beta1.LastOperationStateProcessing,
			},
			nil,
			ConsistOf(beConditionWithStatus(gardencorev1beta1.ConditionProgressing))),
		Entry("should pardon false ConditionStatus when the last operation is processing and no last errors",
			gardencorev1beta1.Condition{
				Type:   gardencorev1beta1.ShootAPIServerAvailable,
				Status: gardencorev1beta1.ConditionFalse,
			},
			&gardencorev1beta1.LastOperation{
				Type:  gardencorev1beta1.LastOperationTypeReconcile,
				State: gardencorev1beta1.LastOperationStateProcessing,
			},
			nil,
			ConsistOf(beConditionWithStatus(gardencorev1beta1.ConditionProgressing))),
		Entry("should not pardon false ConditionStatus when the last operation is processing and last errors",
			gardencorev1beta1.Condition{
				Type:   gardencorev1beta1.ShootAPIServerAvailable,
				Status: gardencorev1beta1.ConditionFalse,
			},
			&gardencorev1beta1.LastOperation{
				Type:  gardencorev1beta1.LastOperationTypeReconcile,
				State: gardencorev1beta1.LastOperationStateProcessing,
			},
			[]gardencorev1beta1.LastError{
				{Description: "error"},
			},
			ConsistOf(beConditionWithStatus(gardencorev1beta1.ConditionFalse))),
		Entry("should not pardon false ConditionStatus when the last operation is create succeeded",
			gardencorev1beta1.Condition{
				Type:   gardencorev1beta1.ShootAPIServerAvailable,
				Status: gardencorev1beta1.ConditionFalse,
			},
			&gardencorev1beta1.LastOperation{
				Type:  gardencorev1beta1.LastOperationTypeCreate,
				State: gardencorev1beta1.LastOperationStateSucceeded,
			},
			nil,
			ConsistOf(beConditionWithStatus(gardencorev1beta1.ConditionFalse))),
	)
})

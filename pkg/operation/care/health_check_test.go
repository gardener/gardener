// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	"github.com/Masterminds/semver"
	druidv1alpha1 "github.com/gardener/etcd-druid/api/v1alpha1"
	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/onsi/gomega/types"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	testclock "k8s.io/utils/clock/testing"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/executor"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/gardener/gardener/pkg/operation/care"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var (
	zeroTime     time.Time
	zeroMetaTime metav1.Time
)

func roleOf(obj metav1.Object) string {
	return obj.GetLabels()[v1beta1constants.GardenRole]
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
	return WithStatus(status)
}

func beConditionWithMissingRequiredDeployment(deployments []*appsv1.Deployment) types.GomegaMatcher {
	var names = make([]string, 0, len(deployments))
	for _, deploy := range deployments {
		names = append(names, deploy.Name)
	}
	return And(WithStatus(gardencorev1beta1.ConditionFalse), WithMessage(fmt.Sprintf("%s", names)))
}

func beConditionWithStatusAndCodes(status gardencorev1beta1.ConditionStatus, codes ...gardencorev1beta1.ErrorCode) types.GomegaMatcher {
	return And(WithStatus(status), WithCodes(codes...))
}

func beConditionWithStatusAndMsg(status gardencorev1beta1.ConditionStatus, reason, message string) types.GomegaMatcher {
	return And(WithStatus(status), WithReason(reason), WithMessage(message))
}

var _ = Describe("health check", func() {
	var (
		ctx        = context.TODO()
		fakeClient client.Client
		fakeClock  = testclock.NewFakeClock(time.Now())

		condition gardencorev1beta1.Condition
		shoot     = &gardencorev1beta1.Shoot{
			Spec: gardencorev1beta1.ShootSpec{
				Provider: gardencorev1beta1.Provider{
					Workers: []gardencorev1beta1.Worker{
						{
							Name: "worker",
						},
					},
				},
			},
		}

		workerlessShoot = &gardencorev1beta1.Shoot{
			Spec: gardencorev1beta1.ShootSpec{
				Provider: gardencorev1beta1.Provider{
					Workers: []gardencorev1beta1.Worker{},
				},
			},
		}

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
				Provider: gardencorev1beta1.Provider{
					Workers: []gardencorev1beta1.Worker{
						{
							Name: "foo",
						},
					},
				},
			},
		}

		seedNamespace     = "shoot--foo--bar"
		kubernetesVersion = semver.MustParse("1.23.3")
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

		// TODO(rickardsjp, istvanballok): Remove in release v1.77
		grafanaDeployment               = newDeployment(seedNamespace, v1beta1constants.DeploymentNameGrafana, v1beta1constants.GardenRoleMonitoring, true)
		plutonoDeployment               = newDeployment(seedNamespace, v1beta1constants.DeploymentNamePlutono, v1beta1constants.GardenRoleMonitoring, true)
		kubeStateMetricsShootDeployment = newDeployment(seedNamespace, v1beta1constants.DeploymentNameKubeStateMetrics, v1beta1constants.GardenRoleMonitoring, true)

		requiredMonitoringControlPlaneDeploymentsLessThan171 = []*appsv1.Deployment{
			grafanaDeployment,
			kubeStateMetricsShootDeployment,
		}

		requiredMonitoringControlPlaneDeploymentsGreaterEqual171 = []*appsv1.Deployment{
			plutonoDeployment,
			kubeStateMetricsShootDeployment,
		}

		alertManagerStatefulSet = newStatefulSet(seedNamespace, v1beta1constants.StatefulSetNameAlertManager, v1beta1constants.GardenRoleMonitoring, true)
		prometheusStatefulSet   = newStatefulSet(seedNamespace, v1beta1constants.StatefulSetNamePrometheus, v1beta1constants.GardenRoleMonitoring, true)

		requiredMonitoringControlPlaneStatefulSets = []*appsv1.StatefulSet{
			alertManagerStatefulSet,
			prometheusStatefulSet,
		}

		lokiStatefulSet = newStatefulSet(seedNamespace, v1beta1constants.StatefulSetNameLoki, v1beta1constants.GardenRoleLogging, true)
		valiStatefulSet = newStatefulSet(seedNamespace, v1beta1constants.StatefulSetNameVali, v1beta1constants.GardenRoleLogging, true)

		requiredLoggingControlPlaneStatefulSetsLessThan171 = []*appsv1.StatefulSet{
			lokiStatefulSet,
		}

		requiredLoggingControlPlaneStatefulSetsGreaterEqual171 = []*appsv1.StatefulSet{
			valiStatefulSet,
		}

		eventLoggerDepployment = newDeployment(seedNamespace, v1beta1constants.DeploymentNameEventLogger, v1beta1constants.GardenRoleLogging, true)

		requiredLoggingControlPlaneDeployments = []*appsv1.Deployment{
			eventLoggerDepployment,
		}
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		fakeClock = testclock.NewFakeClock(time.Now())
		condition = gardencorev1beta1.Condition{
			Type:               gardencorev1beta1.ConditionType("test"),
			LastTransitionTime: metav1.Time{Time: fakeClock.Now()},
		}
	})

	DescribeTable("#CheckControlPlane",
		func(shoot *gardencorev1beta1.Shoot, deployments []*appsv1.Deployment, etcds []*druidv1alpha1.Etcd, workers []*extensionsv1alpha1.Worker, conditionMatcher types.GomegaMatcher) {
			for _, obj := range deployments {
				Expect(fakeClient.Create(ctx, obj.DeepCopy())).To(Succeed(), "creating deployment "+client.ObjectKeyFromObject(obj).String())
			}
			for _, obj := range etcds {
				Expect(fakeClient.Create(ctx, obj.DeepCopy())).To(Succeed(), "creating etcd "+client.ObjectKeyFromObject(obj).String())
			}
			for _, obj := range workers {
				Expect(fakeClient.Create(ctx, obj.DeepCopy())).To(Succeed(), "creating worker "+client.ObjectKeyFromObject(obj).String())
			}

			checker := care.NewHealthChecker(fakeClient, fakeClock, map[gardencorev1beta1.ConditionType]time.Duration{}, nil, nil, nil, kubernetesVersion, gardenerVersion)

			exitCondition, err := checker.CheckControlPlane(ctx, shoot, seedNamespace, condition)
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
		Entry("all healthy (workerless)",
			workerlessShoot,
			[]*appsv1.Deployment{
				gardenerResourceManagerDeployment,
				kubeAPIServerDeployment,
				kubeControllerManagerDeployment,
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
				{ObjectMeta: metav1.ObjectMeta{Name: "worker", Namespace: seedNamespace},
					Status: extensionsv1alpha1.WorkerStatus{DefaultStatus: extensionsv1alpha1.DefaultStatus{
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
				{ObjectMeta: metav1.ObjectMeta{Name: "worker", Namespace: seedNamespace},
					Status: extensionsv1alpha1.WorkerStatus{DefaultStatus: extensionsv1alpha1.DefaultStatus{
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
		Entry("missing required deployments (workerless)",
			workerlessShoot,
			[]*appsv1.Deployment{
				kubeAPIServerDeployment,
			},
			requiredControlPlaneEtcds,
			nil,
			PointTo(beConditionWithMissingRequiredDeployment([]*appsv1.Deployment{gardenerResourceManagerDeployment, kubeControllerManagerDeployment}))),
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
			PointTo(beConditionWithStatusAndCodes(gardencorev1beta1.ConditionFalse))),
	)

	DescribeTable("#CheckManagedResource",
		func(conditions []gardencorev1beta1.Condition, upToDate bool, stepTime bool, conditionMatcher types.GomegaMatcher) {
			var (
				mr      = new(resourcesv1alpha1.ManagedResource)
				checker = care.NewHealthChecker(fakeClient, fakeClock, map[gardencorev1beta1.ConditionType]time.Duration{}, nil, &metav1.Duration{Duration: 5 * time.Minute}, nil, kubernetesVersion, gardenerVersion)
			)

			if !upToDate {
				mr.Generation++
			}

			if stepTime {
				fakeClock.Step(5 * time.Minute)
			}

			mr.Status.Conditions = conditions

			exitCondition := checker.CheckManagedResource(condition, mr)
			Expect(exitCondition).To(conditionMatcher)
		},
		Entry("no conditions",
			nil,
			true,
			false,
			PointTo(beConditionWithStatusAndMsg(gardencorev1beta1.ConditionFalse, gardencorev1beta1.ManagedResourceMissingConditionError, ""))),
		Entry("one true condition, one missing",
			[]gardencorev1beta1.Condition{
				{
					Type:   resourcesv1alpha1.ResourcesApplied,
					Status: gardencorev1beta1.ConditionTrue,
				},
			},
			true,
			false,
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
				{
					Type:   resourcesv1alpha1.ResourcesProgressing,
					Status: gardencorev1beta1.ConditionFalse,
				},
			},
			true,
			false,
			BeNil()),
		Entry("both progressing and healthy conditions are true for less than ManagedResourceProgressingThreshold",
			[]gardencorev1beta1.Condition{
				{
					Type:               resourcesv1alpha1.ResourcesProgressing,
					Status:             gardencorev1beta1.ConditionTrue,
					LastTransitionTime: metav1.Time{Time: fakeClock.Now()},
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
			false,
			BeNil()),
		Entry("both progressing and healthy conditions are true for more than ManagedResourceProgressingThreshold",
			[]gardencorev1beta1.Condition{
				{
					Type:               resourcesv1alpha1.ResourcesProgressing,
					Status:             gardencorev1beta1.ConditionTrue,
					LastTransitionTime: metav1.Time{Time: fakeClock.Now()},
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
			true,
			PointTo(beConditionWithStatusAndMsg(gardencorev1beta1.ConditionFalse, gardencorev1beta1.ManagedResourceProgressingRolloutStuck, "ManagedResource  is progressing for more than 5m0s"))),
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
			false,
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
			false,
			PointTo(beConditionWithStatus(gardencorev1beta1.ConditionFalse))),
		Entry("multiple false conditions with reason & message & ResourcesApplied condition is not false",
			[]gardencorev1beta1.Condition{
				{
					Type:    resourcesv1alpha1.ResourcesHealthy,
					Status:  gardencorev1beta1.ConditionFalse,
					Reason:  "barFailed",
					Message: "bar is unhealthy",
				},
				{
					Type:    resourcesv1alpha1.ResourcesProgressing,
					Status:  gardencorev1beta1.ConditionFalse,
					Reason:  "fooFailed",
					Message: "foo is unhealthy",
				},
			},
			true,
			false,
			PointTo(beConditionWithStatusAndMsg(gardencorev1beta1.ConditionFalse, "barFailed", "bar is unhealthy"))),
		Entry("multiple false conditions with reason & message & ResourcesApplied condition is false",
			[]gardencorev1beta1.Condition{
				{
					Type:    resourcesv1alpha1.ResourcesHealthy,
					Status:  gardencorev1beta1.ConditionFalse,
					Reason:  "barFailed",
					Message: "bar is unhealthy",
				},
				{
					Type:    resourcesv1alpha1.ResourcesApplied,
					Status:  gardencorev1beta1.ConditionFalse,
					Reason:  "fooFailed",
					Message: "foo is unhealthy",
				},
			},
			true,
			false,
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
			false,
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
			cloudConfigSecretMeta      = map[string]metav1.ObjectMeta{
				workerPoolName1: {
					Name:        "cloud-config-cpu-worker-1-c63c0",
					Labels:      map[string]string{"worker.gardener.cloud/pool": workerPoolName1},
					Annotations: map[string]string{"checksum/data-script": cloudConfigSecretChecksum1},
				},
				workerPoolName2: {
					Name:        "bar",
					Labels:      map[string]string{"worker.gardener.cloud/pool": workerPoolName2},
					Annotations: map[string]string{"checksum/data-script": cloudConfigSecretChecksum2},
				},
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
			func(nodes []corev1.Node, workerPools []gardencorev1beta1.Worker, cloudConfigSecretMeta map[string]metav1.ObjectMeta, conditionMatcher types.GomegaMatcher) {
				c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&corev1.NodeList{})).DoAndReturn(func(_ context.Context, list *corev1.NodeList, _ ...client.ListOption) error {
					*list = corev1.NodeList{Items: nodes}
					return nil
				})
				cloudConfigSecretListOptions := []client.ListOption{
					client.InNamespace(metav1.NamespaceSystem),
					client.MatchingLabels{"gardener.cloud/role": "cloud-config"},
				}
				c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&corev1.SecretList{}), cloudConfigSecretListOptions).DoAndReturn(func(_ context.Context, list *corev1.SecretList, _ ...client.ListOption) error {
					*list = corev1.SecretList{}
					for _, meta := range cloudConfigSecretMeta {
						list.Items = append(list.Items, corev1.Secret{
							ObjectMeta: meta,
						})
					}
					return nil
				})

				checker := care.NewHealthChecker(fakeClient, fakeClock, map[gardencorev1beta1.ConditionType]time.Duration{}, nil, nil, nil, kubernetesVersion, gardenerVersion)

				exitCondition, err := checker.CheckClusterNodes(ctx, c, seedNamespace, workerPools, condition)
				Expect(err).NotTo(HaveOccurred())
				Expect(exitCondition).To(conditionMatcher)
			},
			Entry("all healthy",
				[]corev1.Node{
					newNode(nodeName, true, labels.Set{"worker.gardener.cloud/pool": workerPoolName1, "worker.gardener.cloud/kubernetes-version": "1.24.0"}, nil, kubernetesVersion.Original()),
				},
				[]gardencorev1beta1.Worker{
					{
						Name:    workerPoolName1,
						Maximum: 10,
						Minimum: 1,
					},
				},
				cloudConfigSecretMeta,
				BeNil()),
			Entry("node not healthy",
				[]corev1.Node{
					newNode(nodeName, false, labels.Set{"worker.gardener.cloud/pool": workerPoolName1, "worker.gardener.cloud/kubernetes-version": "1.24.0"}, nil, kubernetesVersion.Original()),
				},
				[]gardencorev1beta1.Worker{
					{
						Name:    workerPoolName1,
						Maximum: 10,
						Minimum: 1,
					},
				},
				cloudConfigSecretMeta,
				PointTo(beConditionWithStatusAndMsg(gardencorev1beta1.ConditionFalse, "NodeUnhealthy", fmt.Sprintf("Node %q in worker group %q is unhealthy", nodeName, workerPoolName1)))),
			Entry("node not healthy with error codes",
				[]corev1.Node{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:   nodeName,
							Labels: labels.Set{"worker.gardener.cloud/pool": workerPoolName1, "worker.gardener.cloud/kubernetes-version": "1.24.0"},
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
				cloudConfigSecretMeta,
				PointTo(beConditionWithStatusAndCodes(gardencorev1beta1.ConditionFalse, gardencorev1beta1.ErrorConfigurationProblem))),
			Entry("not enough nodes in worker pool",
				[]corev1.Node{
					newNode(nodeName, true, labels.Set{"worker.gardener.cloud/pool": workerPoolName1, "worker.gardener.cloud/kubernetes-version": "1.24.0"}, nil, kubernetesVersion.Original()),
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
				cloudConfigSecretMeta,
				PointTo(beConditionWithStatusAndMsg(gardencorev1beta1.ConditionFalse, "MissingNodes", fmt.Sprintf("Not enough worker nodes registered in worker pool %q to meet minimum desired machine count. (%d/%d).", workerPoolName2, 0, 1)))),
			Entry("not enough nodes in worker pool",
				[]corev1.Node{
					newNode(nodeName, true, labels.Set{"worker.gardener.cloud/pool": workerPoolName1, "worker.gardener.cloud/kubernetes-version": "1.24.0"}, nil, kubernetesVersion.Original()),
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
				cloudConfigSecretMeta,
				PointTo(beConditionWithStatusAndMsg(gardencorev1beta1.ConditionFalse, "MissingNodes", fmt.Sprintf("Not enough worker nodes registered in worker pool %q to meet minimum desired machine count. (%d/%d).", workerPoolName2, 0, 1)))),
			Entry("too old Kubernetes patch version",
				[]corev1.Node{
					newNode(nodeName, true, labels.Set{"worker.gardener.cloud/pool": workerPoolName1, "worker.gardener.cloud/kubernetes-version": "1.24.0"}, nil, "v1.23.2"),
				},
				[]gardencorev1beta1.Worker{
					{
						Name:    workerPoolName1,
						Maximum: 10,
						Minimum: 1,
					},
				},
				cloudConfigSecretMeta,
				PointTo(beConditionWithStatusAndMsg(gardencorev1beta1.ConditionFalse, "KubeletVersionMismatch", fmt.Sprintf("The kubelet version for node %q (v1.23.2) does not match the desired Kubernetes version (v%s)", nodeName, kubernetesVersion.Original())))),
			Entry("same Kubernetes patch version",
				[]corev1.Node{
					newNode(nodeName, true, labels.Set{"worker.gardener.cloud/pool": workerPoolName1, "worker.gardener.cloud/kubernetes-version": "1.24.0"}, nil, "v1.23.3"),
				},
				[]gardencorev1beta1.Worker{
					{
						Name:    workerPoolName1,
						Maximum: 10,
						Minimum: 1,
					},
				},
				cloudConfigSecretMeta,
				BeNil()),
			Entry("too old Kubernetes patch version with pool version overwrite",
				[]corev1.Node{
					newNode(nodeName, true, labels.Set{"worker.gardener.cloud/pool": workerPoolName1, "worker.gardener.cloud/kubernetes-version": "1.24.0"}, nil, "v1.22.2"),
				},
				[]gardencorev1beta1.Worker{
					{
						Name:    workerPoolName1,
						Maximum: 10,
						Minimum: 1,
						Kubernetes: &gardencorev1beta1.WorkerKubernetes{
							Version: pointer.String("1.22.3"),
						},
					},
				},
				cloudConfigSecretMeta,
				PointTo(beConditionWithStatusAndMsg(gardencorev1beta1.ConditionFalse, "KubeletVersionMismatch", fmt.Sprintf("The kubelet version for node %q (v1.22.2) does not match the desired Kubernetes version (v1.22.3)", nodeName)))),
			Entry("different Kubernetes minor version (all healthy)",
				[]corev1.Node{
					newNode(nodeName, true, labels.Set{"worker.gardener.cloud/pool": workerPoolName1, "worker.gardener.cloud/kubernetes-version": "1.24.0"}, nil, "v1.22.2"),
				},
				[]gardencorev1beta1.Worker{
					{
						Name:    workerPoolName1,
						Maximum: 10,
						Minimum: 1,
					},
				},
				cloudConfigSecretMeta,
				BeNil()),
			Entry("missing cloud-config secret checksum for a worker pool",
				[]corev1.Node{
					newNode(nodeName, true, labels.Set{"worker.gardener.cloud/pool": workerPoolName1, "worker.gardener.cloud/kubernetes-version": "1.24.0"}, nil, "v1.22.2"),
				},
				[]gardencorev1beta1.Worker{
					{
						Name:    workerPoolName1,
						Maximum: 10,
						Minimum: 1,
					},
				},
				nil,
				PointTo(beConditionWithStatusAndMsg(gardencorev1beta1.ConditionFalse, "CloudConfigOutdated", fmt.Sprintf("missing cloud config secret metadata for worker pool %q", workerPoolName1)))),
			Entry("no cloud-config node checksum for a worker pool",
				[]corev1.Node{
					newNode(nodeName, true, labels.Set{"worker.gardener.cloud/pool": workerPoolName1, "worker.gardener.cloud/kubernetes-version": "1.24.0"}, nil, "v1.22.2"),
				},
				[]gardencorev1beta1.Worker{
					{
						Name:    workerPoolName1,
						Maximum: 10,
						Minimum: 1,
					},
				},
				cloudConfigSecretMeta,
				BeNil()),
			Entry("outdated cloud-config secret checksum for a worker pool",
				[]corev1.Node{
					newNode(nodeName, true, labels.Set{"worker.gardener.cloud/pool": workerPoolName1, "worker.gardener.cloud/kubernetes-version": "1.24.0"}, map[string]string{executor.AnnotationKeyChecksum: "outdated"}, "v1.22.2"),
				},
				[]gardencorev1beta1.Worker{
					{
						Name:    workerPoolName1,
						Maximum: 10,
						Minimum: 1,
					},
				},
				map[string]metav1.ObjectMeta{
					workerPoolName1: {
						Name:        "cloud-config-cpu-worker-1-c63c0",
						Annotations: map[string]string{"checksum/data-script": cloudConfigSecretChecksum1},
						Labels:      map[string]string{"worker.gardener.cloud/pool": workerPoolName1},
					},
				},
				PointTo(beConditionWithStatusAndMsg(gardencorev1beta1.ConditionFalse, "CloudConfigOutdated", fmt.Sprintf("the last successfully applied cloud config on node %q is outdated", nodeName)))),
		)
	})

	gardenerVersion170 := semver.MustParse("1.70.0-mod1")
	gardenerVersion171 := semver.MustParse("1.71.0")
	DescribeTable("#CheckMonitoringControlPlane",
		func(deployments []*appsv1.Deployment, statefulSets []*appsv1.StatefulSet, workerless, wantsShootMonitoring, wantsAlertmanager bool, gardenerVersion *semver.Version, conditionMatcher types.GomegaMatcher) {
			for _, obj := range deployments {
				Expect(fakeClient.Create(ctx, obj.DeepCopy())).To(Succeed(), "creating deployment "+client.ObjectKeyFromObject(obj).String())
			}
			for _, obj := range statefulSets {
				Expect(fakeClient.Create(ctx, obj.DeepCopy())).To(Succeed(), "creating statefulset "+client.ObjectKeyFromObject(obj).String())
			}

			checker := care.NewHealthChecker(fakeClient, fakeClock, map[gardencorev1beta1.ConditionType]time.Duration{}, nil, nil, nil, kubernetesVersion, gardenerVersion)

			s := shoot.DeepCopy()
			if workerless {
				s = workerlessShoot.DeepCopy()
			}

			exitCondition, err := checker.CheckMonitoringControlPlane(ctx, s, seedNamespace, wantsShootMonitoring, wantsAlertmanager, condition)
			Expect(err).NotTo(HaveOccurred())
			Expect(exitCondition).To(conditionMatcher)
		},
		Entry("all healthy (1.70)",
			requiredMonitoringControlPlaneDeploymentsLessThan171,
			requiredMonitoringControlPlaneStatefulSets,
			false,
			true,
			true,
			gardenerVersion170,
			BeNil()),
		Entry("all healthy (1.70, workerless)",
			[]*appsv1.Deployment{
				grafanaDeployment,
			},
			requiredMonitoringControlPlaneStatefulSets,
			true,
			true,
			true,
			gardenerVersion170,
			BeNil()),
		Entry("all healthy (1.71)",
			requiredMonitoringControlPlaneDeploymentsGreaterEqual171,
			requiredMonitoringControlPlaneStatefulSets,
			false,
			true,
			true,
			gardenerVersion171,
			BeNil()),
		Entry("all healthy (1.71, workerless)",
			[]*appsv1.Deployment{
				plutonoDeployment,
			},
			requiredMonitoringControlPlaneStatefulSets,
			true,
			true,
			true,
			gardenerVersion171,
			BeNil()),
		Entry("required deployment missing",
			[]*appsv1.Deployment{
				plutonoDeployment,
			},
			requiredMonitoringControlPlaneStatefulSets,
			false,
			true,
			true,
			gardenerVersion171,
			PointTo(beConditionWithMissingRequiredDeployment([]*appsv1.Deployment{kubeStateMetricsShootDeployment}))),
		Entry("required deployment missing (workerless Shoot)",
			[]*appsv1.Deployment{},
			requiredMonitoringControlPlaneStatefulSets,
			true,
			true,
			true,
			gardenerVersion171,
			PointTo(beConditionWithMissingRequiredDeployment([]*appsv1.Deployment{plutonoDeployment}))),
		Entry("required stateful set set missing",
			requiredMonitoringControlPlaneDeploymentsGreaterEqual171,
			[]*appsv1.StatefulSet{
				prometheusStatefulSet,
			},
			false,
			true,
			true,
			gardenerVersion171,
			PointTo(beConditionWithStatus(gardencorev1beta1.ConditionFalse))),
		Entry("deployment unhealthy",
			[]*appsv1.Deployment{
				newDeployment(plutonoDeployment.Namespace, plutonoDeployment.Name, roleOf(plutonoDeployment), false),
				kubeStateMetricsShootDeployment,
			},
			requiredMonitoringControlPlaneStatefulSets,
			false,
			true,
			true,
			gardenerVersion171,
			PointTo(beConditionWithStatus(gardencorev1beta1.ConditionFalse))),
		Entry("stateful set unhealthy",
			requiredMonitoringControlPlaneDeploymentsGreaterEqual171,
			[]*appsv1.StatefulSet{
				newStatefulSet(alertManagerStatefulSet.Namespace, alertManagerStatefulSet.Name, roleOf(alertManagerStatefulSet), false),
				prometheusStatefulSet,
			},
			false,
			true,
			true,
			gardenerVersion171,
			PointTo(beConditionWithStatus(gardencorev1beta1.ConditionFalse))),
		Entry("shoot has monitoring disabled, omit all checks",
			[]*appsv1.Deployment{},
			[]*appsv1.StatefulSet{},
			false,
			false,
			true,
			gardenerVersion171,
			BeNil()),
	)

	DescribeTable("#CheckLoggingControlPlane",
		func(deployments []*appsv1.Deployment, statefulSets []*appsv1.StatefulSet, isTestingShoot, eventLoggingEnabled, valiEnabled bool, conditionMatcher types.GomegaMatcher, gardenerVersion *semver.Version) {
			for _, obj := range deployments {
				Expect(fakeClient.Create(ctx, obj.DeepCopy())).To(Succeed(), "creating deployment "+client.ObjectKeyFromObject(obj).String())
			}
			for _, obj := range statefulSets {
				Expect(fakeClient.Create(ctx, obj.DeepCopy())).To(Succeed(), "creating statefulset "+client.ObjectKeyFromObject(obj).String())
			}

			checker := care.NewHealthChecker(fakeClient, fakeClock, map[gardencorev1beta1.ConditionType]time.Duration{}, nil, nil, nil, kubernetesVersion, gardenerVersion)

			exitCondition, err := checker.CheckLoggingControlPlane(ctx, seedNamespace, isTestingShoot, eventLoggingEnabled, valiEnabled, condition)
			Expect(err).NotTo(HaveOccurred())
			Expect(exitCondition).To(conditionMatcher)
		},
		Entry("all healthy (1.70)",
			requiredLoggingControlPlaneDeployments,
			requiredLoggingControlPlaneStatefulSetsLessThan171,
			false,
			true,
			true,
			BeNil(),
			gardenerVersion170,
		),
		Entry("all healthy (1.71)",
			requiredLoggingControlPlaneDeployments,
			requiredLoggingControlPlaneStatefulSetsGreaterEqual171,
			false,
			true,
			true,
			BeNil(),
			gardenerVersion171,
		),
		Entry("required stateful set missing",
			requiredLoggingControlPlaneDeployments,
			nil,
			false,
			true,
			true,
			PointTo(beConditionWithStatus(gardencorev1beta1.ConditionFalse)),
			gardenerVersion171,
		),
		Entry("required deployment is missing",
			nil,
			requiredLoggingControlPlaneStatefulSetsGreaterEqual171,
			false,
			true,
			true,
			PointTo(beConditionWithStatus(gardencorev1beta1.ConditionFalse)),
			gardenerVersion171,
		),
		Entry("stateful set unhealthy",
			requiredLoggingControlPlaneDeployments,
			[]*appsv1.StatefulSet{
				newStatefulSet(valiStatefulSet.Namespace, valiStatefulSet.Name, roleOf(valiStatefulSet), false),
			},
			false,
			true,
			true,
			PointTo(beConditionWithStatus(gardencorev1beta1.ConditionFalse)),
			gardenerVersion171,
		),
		Entry("stateful set unhealthy",
			[]*appsv1.Deployment{
				newDeployment(eventLoggerDepployment.Namespace, eventLoggerDepployment.Name, roleOf(eventLoggerDepployment), false),
			},
			requiredLoggingControlPlaneStatefulSetsGreaterEqual171,
			false,
			true,
			true,
			PointTo(beConditionWithStatus(gardencorev1beta1.ConditionFalse)),
			gardenerVersion171,
		),
		Entry("shoot purpose is testing, omit all checks",
			[]*appsv1.Deployment{},
			[]*appsv1.StatefulSet{},
			true,
			true,
			true,
			BeNil(),
			gardenerVersion171,
		),
		Entry("vali is disabled in gardenlet config, omit stateful set check",
			requiredLoggingControlPlaneDeployments,
			[]*appsv1.StatefulSet{},
			false,
			true,
			false,
			BeNil(),
			gardenerVersion171,
		),
		Entry("event logging is disabled in gardenlet config, omit deployment check",
			[]*appsv1.Deployment{},
			requiredLoggingControlPlaneStatefulSetsGreaterEqual171,
			false,
			false,
			true,
			BeNil(),
			gardenerVersion171,
		),
	)

	DescribeTable("#FailedCondition",
		func(thresholds map[gardencorev1beta1.ConditionType]time.Duration, lastOperation *gardencorev1beta1.LastOperation, now time.Time, condition gardencorev1beta1.Condition, reason, message string, expected types.GomegaMatcher) {
			fakeClock.SetTime(now)
			checker := care.NewHealthChecker(fakeClient, fakeClock, thresholds, nil, nil, lastOperation, kubernetesVersion, gardenerVersion)
			Expect(checker.FailedCondition(condition, reason, message)).To(expected)
		},
		Entry("true condition with threshold",
			map[gardencorev1beta1.ConditionType]time.Duration{
				gardencorev1beta1.ShootControlPlaneHealthy: time.Minute,
			},
			nil,
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
			checker := care.NewHealthChecker(fakeClient, fakeClock, nil, healthCheckOutdatedThreshold, nil, nil, kubernetesVersion, gardenerVersion)
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
						Type:   gardencorev1beta1.ShootControlPlaneHealthy,
						Status: gardencorev1beta1.ConditionTrue,
					},
					LastHeartbeatTime: &metav1.MicroTime{Time: time.Now().Add(time.Second * -30)},
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
					},
					// health check result is only 30 seconds old so < than the staleExtensionHealthCheckThreshold
					LastHeartbeatTime: &metav1.MicroTime{Time: time.Now().Add(time.Second * -30)},
				},
			},
			BeNil(),
		),
		Entry("should determine that health check report is outdated - LastHeartbeatTime is nil",
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
					},
					ExtensionType:      "Worker",
					ExtensionName:      "worker-ubuntu",
					ExtensionNamespace: "shoot-namespace-in-seed",
				},
			},
			PointTo(beConditionWithStatus(gardencorev1beta1.ConditionUnknown)),
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
					},
					ExtensionType:      "Worker",
					ExtensionName:      "worker-ubuntu",
					ExtensionNamespace: "shoot-namespace-in-seed",
					// health check result is already 3 minutes old
					LastHeartbeatTime: &metav1.MicroTime{Time: time.Now().Add(time.Minute * -3)},
				},
			},
			PointTo(beConditionWithStatus(gardencorev1beta1.ConditionUnknown)),
		),
		Entry("health check reports status progressing",
			nil,
			gardencorev1beta1.Condition{Type: "type"},
			[]care.ExtensionCondition{
				{
					ExtensionType: "Foo",
					Condition: gardencorev1beta1.Condition{
						Type:    gardencorev1beta1.ShootControlPlaneHealthy,
						Status:  gardencorev1beta1.ConditionProgressing,
						Reason:  "Bar",
						Message: "Baz",
					},
					LastHeartbeatTime: &metav1.MicroTime{Time: time.Now()},
				},
			},
			PointTo(beConditionWithStatusReasonAndMessage(gardencorev1beta1.ConditionProgressing, "FooBar", "Baz")),
		),
		Entry("health check reports status false",
			nil,
			gardencorev1beta1.Condition{Type: "type"},
			[]care.ExtensionCondition{
				{
					ExtensionType: "Foo",
					Condition: gardencorev1beta1.Condition{
						Type:   gardencorev1beta1.ShootControlPlaneHealthy,
						Status: gardencorev1beta1.ConditionFalse,
					},
					LastHeartbeatTime: &metav1.MicroTime{Time: time.Now()},
				},
			},
			PointTo(beConditionWithStatusReasonAndMessage(gardencorev1beta1.ConditionFalse, "FooUnhealthyReport", "failing health check")),
		),
		Entry("health check reports status unknown",
			nil,
			gardencorev1beta1.Condition{Type: "type"},
			[]care.ExtensionCondition{
				{
					ExtensionType: "Foo",
					Condition: gardencorev1beta1.Condition{
						Type:   gardencorev1beta1.ShootControlPlaneHealthy,
						Status: gardencorev1beta1.ConditionUnknown,
					},
					LastHeartbeatTime: &metav1.MicroTime{Time: time.Now()},
				},
			},
			PointTo(beConditionWithStatusReasonAndMessage(gardencorev1beta1.ConditionFalse, "FooUnhealthyReport", "failing health check")),
		),
	)

	DescribeTable("#PardonCondition",
		func(condition gardencorev1beta1.Condition, lastOp *gardencorev1beta1.LastOperation, lastErrors []gardencorev1beta1.LastError, expected types.GomegaMatcher) {
			conditions := []gardencorev1beta1.Condition{condition}
			updatedConditions := care.PardonConditions(fakeClock, conditions, lastOp, lastErrors)
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

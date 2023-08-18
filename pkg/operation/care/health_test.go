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
	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/onsi/gomega/types"
	"go.uber.org/mock/gomock"
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
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component/extensions/operatingsystemconfig/executor"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	. "github.com/gardener/gardener/pkg/operation/care"
	healthchecker "github.com/gardener/gardener/pkg/utils/kubernetes/health/checker"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("health check", func() {
	var (
		ctx          = context.Background()
		fakeClient   client.Client
		zeroTime     time.Time
		zeroMetaTime metav1.Time
		fakeClock    = testclock.NewFakeClock(time.Now())

		condition gardencorev1beta1.Condition

		seedNamespace     = "shoot--foo--bar"
		kubernetesVersion = semver.MustParse("1.23.3")

		shoot = &gardencorev1beta1.Shoot{
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

		plutonoDeployment               = newDeployment(seedNamespace, v1beta1constants.DeploymentNamePlutono, v1beta1constants.GardenRoleMonitoring, true)
		kubeStateMetricsShootDeployment = newDeployment(seedNamespace, v1beta1constants.DeploymentNameKubeStateMetrics, v1beta1constants.GardenRoleMonitoring, true)

		requiredMonitoringControlPlaneDeployments = []*appsv1.Deployment{
			plutonoDeployment,
			kubeStateMetricsShootDeployment,
		}

		alertManagerStatefulSet = newStatefulSet(seedNamespace, v1beta1constants.StatefulSetNameAlertManager, v1beta1constants.GardenRoleMonitoring, true)
		prometheusStatefulSet   = newStatefulSet(seedNamespace, v1beta1constants.StatefulSetNamePrometheus, v1beta1constants.GardenRoleMonitoring, true)

		requiredMonitoringControlPlaneStatefulSets = []*appsv1.StatefulSet{
			alertManagerStatefulSet,
			prometheusStatefulSet,
		}

		valiStatefulSet = newStatefulSet(seedNamespace, v1beta1constants.StatefulSetNameVali, v1beta1constants.GardenRoleLogging, true)

		requiredLoggingControlPlaneStatefulSets = []*appsv1.StatefulSet{
			valiStatefulSet,
		}

		eventLoggerDepployment = newDeployment(seedNamespace, v1beta1constants.DeploymentNameEventLogger, v1beta1constants.GardenRoleLogging, true)

		requiredLoggingControlPlaneDeployments = []*appsv1.Deployment{
			eventLoggerDepployment,
		}

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
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		fakeClock = testclock.NewFakeClock(time.Now())
		condition = gardencorev1beta1.Condition{
			Type:               "test",
			LastTransitionTime: metav1.Time{Time: fakeClock.Now()},
		}
	})

	DescribeTable("#CheckShootontrolPlane",
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

			checker := healthchecker.NewHealthChecker(fakeClient, fakeClock, map[gardencorev1beta1.ConditionType]time.Duration{}, nil, nil, nil, kubernetesVersion)

			exitCondition, err := CheckShootControlPlane(ctx, shoot, checker, seedNamespace, condition)
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

	DescribeTable("#CheckShootMonitoringControlPlane",
		func(deployments []*appsv1.Deployment, statefulSets []*appsv1.StatefulSet, workerless, wantsShootMonitoring, wantsAlertmanager bool, conditionMatcher types.GomegaMatcher) {
			for _, obj := range deployments {
				Expect(fakeClient.Create(ctx, obj.DeepCopy())).To(Succeed(), "creating deployment "+client.ObjectKeyFromObject(obj).String())
			}
			for _, obj := range statefulSets {
				Expect(fakeClient.Create(ctx, obj.DeepCopy())).To(Succeed(), "creating statefulset "+client.ObjectKeyFromObject(obj).String())
			}

			checker := healthchecker.NewHealthChecker(fakeClient, fakeClock, map[gardencorev1beta1.ConditionType]time.Duration{}, nil, nil, nil, kubernetesVersion)

			s := shoot.DeepCopy()
			if workerless {
				s = workerlessShoot.DeepCopy()
			}

			exitCondition, err := CheckShootMonitoringControlPlane(ctx, s, checker, seedNamespace, wantsShootMonitoring, wantsAlertmanager, condition)
			Expect(err).NotTo(HaveOccurred())
			Expect(exitCondition).To(conditionMatcher)
		},
		Entry("all healthy",
			requiredMonitoringControlPlaneDeployments,
			requiredMonitoringControlPlaneStatefulSets,
			false,
			true,
			true,
			BeNil()),
		Entry("all healthy (workerless)",
			[]*appsv1.Deployment{
				plutonoDeployment,
			},
			requiredMonitoringControlPlaneStatefulSets,
			true,
			true,
			true,
			BeNil()),
		Entry("required deployment missing",
			[]*appsv1.Deployment{
				plutonoDeployment,
			},
			requiredMonitoringControlPlaneStatefulSets,
			false,
			true,
			true,
			PointTo(beConditionWithMissingRequiredDeployment([]*appsv1.Deployment{kubeStateMetricsShootDeployment}))),
		Entry("required deployment missing (workerless Shoot)",
			[]*appsv1.Deployment{},
			requiredMonitoringControlPlaneStatefulSets,
			true,
			true,
			true,
			PointTo(beConditionWithMissingRequiredDeployment([]*appsv1.Deployment{plutonoDeployment}))),
		Entry("required stateful set set missing",
			requiredMonitoringControlPlaneDeployments,
			[]*appsv1.StatefulSet{
				prometheusStatefulSet,
			},
			false,
			true,
			true,
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
			PointTo(beConditionWithStatus(gardencorev1beta1.ConditionFalse))),
		Entry("stateful set unhealthy",
			requiredMonitoringControlPlaneDeployments,
			[]*appsv1.StatefulSet{
				newStatefulSet(alertManagerStatefulSet.Namespace, alertManagerStatefulSet.Name, roleOf(alertManagerStatefulSet), false),
				prometheusStatefulSet,
			},
			false,
			true,
			true,
			PointTo(beConditionWithStatus(gardencorev1beta1.ConditionFalse))),
		Entry("shoot has monitoring disabled, omit all checks",
			[]*appsv1.Deployment{},
			[]*appsv1.StatefulSet{},
			false,
			false,
			true,
			BeNil()),
	)

	DescribeTable("#CheckLoggingControlPlane",
		func(deployments []*appsv1.Deployment, statefulSets []*appsv1.StatefulSet, isTestingShoot, eventLoggingEnabled, valiEnabled bool, conditionMatcher types.GomegaMatcher) {
			for _, obj := range deployments {
				Expect(fakeClient.Create(ctx, obj.DeepCopy())).To(Succeed(), "creating deployment "+client.ObjectKeyFromObject(obj).String())
			}
			for _, obj := range statefulSets {
				Expect(fakeClient.Create(ctx, obj.DeepCopy())).To(Succeed(), "creating statefulset "+client.ObjectKeyFromObject(obj).String())
			}

			checker := healthchecker.NewHealthChecker(fakeClient, fakeClock, map[gardencorev1beta1.ConditionType]time.Duration{}, nil, nil, nil, kubernetesVersion)

			exitCondition, err := checker.CheckLoggingControlPlane(ctx, seedNamespace, isTestingShoot, eventLoggingEnabled, valiEnabled, condition)
			Expect(err).NotTo(HaveOccurred())
			Expect(exitCondition).To(conditionMatcher)
		},
		Entry("all healthy",
			requiredLoggingControlPlaneDeployments,
			requiredLoggingControlPlaneStatefulSets,
			false,
			true,
			true,
			BeNil(),
		),
		Entry("required stateful set missing",
			requiredLoggingControlPlaneDeployments,
			nil,
			false,
			true,
			true,
			PointTo(beConditionWithStatus(gardencorev1beta1.ConditionFalse)),
		),
		Entry("required deployment is missing",
			nil,
			requiredLoggingControlPlaneStatefulSets,
			false,
			true,
			true,
			PointTo(beConditionWithStatus(gardencorev1beta1.ConditionFalse)),
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
		),
		Entry("stateful set unhealthy",
			[]*appsv1.Deployment{
				newDeployment(eventLoggerDepployment.Namespace, eventLoggerDepployment.Name, roleOf(eventLoggerDepployment), false),
			},
			requiredLoggingControlPlaneStatefulSets,
			false,
			true,
			true,
			PointTo(beConditionWithStatus(gardencorev1beta1.ConditionFalse)),
		),
		Entry("shoot purpose is testing, omit all checks",
			[]*appsv1.Deployment{},
			[]*appsv1.StatefulSet{},
			true,
			true,
			true,
			BeNil(),
		),
		Entry("vali is disabled in gardenlet config, omit stateful set check",
			requiredLoggingControlPlaneDeployments,
			[]*appsv1.StatefulSet{},
			false,
			true,
			false,
			BeNil(),
		),
		Entry("event logging is disabled in gardenlet config, omit deployment check",
			[]*appsv1.Deployment{},
			requiredLoggingControlPlaneStatefulSets,
			false,
			false,
			true,
			BeNil(),
		),
	)

	DescribeTable("#FailedCondition",
		func(thresholds map[gardencorev1beta1.ConditionType]time.Duration, lastOperation *gardencorev1beta1.LastOperation, now time.Time, condition gardencorev1beta1.Condition, reason, message string, expected types.GomegaMatcher) {
			fakeClock.SetTime(now)
			checker := healthchecker.NewHealthChecker(fakeClient, fakeClock, thresholds, nil, nil, lastOperation, kubernetesVersion)
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

	DescribeTable("#PardonCondition",
		func(condition gardencorev1beta1.Condition, lastOp *gardencorev1beta1.LastOperation, lastErrors []gardencorev1beta1.LastError, expected types.GomegaMatcher) {
			conditions := []gardencorev1beta1.Condition{condition}
			updatedConditions := PardonConditions(fakeClock, conditions, lastOp, lastErrors)
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

	Describe("#CheckClusterNodes", func() {
		var (
			ctrl *gomock.Controller
			c    *mockclient.MockClient

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

				checker := healthchecker.NewHealthChecker(fakeClient, fakeClock, map[gardencorev1beta1.ConditionType]time.Duration{}, nil, nil, nil, kubernetesVersion)

				exitCondition, err := CheckClusterNodes(ctx, c, fakeClient, checker, kubernetesVersion, seedNamespace, workerPools, condition)
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

	Describe("#CheckNodesScalingUp", func() {
		It("should return true if number of ready nodes equal number of desired machines", func() {
			Expect(CheckNodesScalingUp(nil, 1, 1)).To(Succeed())
		})

		It("should return an error if not enough machine objects as desired were created", func() {
			Expect(CheckNodesScalingUp(&machinev1alpha1.MachineList{}, 0, 1)).To(MatchError(ContainSubstring("not enough machine objects created yet")))
		})

		It("should return an error when detecting erroneous machines", func() {
			machineList := &machinev1alpha1.MachineList{
				Items: []machinev1alpha1.Machine{
					{
						Status: machinev1alpha1.MachineStatus{
							CurrentStatus: machinev1alpha1.CurrentStatus{Phase: machinev1alpha1.MachineUnknown},
						},
					},
				},
			}

			Expect(CheckNodesScalingUp(machineList, 0, 1)).To(MatchError(ContainSubstring("is erroneous")))
		})

		It("should return an error when not enough ready nodes are registered", func() {
			machineList := &machinev1alpha1.MachineList{
				Items: []machinev1alpha1.Machine{
					{
						Status: machinev1alpha1.MachineStatus{
							CurrentStatus: machinev1alpha1.CurrentStatus{Phase: machinev1alpha1.MachineRunning},
						},
					},
				},
			}

			Expect(CheckNodesScalingUp(machineList, 0, 1)).To(MatchError(ContainSubstring("not enough ready worker nodes registered in the cluster")))
		})

		It("should return progressing when detecting a regular scale up (pending status)", func() {
			machineList := &machinev1alpha1.MachineList{
				Items: []machinev1alpha1.Machine{
					{
						Status: machinev1alpha1.MachineStatus{
							CurrentStatus: machinev1alpha1.CurrentStatus{Phase: machinev1alpha1.MachinePending},
						},
					},
				},
			}

			Expect(CheckNodesScalingUp(machineList, 0, 1)).To(MatchError(ContainSubstring("is provisioning and should join the cluster soon")))
		})

		It("should return progressing when detecting a regular scale up (no status)", func() {
			machineList := &machinev1alpha1.MachineList{
				Items: []machinev1alpha1.Machine{
					{},
				},
			}

			Expect(CheckNodesScalingUp(machineList, 0, 1)).To(MatchError(ContainSubstring("is provisioning and should join the cluster soon")))
		})
	})

	Describe("#CheckNodesScalingDown", func() {
		It("should return true if number of registered nodes equal number of desired machines", func() {
			Expect(CheckNodesScalingDown(nil, nil, 1, 1)).To(Succeed())
		})

		It("should return an error if the machine for a cordoned node is not found", func() {
			nodeList := &corev1.NodeList{
				Items: []corev1.Node{
					{Spec: corev1.NodeSpec{Unschedulable: true}},
				},
			}

			Expect(CheckNodesScalingDown(&machinev1alpha1.MachineList{}, nodeList, 2, 1)).To(MatchError(ContainSubstring("machine object for cordoned node \"\" not found")))
		})

		It("should return an error if the machine for a cordoned node is not deleted", func() {
			var (
				nodeName = "foo"

				machineList = &machinev1alpha1.MachineList{
					Items: []machinev1alpha1.Machine{
						{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"node": nodeName}}},
					},
				}
				nodeList = &corev1.NodeList{
					Items: []corev1.Node{
						{
							ObjectMeta: metav1.ObjectMeta{Name: nodeName},
							Spec:       corev1.NodeSpec{Unschedulable: true},
						},
					},
				}
			)

			Expect(CheckNodesScalingDown(machineList, nodeList, 2, 1)).To(MatchError(ContainSubstring("found but corresponding machine object does not have a deletion timestamp")))
		})

		It("should return an error if there are more nodes then machines", func() {
			Expect(CheckNodesScalingDown(&machinev1alpha1.MachineList{}, &corev1.NodeList{Items: []corev1.Node{{}}}, 2, 1)).To(MatchError(ContainSubstring("too many worker nodes are registered. Exceeding maximum desired machine count")))
		})

		It("should return progressing for a regular scale down", func() {
			var (
				nodeName          = "foo"
				deletionTimestamp = metav1.Now()

				machineList = &machinev1alpha1.MachineList{
					Items: []machinev1alpha1.Machine{
						{ObjectMeta: metav1.ObjectMeta{DeletionTimestamp: &deletionTimestamp, Labels: map[string]string{"node": nodeName}}},
					},
				}
				nodeList = &corev1.NodeList{
					Items: []corev1.Node{
						{
							ObjectMeta: metav1.ObjectMeta{Name: nodeName},
							Spec:       corev1.NodeSpec{Unschedulable: true},
						},
					},
				}
			)

			Expect(CheckNodesScalingDown(machineList, nodeList, 2, 1)).To(MatchError(ContainSubstring("is waiting to be completely drained from pods")))
		})

		It("should ignore node not managed by MCM and return progressing for a regular scale down", func() {
			var (
				nodeName          = "foo"
				deletionTimestamp = metav1.Now()

				machineList = &machinev1alpha1.MachineList{
					Items: []machinev1alpha1.Machine{
						{ObjectMeta: metav1.ObjectMeta{DeletionTimestamp: &deletionTimestamp, Labels: map[string]string{"node": nodeName}}},
					},
				}
				nodeList = &corev1.NodeList{
					Items: []corev1.Node{
						{
							ObjectMeta: metav1.ObjectMeta{Name: nodeName},
							Spec:       corev1.NodeSpec{Unschedulable: true},
						},
						{
							ObjectMeta: metav1.ObjectMeta{
								Name:        "bar",
								Annotations: map[string]string{"node.machine.sapcloud.io/not-managed-by-mcm": "1"},
							},
						},
					},
				}
			)

			Expect(CheckNodesScalingDown(machineList, nodeList, 2, 1)).To(MatchError(ContainSubstring("is waiting to be completely drained from pods")))
		})
	})
})

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

func roleLabels(role string) map[string]string {
	return map[string]string{v1beta1constants.GardenRole: role}
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

func roleOf(obj metav1.Object) string {
	return obj.GetLabels()[v1beta1constants.GardenRole]
}

func beConditionWithStatusAndCodes(status gardencorev1beta1.ConditionStatus, codes ...gardencorev1beta1.ErrorCode) types.GomegaMatcher {
	return And(WithStatus(status), WithCodes(codes...))
}

func beConditionWithStatusAndMsg(status gardencorev1beta1.ConditionStatus, reason, message string) types.GomegaMatcher {
	return And(WithStatus(status), WithReason(reason), WithMessage(message))
}

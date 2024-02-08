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

package checker_test

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/onsi/gomega/types"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	testclock "k8s.io/utils/clock/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/utils/kubernetes/health/checker"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("HealthChecker", func() {
	var _ = Describe("health check", func() {
		var (
			ctx        = context.Background()
			fakeClient client.Client
			fakeClock  = testclock.NewFakeClock(time.Now())

			condition gardencorev1beta1.Condition

			namespace = "shoot--foo--bar"
		)

		BeforeEach(func() {
			fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
			fakeClock = testclock.NewFakeClock(time.Now())
			condition = gardencorev1beta1.Condition{
				Type:               "test",
				LastTransitionTime: metav1.Time{Time: fakeClock.Now()},
			}
		})

		DescribeTable("#CheckManagedResource",
			func(conditions []gardencorev1beta1.Condition, upToDate bool, stepTime bool, conditionMatcher types.GomegaMatcher) {
				var (
					mr      = new(resourcesv1alpha1.ManagedResource)
					checker = NewHealthChecker(fakeClient, fakeClock, map[gardencorev1beta1.ConditionType]time.Duration{}, nil)
				)

				if !upToDate {
					mr.Generation++
				}

				if stepTime {
					fakeClock.Step(5 * time.Minute)
				}

				mr.Status.Conditions = conditions

				exitCondition := checker.CheckManagedResource(condition, mr, &metav1.Duration{Duration: 5 * time.Minute})
				Expect(exitCondition).To(conditionMatcher)
			},
			Entry("no conditions",
				nil,
				true,
				false,
				PointTo(beConditionWithFalseStatusReasonAndMsg(gardencorev1beta1.ManagedResourceMissingConditionError, ""))),
			Entry("one true condition, one missing",
				[]gardencorev1beta1.Condition{
					{
						Type:   resourcesv1alpha1.ResourcesApplied,
						Status: gardencorev1beta1.ConditionTrue,
					},
				},
				true,
				false,
				PointTo(beConditionWithFalseStatusReasonAndMsg(gardencorev1beta1.ManagedResourceMissingConditionError, string(resourcesv1alpha1.ResourcesHealthy)))),
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
				PointTo(beConditionWithFalseStatusReasonAndMsg(gardencorev1beta1.ManagedResourceProgressingRolloutStuck, "ManagedResource  is progressing for more than 5m0s"))),
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
				PointTo(beConditionWithFalseStatusReasonAndMsg("barFailed", "bar is unhealthy"))),
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
				PointTo(beConditionWithFalseStatusReasonAndMsg("fooFailed", "foo is unhealthy"))),
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
				PointTo(beConditionWithFalseStatusReasonAndMsg(gardencorev1beta1.OutdatedStatusError, "outdated"))),
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
				PointTo(beConditionWithFalseStatusReasonAndMsg("Unknown", "bar is unknown"))),
		)

		var (
			valiStatefulSet = newStatefulSet(namespace, v1beta1constants.StatefulSetNameVali, v1beta1constants.GardenRoleLogging, true)

			requiredLoggingControlPlaneStatefulSets = []*appsv1.StatefulSet{
				valiStatefulSet,
			}

			eventLoggerDepployment = newDeployment(namespace, v1beta1constants.DeploymentNameEventLogger, v1beta1constants.GardenRoleLogging, true)

			requiredLoggingControlPlaneDeployments = []*appsv1.Deployment{
				eventLoggerDepployment,
			}
		)

		DescribeTable("#CheckLoggingControlPlane",
			func(deployments []*appsv1.Deployment, statefulSets []*appsv1.StatefulSet, eventLoggingEnabled, valiEnabled bool, conditionMatcher types.GomegaMatcher) {
				for _, obj := range deployments {
					Expect(fakeClient.Create(ctx, obj.DeepCopy())).To(Succeed(), "creating deployment "+client.ObjectKeyFromObject(obj).String())
				}
				for _, obj := range statefulSets {
					Expect(fakeClient.Create(ctx, obj.DeepCopy())).To(Succeed(), "creating statefulset "+client.ObjectKeyFromObject(obj).String())
				}

				checker := NewHealthChecker(fakeClient, fakeClock, map[gardencorev1beta1.ConditionType]time.Duration{}, nil)

				exitCondition, err := checker.CheckLoggingControlPlane(ctx, namespace, eventLoggingEnabled, valiEnabled, condition)
				Expect(err).NotTo(HaveOccurred())
				Expect(exitCondition).To(conditionMatcher)
			},
			Entry("all healthy",
				requiredLoggingControlPlaneDeployments,
				requiredLoggingControlPlaneStatefulSets,
				true,
				true,
				BeNil(),
			),
			Entry("required stateful set missing",
				requiredLoggingControlPlaneDeployments,
				nil,
				true,
				true,
				PointTo(beConditionWithStatus(gardencorev1beta1.ConditionFalse)),
			),
			Entry("required deployment is missing",
				nil,
				requiredLoggingControlPlaneStatefulSets,
				true,
				true,
				PointTo(beConditionWithStatus(gardencorev1beta1.ConditionFalse)),
			),
			Entry("stateful set unhealthy",
				requiredLoggingControlPlaneDeployments,
				[]*appsv1.StatefulSet{
					newStatefulSet(valiStatefulSet.Namespace, valiStatefulSet.Name, roleOf(valiStatefulSet), false),
				},
				true,
				true,
				PointTo(beConditionWithStatus(gardencorev1beta1.ConditionFalse)),
			),
			Entry("stateful set unhealthy",
				[]*appsv1.Deployment{
					newDeployment(eventLoggerDepployment.Namespace, eventLoggerDepployment.Name, roleOf(eventLoggerDepployment), false),
				},
				requiredLoggingControlPlaneStatefulSets,
				true,
				true,
				PointTo(beConditionWithStatus(gardencorev1beta1.ConditionFalse)),
			),
			Entry("vali is disabled in gardenlet config, omit stateful set check",
				requiredLoggingControlPlaneDeployments,
				[]*appsv1.StatefulSet{},
				true,
				false,
				BeNil(),
			),
			Entry("event logging is disabled in gardenlet config, omit deployment check",
				[]*appsv1.Deployment{},
				requiredLoggingControlPlaneStatefulSets,
				false,
				true,
				BeNil(),
			),
		)

		// CheckExtensionCondition
		DescribeTable("#CheckExtensionCondition - HealthCheckReport",
			func(healthCheckOutdatedThreshold *metav1.Duration, condition gardencorev1beta1.Condition, extensionsConditions []ExtensionCondition, expected types.GomegaMatcher) {
				checker := NewHealthChecker(fakeClient, fakeClock, nil, nil)
				updatedCondition := checker.CheckExtensionCondition(condition, extensionsConditions, healthCheckOutdatedThreshold)
				if expected == nil {
					Expect(updatedCondition).To(BeNil())
					return
				}
				Expect(updatedCondition).To(expected)
			},

			Entry("health check report is not outdated - threshold not configured in Gardenlet config",
				nil,
				gardencorev1beta1.Condition{Type: "type"},
				[]ExtensionCondition{
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
				[]ExtensionCondition{
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
				[]ExtensionCondition{
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
				[]ExtensionCondition{
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
				[]ExtensionCondition{
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
				[]ExtensionCondition{
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
				[]ExtensionCondition{
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

		var (
			plutonoDeployment               = newDeployment(namespace, v1beta1constants.DeploymentNamePlutono, v1beta1constants.GardenRoleMonitoring, true)
			kubeStateMetricsShootDeployment = newDeployment(namespace, v1beta1constants.DeploymentNameKubeStateMetrics, v1beta1constants.GardenRoleMonitoring, true)

			requiredMonitoringControlPlaneDeployments = []*appsv1.Deployment{
				plutonoDeployment,
				kubeStateMetricsShootDeployment,
			}

			alertManagerStatefulSet = newStatefulSet(namespace, v1beta1constants.StatefulSetNameAlertManager, v1beta1constants.GardenRoleMonitoring, true)
			prometheusStatefulSet   = newStatefulSet(namespace, v1beta1constants.StatefulSetNamePrometheus, v1beta1constants.GardenRoleMonitoring, true)

			requiredMonitoringControlPlaneStatefulSets = []*appsv1.StatefulSet{
				alertManagerStatefulSet,
				prometheusStatefulSet,
			}
		)

		DescribeTable("#CheckShootMonitoringControlPlane",
			func(deployments []*appsv1.Deployment, statefulSets []*appsv1.StatefulSet, workerless, wantsAlertmanager bool, conditionMatcher types.GomegaMatcher) {
				for _, obj := range deployments {
					Expect(fakeClient.Create(ctx, obj.DeepCopy())).To(Succeed(), "creating deployment "+client.ObjectKeyFromObject(obj).String())
				}
				for _, obj := range statefulSets {
					Expect(fakeClient.Create(ctx, obj.DeepCopy())).To(Succeed(), "creating statefulset "+client.ObjectKeyFromObject(obj).String())
				}

				checker := NewHealthChecker(fakeClient, fakeClock, nil, nil)

				exitCondition, err := checker.CheckMonitoringControlPlane(
					ctx,
					namespace,
					objectNameSet(requiredMonitoringControlPlaneDeployments),
					objectNameSet(requiredMonitoringControlPlaneStatefulSets),
					labels.SelectorFromSet(map[string]string{"gardener.cloud/role": "monitoring"}),
					condition,
				)
				Expect(err).NotTo(HaveOccurred())
				Expect(exitCondition).To(conditionMatcher)
			},
			Entry("all healthy",
				requiredMonitoringControlPlaneDeployments,
				requiredMonitoringControlPlaneStatefulSets,
				false,
				true,
				BeNil()),
			Entry("required deployment missing",
				[]*appsv1.Deployment{
					plutonoDeployment,
				},
				requiredMonitoringControlPlaneStatefulSets,
				false,
				true,
				PointTo(beConditionWithMissingRequiredDeployment([]*appsv1.Deployment{kubeStateMetricsShootDeployment}))),
			Entry("required stateful set set missing",
				requiredMonitoringControlPlaneDeployments,
				[]*appsv1.StatefulSet{
					prometheusStatefulSet,
				},
				false,
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
				PointTo(beConditionWithStatus(gardencorev1beta1.ConditionFalse))),
			Entry("stateful set unhealthy",
				requiredMonitoringControlPlaneDeployments,
				[]*appsv1.StatefulSet{
					newStatefulSet(alertManagerStatefulSet.Namespace, alertManagerStatefulSet.Name, roleOf(alertManagerStatefulSet), false),
					prometheusStatefulSet,
				},
				false,
				true,
				PointTo(beConditionWithStatus(gardencorev1beta1.ConditionFalse))),
		)
	})
})

func beConditionWithStatusReasonAndMessage(status gardencorev1beta1.ConditionStatus, reason, message string) types.GomegaMatcher {
	return And(WithStatus(status), WithReason(reason), WithMessage(message))
}

func beConditionWithStatus(status gardencorev1beta1.ConditionStatus) types.GomegaMatcher {
	return WithStatus(status)
}

func beConditionWithFalseStatusReasonAndMsg(reason, message string) types.GomegaMatcher {
	return And(WithStatus(gardencorev1beta1.ConditionFalse), WithReason(reason), WithMessage(message))
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

func objectNameSet[o client.Object](objs []o) sets.Set[string] {
	names := sets.New[string]()

	for _, obj := range objs {
		names.Insert(obj.GetName())
	}

	return names
}

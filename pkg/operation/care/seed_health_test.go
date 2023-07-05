// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	testclock "k8s.io/utils/clock/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component/clusterautoscaler"
	"github.com/gardener/gardener/pkg/component/clusteridentity"
	"github.com/gardener/gardener/pkg/component/dependencywatchdog"
	"github.com/gardener/gardener/pkg/component/etcd"
	"github.com/gardener/gardener/pkg/component/hvpa"
	"github.com/gardener/gardener/pkg/component/kubestatemetrics"
	"github.com/gardener/gardener/pkg/component/logging/fluentoperator"
	"github.com/gardener/gardener/pkg/component/nginxingress"
	"github.com/gardener/gardener/pkg/component/seedsystem"
	"github.com/gardener/gardener/pkg/component/vpa"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/operation/care"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var (
	requiredManagedResources = []string{
		etcd.Druid,
		clusteridentity.ManagedResourceControlName,
		clusterautoscaler.ManagedResourceControlName,
		kubestatemetrics.ManagedResourceName,
		nginxingress.ManagedResourceName,
		seedsystem.ManagedResourceName,
		vpa.ManagedResourceControlName,
		"istio",
	}

	optionalManagedResources = []string{
		dependencywatchdog.ManagedResourceDependencyWatchdogWeeder,
		dependencywatchdog.ManagedResourceDependencyWatchdogProber,
		hvpa.ManagedResourceName,
		"istio-system",
		fluentoperator.CustomResourcesManagedResourceName,
		fluentoperator.OperatorManagedResourceName,
	}
)

var _ = Describe("Seed health", func() {
	var (
		ctx       context.Context
		c         client.Client
		fakeClock *testclock.FakeClock

		seed *gardencorev1beta1.Seed

		seedSystemComponentsHealthyCondition gardencorev1beta1.Condition
	)

	BeforeEach(func() {
		defer test.WithFeatureGate(features.DefaultFeatureGate, features.HVPA, true)()

		ctx = context.TODO()
		c = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()

		seed = &gardencorev1beta1.Seed{
			ObjectMeta: metav1.ObjectMeta{
				Name: "foo",
			},
			Spec: gardencorev1beta1.SeedSpec{
				Ingress: &gardencorev1beta1.Ingress{
					Controller: gardencorev1beta1.IngressController{
						Kind: "nginx",
					},
				},
				Settings: &gardencorev1beta1.SeedSettings{
					DependencyWatchdog: &gardencorev1beta1.SeedSettingDependencyWatchdog{
						Endpoint: &gardencorev1beta1.SeedSettingDependencyWatchdogEndpoint{
							Enabled: true,
						},
						Probe: &gardencorev1beta1.SeedSettingDependencyWatchdogProbe{
							Enabled: true,
						},
						Weeder: &gardencorev1beta1.SeedSettingDependencyWatchdogWeeder{
							Enabled: true,
						},
						Prober: &gardencorev1beta1.SeedSettingDependencyWatchdogProber{
							Enabled: true,
						},
					},
				},
			},
		}

		fakeClock = testclock.NewFakeClock(time.Now())

		seedSystemComponentsHealthyCondition = gardencorev1beta1.Condition{
			Type:               gardencorev1beta1.SeedSystemComponentsHealthy,
			LastTransitionTime: metav1.Time{Time: fakeClock.Now()},
		}
	})

	Describe("#CheckSeed", func() {
		Context("When all managed resources are deployed successfully", func() {
			JustBeforeEach(func() {
				for _, name := range append(requiredManagedResources, optionalManagedResources...) {
					Expect(c.Create(ctx, healthyManagedResource(name))).To(Succeed())
				}
			})

			It("should set SeedSystemComponentsHealthy condition to true", func() {
				healthCheck := care.NewHealthForSeed(seed, c, fakeClock, nil, false, true)
				updatedConditions := healthCheck.CheckSeed(ctx, []gardencorev1beta1.Condition{seedSystemComponentsHealthyCondition}, nil)
				Expect(len(updatedConditions)).ToNot(BeZero())
				Expect(updatedConditions[0]).To(beConditionWithStatusReasonAndMessage(gardencorev1beta1.ConditionTrue, "SystemComponentsRunning", "All system components are healthy."))
			})
		})

		Context("When optional managed resources are turned off, and required resources are deployed successfully", func() {
			JustBeforeEach(func() {
				defer test.WithFeatureGate(features.DefaultFeatureGate, features.HVPA, false)()
				seed.Spec.Settings.DependencyWatchdog.Endpoint.Enabled = false
				seed.Spec.Settings.DependencyWatchdog.Probe.Enabled = false
				seed.Spec.Settings.DependencyWatchdog.Weeder.Enabled = false
				seed.Spec.Settings.DependencyWatchdog.Prober.Enabled = false

				for _, name := range requiredManagedResources {
					Expect(c.Create(ctx, healthyManagedResource(name))).To(Succeed())
				}
			})

			It("should set SeedSystemComponentsHealthy condition to true", func() {
				healthCheck := care.NewHealthForSeed(seed, c, fakeClock, nil, true, false)
				updatedConditions := healthCheck.CheckSeed(ctx, []gardencorev1beta1.Condition{seedSystemComponentsHealthyCondition}, nil)
				Expect(len(updatedConditions)).ToNot(BeZero())
				Expect(updatedConditions[0]).To(beConditionWithStatusReasonAndMessage(gardencorev1beta1.ConditionTrue, "SystemComponentsRunning", "All system components are healthy."))
			})
		})

		Context("When there are issues with seed managed resources", func() {
			var (
				tests = func(reason, message string) {
					It("should set SeedSystemComponentsHealthy condition to False if there is no Progressing threshold duration mapping", func() {
						healthCheck := care.NewHealthForSeed(seed, c, fakeClock, nil, false, false)
						updatedConditions := healthCheck.CheckSeed(ctx, []gardencorev1beta1.Condition{seedSystemComponentsHealthyCondition}, nil)

						Expect(len(updatedConditions)).ToNot(BeZero())
						Expect(updatedConditions[0]).To(beConditionWithStatusReasonAndMessage(gardencorev1beta1.ConditionFalse, reason, message))
					})

					It("should set SeedSystemComponentsHealthy condition to Progressing if time is within threshold duration and condition is currently False", func() {
						seedSystemComponentsHealthyCondition.Status = gardencorev1beta1.ConditionFalse
						fakeClock.Step(30 * time.Second)

						healthCheck := care.NewHealthForSeed(seed, c, fakeClock, nil, false, false)
						updatedConditions := healthCheck.CheckSeed(
							ctx,
							[]gardencorev1beta1.Condition{seedSystemComponentsHealthyCondition},
							map[gardencorev1beta1.ConditionType]time.Duration{gardencorev1beta1.SeedSystemComponentsHealthy: time.Minute},
						)

						Expect(len(updatedConditions)).ToNot(BeZero())
						Expect(updatedConditions[0]).To(beConditionWithStatusReasonAndMessage(gardencorev1beta1.ConditionProgressing, reason, message))
					})

					It("should set SeedSystemComponentsHealthy condition to Progressing if time is within threshold duration and condition is currently True", func() {
						seedSystemComponentsHealthyCondition.Status = gardencorev1beta1.ConditionTrue
						fakeClock.Step(30 * time.Second)

						healthCheck := care.NewHealthForSeed(seed, c, fakeClock, nil, false, false)
						updatedConditions := healthCheck.CheckSeed(
							ctx,
							[]gardencorev1beta1.Condition{seedSystemComponentsHealthyCondition},
							map[gardencorev1beta1.ConditionType]time.Duration{gardencorev1beta1.SeedSystemComponentsHealthy: time.Minute},
						)

						Expect(len(updatedConditions)).ToNot(BeZero())
						Expect(updatedConditions[0]).To(beConditionWithStatusReasonAndMessage(gardencorev1beta1.ConditionProgressing, reason, message))
					})

					It("should not set SeedSystemComponentsHealthy condition to false if Progressing threshold duration has not expired", func() {
						seedSystemComponentsHealthyCondition.Status = gardencorev1beta1.ConditionProgressing
						fakeClock.Step(30 * time.Second)

						healthCheck := care.NewHealthForSeed(seed, c, fakeClock, nil, false, false)
						updatedConditions := healthCheck.CheckSeed(
							ctx,
							[]gardencorev1beta1.Condition{seedSystemComponentsHealthyCondition},
							map[gardencorev1beta1.ConditionType]time.Duration{gardencorev1beta1.SeedSystemComponentsHealthy: time.Minute},
						)

						Expect(len(updatedConditions)).ToNot(BeZero())
						Expect(updatedConditions[0]).To(beConditionWithStatusReasonAndMessage(gardencorev1beta1.ConditionProgressing, reason, message))
					})

					It("should set SeedSystemComponentsHealthy condition to false if Progressing threshold duration has expired", func() {
						seedSystemComponentsHealthyCondition.Status = gardencorev1beta1.ConditionProgressing
						fakeClock.Step(90 * time.Second)

						healthCheck := care.NewHealthForSeed(seed, c, fakeClock, nil, false, false)
						updatedConditions := healthCheck.CheckSeed(
							ctx,
							[]gardencorev1beta1.Condition{seedSystemComponentsHealthyCondition},
							map[gardencorev1beta1.ConditionType]time.Duration{gardencorev1beta1.SeedSystemComponentsHealthy: time.Minute},
						)

						Expect(len(updatedConditions)).ToNot(BeZero())
						Expect(updatedConditions[0]).To(beConditionWithStatusReasonAndMessage(gardencorev1beta1.ConditionFalse, reason, message))
					})
				}
			)

			Context("When optional managed resources are enabled in seed settings but not deployed", func() {
				JustBeforeEach(func() {
					for _, name := range requiredManagedResources {
						Expect(c.Create(ctx, healthyManagedResource(name))).To(Succeed())
					}
				})

				tests("ResourceNotFound", "not found")
			})

			Context("When required managed resources are not deployed", func() {
				JustBeforeEach(func() {
					for _, name := range optionalManagedResources {
						Expect(c.Create(ctx, healthyManagedResource(name))).To(Succeed())
					}
				})

				tests("ResourceNotFound", "not found")
			})

			Context("When all managed resources are deployed, but not healthy", func() {
				JustBeforeEach(func() {
					for _, name := range append(requiredManagedResources, optionalManagedResources...) {
						Expect(c.Create(ctx, notHealthyManagedResource(name))).To(Succeed())
					}
				})

				tests("NotHealthy", "Resources are not healthy")
			})

			Context("When all managed resources are deployed but their resources are not applied", func() {
				JustBeforeEach(func() {
					for _, name := range append(requiredManagedResources, optionalManagedResources...) {
						Expect(c.Create(ctx, notAppliedManagedResource(name))).To(Succeed())
					}
				})

				tests("NotApplied", "Resources are not applied")
			})

			Context("When all managed resources are deployed but their resources are still progressing", func() {
				JustBeforeEach(func() {
					for _, name := range append(requiredManagedResources, optionalManagedResources...) {
						Expect(c.Create(ctx, progressingManagedResource(name))).To(Succeed())
					}
				})

				tests("ResourcesProgressing", "Resources are progressing")
			})

			Context("When all managed resources are deployed but not all required conditions are present", func() {
				JustBeforeEach(func() {
					for _, name := range append(requiredManagedResources, optionalManagedResources...) {
						Expect(c.Create(ctx, managedResource(name, []gardencorev1beta1.Condition{{
							Type:   resourcesv1alpha1.ResourcesApplied,
							Status: gardencorev1beta1.ConditionTrue}},
						))).To(Succeed())
					}
				})

				tests("MissingManagedResourceCondition", "is missing the following condition(s)")
			})
		})
	})
})

func beConditionWithStatusReasonAndMessage(status gardencorev1beta1.ConditionStatus, reason, message string) types.GomegaMatcher {
	return And(WithStatus(status), WithReason(reason), WithMessage(message))

}

func healthyManagedResource(name string) *resourcesv1alpha1.ManagedResource {
	return managedResource(
		name,
		[]gardencorev1beta1.Condition{
			{
				Type:   resourcesv1alpha1.ResourcesApplied,
				Status: gardencorev1beta1.ConditionTrue,
			},
			{
				Type:   resourcesv1alpha1.ResourcesHealthy,
				Status: gardencorev1beta1.ConditionTrue,
			},
			{
				Type:   resourcesv1alpha1.ResourcesProgressing,
				Status: gardencorev1beta1.ConditionFalse,
			},
		})
}

func notHealthyManagedResource(name string) *resourcesv1alpha1.ManagedResource {
	return managedResource(
		name,
		[]gardencorev1beta1.Condition{
			{
				Type:   resourcesv1alpha1.ResourcesApplied,
				Status: gardencorev1beta1.ConditionTrue,
			},
			{
				Type:    resourcesv1alpha1.ResourcesHealthy,
				Reason:  "NotHealthy",
				Message: "Resources are not healthy",
				Status:  gardencorev1beta1.ConditionFalse,
			},
			{
				Type:   resourcesv1alpha1.ResourcesProgressing,
				Status: gardencorev1beta1.ConditionFalse,
			},
		})
}

func notAppliedManagedResource(name string) *resourcesv1alpha1.ManagedResource {
	return managedResource(
		name,
		[]gardencorev1beta1.Condition{
			{
				Type:    resourcesv1alpha1.ResourcesApplied,
				Reason:  "NotApplied",
				Message: "Resources are not applied",
				Status:  gardencorev1beta1.ConditionFalse,
			},
			{
				Type:   resourcesv1alpha1.ResourcesHealthy,
				Status: gardencorev1beta1.ConditionTrue,
			},
			{
				Type:   resourcesv1alpha1.ResourcesProgressing,
				Status: gardencorev1beta1.ConditionFalse,
			},
		})
}

func progressingManagedResource(name string) *resourcesv1alpha1.ManagedResource {
	return managedResource(
		name,
		[]gardencorev1beta1.Condition{
			{
				Type:   resourcesv1alpha1.ResourcesApplied,
				Status: gardencorev1beta1.ConditionTrue,
			},
			{
				Type:   resourcesv1alpha1.ResourcesHealthy,
				Status: gardencorev1beta1.ConditionTrue,
			},
			{
				Type:    resourcesv1alpha1.ResourcesProgressing,
				Reason:  "ResourcesProgressing",
				Message: "Resources are progressing",
				Status:  gardencorev1beta1.ConditionTrue,
			},
		})
}

func managedResource(name string, conditions []gardencorev1beta1.Condition) *resourcesv1alpha1.ManagedResource {
	namespace := v1beta1constants.GardenNamespace
	if name == "istio-system" || strings.HasSuffix(name, "istio") {
		namespace = v1beta1constants.IstioSystemNamespace
	}

	return &resourcesv1alpha1.ManagedResource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Status: resourcesv1alpha1.ManagedResourceStatus{
			Conditions: conditions,
		},
	}
}

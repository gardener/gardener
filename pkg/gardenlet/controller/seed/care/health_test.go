// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/gardenlet/controller/seed/care"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
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
		ctx = context.Background()
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

	Describe("#Check", func() {
		managedResourceName := "foo"

		Context("When all managed resources are deployed successfully", func() {
			JustBeforeEach(func() {
				Expect(c.Create(ctx, healthyManagedResource(managedResourceName))).To(Succeed())
			})

			It("should set SeedSystemComponentsHealthy condition to true", func() {
				healthCheck := NewHealth(seed, c, fakeClock, nil, nil)
				conditions := NewSeedConditions(fakeClock, gardencorev1beta1.SeedStatus{
					Conditions: []gardencorev1beta1.Condition{seedSystemComponentsHealthyCondition},
				})

				updatedConditions := healthCheck.Check(ctx, conditions)
				Expect(updatedConditions).ToNot(BeEmpty())
				Expect(updatedConditions[0]).To(beConditionWithStatusReasonAndMessage(gardencorev1beta1.ConditionTrue, "SystemComponentsRunning", "All system components are healthy."))
			})
		})

		Context("When there are issues with seed managed resources", func() {
			var (
				tests = func(reason, message string) {
					It("should set SeedSystemComponentsHealthy condition to False if there is no Progressing threshold duration mapping", func() {
						healthCheck := NewHealth(seed, c, fakeClock, nil, nil)
						conditions := NewSeedConditions(fakeClock, gardencorev1beta1.SeedStatus{
							Conditions: []gardencorev1beta1.Condition{seedSystemComponentsHealthyCondition},
						})

						updatedConditions := healthCheck.Check(ctx, conditions)

						Expect(updatedConditions).ToNot(BeEmpty())
						Expect(updatedConditions[0]).To(beConditionWithStatusReasonAndMessage(gardencorev1beta1.ConditionFalse, reason, message))
					})

					It("should set SeedSystemComponentsHealthy condition to Progressing if time is within threshold duration and condition is currently False", func() {
						seedSystemComponentsHealthyCondition.Status = gardencorev1beta1.ConditionFalse
						fakeClock.Step(30 * time.Second)

						healthCheck := NewHealth(seed, c, fakeClock, nil, map[gardencorev1beta1.ConditionType]time.Duration{gardencorev1beta1.SeedSystemComponentsHealthy: time.Minute})
						conditions := NewSeedConditions(fakeClock, gardencorev1beta1.SeedStatus{
							Conditions: []gardencorev1beta1.Condition{seedSystemComponentsHealthyCondition},
						})

						updatedConditions := healthCheck.Check(ctx, conditions)

						Expect(updatedConditions).ToNot(BeEmpty())
						Expect(updatedConditions[0]).To(beConditionWithStatusReasonAndMessage(gardencorev1beta1.ConditionProgressing, reason, message))
					})

					It("should set SeedSystemComponentsHealthy condition to Progressing if time is within threshold duration and condition is currently True", func() {
						seedSystemComponentsHealthyCondition.Status = gardencorev1beta1.ConditionTrue
						fakeClock.Step(30 * time.Second)

						healthCheck := NewHealth(seed, c, fakeClock, nil, map[gardencorev1beta1.ConditionType]time.Duration{gardencorev1beta1.SeedSystemComponentsHealthy: time.Minute})
						conditions := NewSeedConditions(fakeClock, gardencorev1beta1.SeedStatus{
							Conditions: []gardencorev1beta1.Condition{seedSystemComponentsHealthyCondition},
						})

						updatedConditions := healthCheck.Check(ctx, conditions)

						Expect(updatedConditions).ToNot(BeEmpty())
						Expect(updatedConditions[0]).To(beConditionWithStatusReasonAndMessage(gardencorev1beta1.ConditionProgressing, reason, message))
					})

					It("should not set SeedSystemComponentsHealthy condition to false if Progressing threshold duration has not expired", func() {
						seedSystemComponentsHealthyCondition.Status = gardencorev1beta1.ConditionProgressing
						fakeClock.Step(30 * time.Second)

						healthCheck := NewHealth(seed, c, fakeClock, nil, map[gardencorev1beta1.ConditionType]time.Duration{gardencorev1beta1.SeedSystemComponentsHealthy: time.Minute})
						conditions := NewSeedConditions(fakeClock, gardencorev1beta1.SeedStatus{
							Conditions: []gardencorev1beta1.Condition{seedSystemComponentsHealthyCondition},
						})

						updatedConditions := healthCheck.Check(ctx, conditions)

						Expect(updatedConditions).ToNot(BeEmpty())
						Expect(updatedConditions[0]).To(beConditionWithStatusReasonAndMessage(gardencorev1beta1.ConditionProgressing, reason, message))
					})

					It("should set SeedSystemComponentsHealthy condition to false if Progressing threshold duration has expired", func() {
						seedSystemComponentsHealthyCondition.Status = gardencorev1beta1.ConditionProgressing
						fakeClock.Step(90 * time.Second)

						healthCheck := NewHealth(seed, c, fakeClock, nil, map[gardencorev1beta1.ConditionType]time.Duration{gardencorev1beta1.SeedSystemComponentsHealthy: time.Minute})
						conditions := NewSeedConditions(fakeClock, gardencorev1beta1.SeedStatus{
							Conditions: []gardencorev1beta1.Condition{seedSystemComponentsHealthyCondition},
						})

						updatedConditions := healthCheck.Check(ctx, conditions)

						Expect(updatedConditions).ToNot(BeEmpty())
						Expect(updatedConditions[0]).To(beConditionWithStatusReasonAndMessage(gardencorev1beta1.ConditionFalse, reason, message))
					})
				}
			)

			Context("When all managed resources are deployed, but not healthy", func() {
				JustBeforeEach(func() {
					Expect(c.Create(ctx, notHealthyManagedResource(managedResourceName))).To(Succeed())
				})

				tests("NotHealthy", "Resources are not healthy")
			})

			Context("When all managed resources are deployed but their resources are not applied", func() {
				JustBeforeEach(func() {
					Expect(c.Create(ctx, notAppliedManagedResource(managedResourceName))).To(Succeed())
				})

				tests("NotApplied", "Resources are not applied")
			})

			Context("When all managed resources are deployed but their resources are still progressing", func() {
				JustBeforeEach(func() {
					Expect(c.Create(ctx, progressingManagedResource(managedResourceName))).To(Succeed())
				})

				tests("ResourcesProgressing", "Resources are progressing")
			})

			Context("When all managed resources are deployed but not all required conditions are present", func() {
				JustBeforeEach(func() {
					Expect(c.Create(ctx, managedResource(managedResourceName, []gardencorev1beta1.Condition{{
						Type:   resourcesv1alpha1.ResourcesApplied,
						Status: gardencorev1beta1.ConditionTrue}},
					))).To(Succeed())
				})

				tests("MissingManagedResourceCondition", "is missing the following condition(s)")
			})
		})
	})

	Describe("SeedConditions", func() {
		Describe("#NewSeedConditions", func() {
			It("should initialize all conditions", func() {
				conditions := NewSeedConditions(fakeClock, gardencorev1beta1.SeedStatus{})

				Expect(conditions.ConvertToSlice()).To(ConsistOf(
					beConditionWithStatusReasonAndMessage("Unknown", "ConditionInitialized", "The condition has been initialized but its semantic check has not been performed yet."),
				))
			})

			It("should only initialize missing conditions", func() {
				conditions := NewSeedConditions(fakeClock, gardencorev1beta1.SeedStatus{
					Conditions: []gardencorev1beta1.Condition{
						{Type: "SeedSystemComponentsHealthy"},
						{Type: "Foo"},
					},
				})

				Expect(conditions.ConvertToSlice()).To(HaveExactElements(
					OfType("SeedSystemComponentsHealthy"),
				))
			})
		})

		Describe("#ConvertToSlice", func() {
			It("should return the expected conditions", func() {
				conditions := NewSeedConditions(fakeClock, gardencorev1beta1.SeedStatus{})

				Expect(conditions.ConvertToSlice()).To(HaveExactElements(
					OfType("SeedSystemComponentsHealthy"),
				))
			})
		})

		Describe("#ConditionTypes", func() {
			It("should return the expected condition types", func() {
				conditions := NewSeedConditions(fakeClock, gardencorev1beta1.SeedStatus{})

				Expect(conditions.ConditionTypes()).To(HaveExactElements(
					gardencorev1beta1.ConditionType("SeedSystemComponentsHealthy"),
				))
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
		Spec: resourcesv1alpha1.ManagedResourceSpec{
			Class: ptr.To("seed"),
		},
		Status: resourcesv1alpha1.ManagedResourceStatus{
			Conditions: conditions,
		},
	}
}

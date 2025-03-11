// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package care_test

import (
	"context"
	"strings"
	"time"

	druidcorev1alpha1 "github.com/gardener/etcd-druid/api/core/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	testclock "k8s.io/utils/clock/testing"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
	. "github.com/gardener/gardener/pkg/operator/controller/garden/care"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var (
	virtualGardenDeployments = []string{
		"virtual-garden-gardener-resource-manager",
		"virtual-garden-kube-apiserver",
		"virtual-garden-kube-controller-manager",
	}

	virtualGardenETCDs = []string{
		"virtual-garden-etcd-events",
		"virtual-garden-etcd-main",
	}
)

var _ = Describe("Garden health", func() {
	var (
		ctx             context.Context
		runtimeClient   client.Client
		gardenClientSet kubernetes.Interface
		fakeClock       *testclock.FakeClock

		garden           *operatorv1alpha1.Garden
		gardenNamespace  string
		gardenConditions GardenConditions

		apiserverAvailabilityCondition          gardencorev1beta1.Condition
		runtimeComponentsHealthyCondition       gardencorev1beta1.Condition
		virtualComponentsHealthyCondition       gardencorev1beta1.Condition
		observabilityComponentsHealthyCondition gardencorev1beta1.Condition
	)

	BeforeEach(func() {
		ctx = context.Background()
		runtimeClient = fakeclient.NewClientBuilder().WithScheme(operatorclient.RuntimeScheme).Build()

		garden = &operatorv1alpha1.Garden{
			ObjectMeta: metav1.ObjectMeta{
				Name: "foo",
			},
		}
		gardenNamespace = "garden"

		fakeClock = testclock.NewFakeClock(time.Now())

		apiserverAvailabilityCondition = gardencorev1beta1.Condition{
			Type:               operatorv1alpha1.VirtualGardenAPIServerAvailable,
			LastTransitionTime: metav1.Time{Time: fakeClock.Now()},
		}
		runtimeComponentsHealthyCondition = gardencorev1beta1.Condition{
			Type:               operatorv1alpha1.RuntimeComponentsHealthy,
			LastTransitionTime: metav1.Time{Time: fakeClock.Now()},
		}
		virtualComponentsHealthyCondition = gardencorev1beta1.Condition{
			Type:               operatorv1alpha1.VirtualComponentsHealthy,
			LastTransitionTime: metav1.Time{Time: fakeClock.Now()},
		}
		observabilityComponentsHealthyCondition = gardencorev1beta1.Condition{
			Type:               operatorv1alpha1.ObservabilityComponentsHealthy,
			LastTransitionTime: metav1.Time{Time: fakeClock.Now()},
		}
	})

	JustBeforeEach(func() {
		garden.Status = operatorv1alpha1.GardenStatus{
			Conditions: []gardencorev1beta1.Condition{
				apiserverAvailabilityCondition,
				runtimeComponentsHealthyCondition,
				virtualComponentsHealthyCondition,
				observabilityComponentsHealthyCondition,
			},
		}

		gardenConditions = NewGardenConditions(fakeClock, garden.Status)
	})

	Describe("#Check", func() {
		Context("when all managed resources, deployments and ETCDs are deployed successfully", func() {
			JustBeforeEach(func() {
				Expect(runtimeClient.Create(ctx, healthyManagedResource("foo", "RuntimeComponentsHealthy"))).To(Succeed())
				Expect(runtimeClient.Create(ctx, healthyManagedResource("bar", "VirtualComponentsHealthy"))).To(Succeed())
				Expect(runtimeClient.Create(ctx, healthyManagedResource("baz", "ObservabilityComponentsRunning"))).To(Succeed())

				for _, name := range virtualGardenDeployments {
					Expect(runtimeClient.Create(ctx, newDeployment(gardenNamespace, name, true))).To(Succeed())
				}
				for _, name := range virtualGardenETCDs {
					Expect(runtimeClient.Create(ctx, newEtcd(gardenNamespace, name, true))).To(Succeed())
				}
			})

			It("should set RuntimeComponentsHealthy and VirtualComponentsHealthy conditions to true", func() {
				updatedConditions := NewHealth(
					garden,
					runtimeClient,
					gardenClientSet,
					fakeClock,
					nil,
					gardenNamespace,
				).Check(ctx, gardenConditions)

				Expect(updatedConditions).ToNot(BeEmpty())
				Expect(updatedConditions).To(ContainElements(
					beConditionWithStatusReasonAndMessage(gardencorev1beta1.ConditionTrue, "RuntimeComponentsRunning", "All runtime components are healthy."),
					beConditionWithStatusReasonAndMessage(gardencorev1beta1.ConditionTrue, "VirtualComponentsRunning", "All virtual garden components are healthy."),
					beConditionWithStatusReasonAndMessage(gardencorev1beta1.ConditionTrue, "ObservabilityComponentsRunning", "All observability components are healthy."),
				))
			})
		})

		Context("when there are issues with runtime and virtual managed resources", func() {
			var (
				tests = func(reason, message string) {
					It("should set RuntimeComponentsHealthy and VirtualComponentsHealthy conditions to False if there is no Progressing threshold duration mapping", func() {
						updatedConditions := NewHealth(
							garden,
							runtimeClient,
							gardenClientSet,
							fakeClock,
							nil,
							gardenNamespace,
						).Check(ctx, gardenConditions)

						Expect(updatedConditions).ToNot(BeEmpty())
						Expect(updatedConditions).To(ContainElements(
							beConditionWithStatusReasonAndMessage(gardencorev1beta1.ConditionFalse, reason, message),
							beConditionWithStatusReasonAndMessage(gardencorev1beta1.ConditionFalse, reason, message),
						))
					})

					Context("condition is currently False", func() {
						BeforeEach(func() {
							runtimeComponentsHealthyCondition.Status = gardencorev1beta1.ConditionFalse
							virtualComponentsHealthyCondition.Status = gardencorev1beta1.ConditionFalse
						})

						It("should set RuntimeComponentsHealthy and VirtualComponentsHealthy conditions to Progressing if time is within threshold duration", func() {
							fakeClock.Step(30 * time.Second)

							updatedConditions := NewHealth(
								garden,
								runtimeClient,
								gardenClientSet,
								fakeClock,
								map[gardencorev1beta1.ConditionType]time.Duration{
									operatorv1alpha1.RuntimeComponentsHealthy: time.Minute,
									operatorv1alpha1.VirtualComponentsHealthy: time.Minute,
								},
								gardenNamespace,
							).Check(ctx, gardenConditions)

							Expect(updatedConditions).ToNot(BeEmpty())
							Expect(updatedConditions).To(ContainElements(
								beConditionWithStatusReasonAndMessage(gardencorev1beta1.ConditionProgressing, reason, message),
								beConditionWithStatusReasonAndMessage(gardencorev1beta1.ConditionProgressing, reason, message),
							))
						})
					})

					Context("condition is currently True", func() {
						BeforeEach(func() {
							runtimeComponentsHealthyCondition.Status = gardencorev1beta1.ConditionTrue
							virtualComponentsHealthyCondition.Status = gardencorev1beta1.ConditionTrue
						})

						It("should set RuntimeComponentsHealthy and VirtualComponentsHealthy conditions to Progressing if time is within threshold duration", func() {
							fakeClock.Step(30 * time.Second)

							updatedConditions := NewHealth(
								garden,
								runtimeClient,
								gardenClientSet,
								fakeClock,
								map[gardencorev1beta1.ConditionType]time.Duration{
									operatorv1alpha1.RuntimeComponentsHealthy: time.Minute,
									operatorv1alpha1.VirtualComponentsHealthy: time.Minute,
								},
								gardenNamespace,
							).Check(ctx, gardenConditions)

							Expect(updatedConditions).ToNot(BeEmpty())
							Expect(updatedConditions).To(ContainElements(
								beConditionWithStatusReasonAndMessage(gardencorev1beta1.ConditionProgressing, reason, message),
								beConditionWithStatusReasonAndMessage(gardencorev1beta1.ConditionProgressing, reason, message),
							))
						})
					})

					Context("condition is currently Progressing", func() {
						BeforeEach(func() {
							runtimeComponentsHealthyCondition.Status = gardencorev1beta1.ConditionProgressing
							virtualComponentsHealthyCondition.Status = gardencorev1beta1.ConditionProgressing
						})

						It("should not set RuntimeComponentsHealthy and VirtualComponentsHealthy conditions to Progressing if Progressing threshold duration has not expired", func() {
							fakeClock.Step(30 * time.Second)

							updatedConditions := NewHealth(
								garden,
								runtimeClient,
								gardenClientSet,
								fakeClock,
								map[gardencorev1beta1.ConditionType]time.Duration{
									operatorv1alpha1.RuntimeComponentsHealthy: time.Minute,
									operatorv1alpha1.VirtualComponentsHealthy: time.Minute,
								},
								gardenNamespace,
							).Check(ctx, gardenConditions)

							Expect(updatedConditions).ToNot(BeEmpty())
							Expect(updatedConditions).To(ContainElements(
								beConditionWithStatusReasonAndMessage(gardencorev1beta1.ConditionProgressing, reason, message),
								beConditionWithStatusReasonAndMessage(gardencorev1beta1.ConditionProgressing, reason, message),
							))
						})

						It("should set RuntimeComponentsHealthy and VirtualComponentsHealthy conditions to false if Progressing threshold duration has expired", func() {
							fakeClock.Step(90 * time.Second)

							updatedConditions := NewHealth(
								garden,
								runtimeClient,
								gardenClientSet,
								fakeClock,
								map[gardencorev1beta1.ConditionType]time.Duration{
									operatorv1alpha1.RuntimeComponentsHealthy: time.Minute,
									operatorv1alpha1.VirtualComponentsHealthy: time.Minute,
								},
								gardenNamespace,
							).Check(ctx, gardenConditions)

							Expect(updatedConditions).ToNot(BeEmpty())
							Expect(updatedConditions).To(ContainElements(
								beConditionWithStatusReasonAndMessage(gardencorev1beta1.ConditionFalse, reason, message),
								beConditionWithStatusReasonAndMessage(gardencorev1beta1.ConditionFalse, reason, message),
							))
						})
					})
				}
			)

			Context("when all managed resources are unhealthy", func() {
				JustBeforeEach(func() {
					Expect(runtimeClient.Create(ctx, unhealthyManagedResource("foo", "RuntimeComponentsHealthy"))).To(Succeed())
					Expect(runtimeClient.Create(ctx, unhealthyManagedResource("bar", "VirtualComponentsHealthy"))).To(Succeed())

					for _, name := range virtualGardenDeployments {
						Expect(runtimeClient.Create(ctx, newDeployment(gardenNamespace, name, true))).To(Succeed())
					}
					for _, name := range virtualGardenETCDs {
						Expect(runtimeClient.Create(ctx, newEtcd(gardenNamespace, name, true))).To(Succeed())
					}
				})

				tests("NotHealthy", "Resources are not healthy")
			})

			Context("when all managed resources are not applied", func() {
				JustBeforeEach(func() {
					Expect(runtimeClient.Create(ctx, unappliedManagedResource("foo", "RuntimeComponentsHealthy"))).To(Succeed())
					Expect(runtimeClient.Create(ctx, unappliedManagedResource("bar", "VirtualComponentsHealthy"))).To(Succeed())

					for _, name := range virtualGardenDeployments {
						Expect(runtimeClient.Create(ctx, newDeployment(gardenNamespace, name, true))).To(Succeed())
					}
					for _, name := range virtualGardenETCDs {
						Expect(runtimeClient.Create(ctx, newEtcd(gardenNamespace, name, true))).To(Succeed())
					}
				})

				tests("NotApplied", "Resources are not applied")
			})

			Context("when all managed resources are still progressing", func() {
				JustBeforeEach(func() {
					Expect(runtimeClient.Create(ctx, progressingManagedResource("foo", "RuntimeComponentsHealthy"))).To(Succeed())
					Expect(runtimeClient.Create(ctx, progressingManagedResource("bar", "VirtualComponentsHealthy"))).To(Succeed())

					for _, name := range virtualGardenDeployments {
						Expect(runtimeClient.Create(ctx, newDeployment(gardenNamespace, name, true))).To(Succeed())
					}
					for _, name := range virtualGardenETCDs {
						Expect(runtimeClient.Create(ctx, newEtcd(gardenNamespace, name, true))).To(Succeed())
					}
				})

				tests("ResourcesProgressing", "Resources are progressing")
			})

			Context("when all managed resources are deployed but not all required conditions are present", func() {
				JustBeforeEach(func() {
					Expect(runtimeClient.Create(ctx, managedResource("foo", "RuntimeComponentsHealthy", []gardencorev1beta1.Condition{{
						Type:   resourcesv1alpha1.ResourcesApplied,
						Status: gardencorev1beta1.ConditionTrue}},
					))).To(Succeed())
					Expect(runtimeClient.Create(ctx, managedResource("bar", "VirtualComponentsHealthy", []gardencorev1beta1.Condition{{
						Type:   resourcesv1alpha1.ResourcesApplied,
						Status: gardencorev1beta1.ConditionTrue}},
					))).To(Succeed())

					for _, name := range virtualGardenDeployments {
						Expect(runtimeClient.Create(ctx, newDeployment(gardenNamespace, name, true))).To(Succeed())
					}
					for _, name := range virtualGardenETCDs {
						Expect(runtimeClient.Create(ctx, newEtcd(gardenNamespace, name, true))).To(Succeed())
					}
				})

				tests("MissingManagedResourceCondition", "is missing the following condition(s)")
			})

			Context("when there are issues with deployments for virtual garden", func() {
				It("should set VirtualComponentsHealthy conditions to false when the deployments are missing", func() {
					updatedConditions := NewHealth(
						garden,
						runtimeClient,
						gardenClientSet,
						fakeClock,
						nil,
						gardenNamespace,
					).Check(ctx, gardenConditions)

					Expect(updatedConditions).ToNot(BeEmpty())
					Expect(updatedConditions).To(ContainElements(
						beConditionWithStatusReasonAndMessage(gardencorev1beta1.ConditionFalse, "DeploymentMissing", "Missing required deployments: [virtual-garden-gardener-resource-manager virtual-garden-kube-apiserver virtual-garden-kube-controller-manager]"),
					))
				})

				It("should set VirtualComponentsHealthy conditions to false when the deployments are existing but unhealthy", func() {
					for _, name := range virtualGardenDeployments {
						Expect(runtimeClient.Create(ctx, newDeployment(gardenNamespace, name, false))).To(Succeed())
					}

					updatedConditions := NewHealth(
						garden,
						runtimeClient,
						gardenClientSet,
						fakeClock,
						nil,
						gardenNamespace,
					).Check(ctx, gardenConditions)

					Expect(updatedConditions).ToNot(BeEmpty())
					Expect(updatedConditions).To(ContainElements(
						beConditionWithStatusReasonAndMessage(gardencorev1beta1.ConditionFalse, "DeploymentUnhealthy", "is unhealthy: condition \"Available\" is missing"),
					))
				})
			})

			Context("when there are issues with ETCDs for virtual garden", func() {
				JustBeforeEach(func() {
					for _, name := range virtualGardenDeployments {
						Expect(runtimeClient.Create(ctx, newDeployment(gardenNamespace, name, true))).To(Succeed())
					}
				})

				It("should set VirtualComponentsHealthy conditions to false when the ETCDs are missing", func() {
					updatedConditions := NewHealth(
						garden,
						runtimeClient,
						gardenClientSet,
						fakeClock,
						nil,
						gardenNamespace,
					).Check(ctx, gardenConditions)

					Expect(updatedConditions).ToNot(BeEmpty())
					Expect(updatedConditions).To(ContainElements(
						beConditionWithStatusReasonAndMessage(gardencorev1beta1.ConditionFalse, "EtcdMissing", "Missing required etcds: [virtual-garden-etcd-events virtual-garden-etcd-main]"),
					))
				})

				It("should set VirtualComponentsHealthy conditions to false when the ETCDs are existing but unhealthy", func() {
					for _, name := range virtualGardenETCDs {
						Expect(runtimeClient.Create(ctx, newEtcd(gardenNamespace, name, false))).To(Succeed())
					}

					updatedConditions := NewHealth(
						garden,
						runtimeClient,
						gardenClientSet,
						fakeClock,
						nil,
						gardenNamespace,
					).Check(ctx, gardenConditions)

					Expect(updatedConditions).ToNot(BeEmpty())
					Expect(updatedConditions).To(ContainElements(
						beConditionWithStatusReasonAndMessage(gardencorev1beta1.ConditionFalse, "EtcdUnhealthy", "Etcd extension resource \"virtual-garden-etcd-events\" is unhealthy: etcd \"virtual-garden-etcd-events\" is not ready yet"),
					))
				})
			})

			Context("when there are issues with observability components", func() {
				It("should set ObservabilityComponentsHealthy conditions to false when ManagedResources are unhealthy", func() {
					Expect(runtimeClient.Create(ctx, unhealthyManagedResource("baz", "ObservabilityComponentsHealthy"))).To(Succeed())

					updatedConditions := NewHealth(
						garden,
						runtimeClient,
						gardenClientSet,
						fakeClock,
						nil,
						gardenNamespace,
					).Check(ctx, gardenConditions)

					Expect(updatedConditions).ToNot(BeEmpty())
					Expect(updatedConditions).To(ContainCondition(OfType(operatorv1alpha1.ObservabilityComponentsHealthy), WithReason("NotHealthy")))
				})

				It("should set ObservabilityComponentsHealthy conditions to false when ManagedResources are not applied", func() {
					Expect(runtimeClient.Create(ctx, unappliedManagedResource("baz", "ObservabilityComponentsHealthy"))).To(Succeed())

					updatedConditions := NewHealth(
						garden,
						runtimeClient,
						gardenClientSet,
						fakeClock,
						nil,
						gardenNamespace,
					).Check(ctx, gardenConditions)

					Expect(updatedConditions).ToNot(BeEmpty())
					Expect(updatedConditions).To(ContainCondition(OfType(operatorv1alpha1.ObservabilityComponentsHealthy), WithReason("NotApplied")))
				})

				It("should set ObservabilityComponentsHealthy conditions to false when ManagedResources are progressing", func() {
					Expect(runtimeClient.Create(ctx, progressingManagedResource("baz", "ObservabilityComponentsHealthy"))).To(Succeed())

					updatedConditions := NewHealth(
						garden,
						runtimeClient,
						gardenClientSet,
						fakeClock,
						nil,
						gardenNamespace,
					).Check(ctx, gardenConditions)

					Expect(updatedConditions).ToNot(BeEmpty())
					Expect(updatedConditions).To(ContainCondition(OfType(operatorv1alpha1.ObservabilityComponentsHealthy), WithReason("ResourcesProgressing")))
				})

				It("should set ObservabilityComponentsHealthy conditions to false when some ManagedResources conditions are missing", func() {
					Expect(runtimeClient.Create(ctx, managedResource("baz", "ObservabilityComponentsHealthy", []gardencorev1beta1.Condition{{
						Type:   resourcesv1alpha1.ResourcesApplied,
						Status: gardencorev1beta1.ConditionTrue}},
					))).To(Succeed())

					updatedConditions := NewHealth(
						garden,
						runtimeClient,
						gardenClientSet,
						fakeClock,
						nil,
						gardenNamespace,
					).Check(ctx, gardenConditions)

					Expect(updatedConditions).ToNot(BeEmpty())
					Expect(updatedConditions).To(ContainCondition(OfType(operatorv1alpha1.ObservabilityComponentsHealthy), WithReason("MissingManagedResourceCondition")))
				})
			})
		})
	})

	Describe("GardenConditions", func() {
		Describe("#NewGardenConditions", func() {
			It("should initialize all conditions", func() {
				conditions := NewGardenConditions(fakeClock, operatorv1alpha1.GardenStatus{})

				Expect(conditions.ConvertToSlice()).To(ConsistOf(
					beConditionWithStatusReasonAndMessage("Unknown", "ConditionInitialized", "The condition has been initialized but its semantic check has not been performed yet."),
					beConditionWithStatusReasonAndMessage("Unknown", "ConditionInitialized", "The condition has been initialized but its semantic check has not been performed yet."),
					beConditionWithStatusReasonAndMessage("Unknown", "ConditionInitialized", "The condition has been initialized but its semantic check has not been performed yet."),
					beConditionWithStatusReasonAndMessage("Unknown", "ConditionInitialized", "The condition has been initialized but its semantic check has not been performed yet."),
				))
			})

			It("should only initialize missing conditions", func() {
				conditions := NewGardenConditions(fakeClock, operatorv1alpha1.GardenStatus{
					Conditions: []gardencorev1beta1.Condition{
						{Type: "VirtualGardenAPIServerAvailable"},
						{Type: "Foo"},
					},
				})

				Expect(conditions.ConvertToSlice()).To(HaveExactElements(
					OfType("VirtualGardenAPIServerAvailable"),
					beConditionWithStatusReasonAndMessage("Unknown", "ConditionInitialized", "The condition has been initialized but its semantic check has not been performed yet."),
					beConditionWithStatusReasonAndMessage("Unknown", "ConditionInitialized", "The condition has been initialized but its semantic check has not been performed yet."),
					beConditionWithStatusReasonAndMessage("Unknown", "ConditionInitialized", "The condition has been initialized but its semantic check has not been performed yet."),
				))
			})
		})

		Describe("#ConvertToSlice", func() {
			It("should return the expected conditions", func() {
				conditions := NewGardenConditions(fakeClock, operatorv1alpha1.GardenStatus{})

				Expect(conditions.ConvertToSlice()).To(HaveExactElements(
					OfType("VirtualGardenAPIServerAvailable"),
					OfType("RuntimeComponentsHealthy"),
					OfType("VirtualComponentsHealthy"),
					OfType("ObservabilityComponentsHealthy"),
				))
			})
		})

		Describe("#ConditionTypes", func() {
			It("should return the expected condition types", func() {
				conditions := NewGardenConditions(fakeClock, operatorv1alpha1.GardenStatus{})

				Expect(conditions.ConditionTypes()).To(HaveExactElements(
					gardencorev1beta1.ConditionType("VirtualGardenAPIServerAvailable"),
					gardencorev1beta1.ConditionType("RuntimeComponentsHealthy"),
					gardencorev1beta1.ConditionType("VirtualComponentsHealthy"),
					gardencorev1beta1.ConditionType("ObservabilityComponentsHealthy"),
				))
			})
		})
	})
})

func beConditionWithStatusReasonAndMessage(status gardencorev1beta1.ConditionStatus, reason, message string) types.GomegaMatcher {
	return And(WithStatus(status), WithReason(reason), WithMessage(message))
}

func healthyManagedResource(name string, relevantCareCondition string) *resourcesv1alpha1.ManagedResource {
	return managedResource(
		name,
		relevantCareCondition,
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

func unhealthyManagedResource(name string, relevantCareCondition string) *resourcesv1alpha1.ManagedResource {
	return managedResource(
		name,
		relevantCareCondition,
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

func unappliedManagedResource(name string, relevantCareCondition string) *resourcesv1alpha1.ManagedResource {
	return managedResource(
		name,
		relevantCareCondition,
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

func progressingManagedResource(name string, relevantCareCondition string) *resourcesv1alpha1.ManagedResource {
	return managedResource(
		name,
		relevantCareCondition,
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

func managedResource(name string, relevantCareCondition string, conditions []gardencorev1beta1.Condition) *resourcesv1alpha1.ManagedResource {
	namespace := v1beta1constants.GardenNamespace
	if name == "istio-system" || strings.HasSuffix(name, "istio") {
		namespace = v1beta1constants.IstioSystemNamespace
	}

	var (
		class  *string
		labels map[string]string
	)

	if relevantCareCondition != "" {
		if relevantCareCondition == "RuntimeComponentsHealthy" {
			class = ptr.To("seed")
		} else {
			labels = map[string]string{"care.gardener.cloud/condition-type": relevantCareCondition}
		}
	}

	return &resourcesv1alpha1.ManagedResource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: resourcesv1alpha1.ManagedResourceSpec{
			Class: class,
		},
		Status: resourcesv1alpha1.ManagedResourceStatus{
			Conditions: conditions,
		},
	}
}

func roleLabels(role string) map[string]string {
	return map[string]string{v1beta1constants.GardenRole: role}
}

func newDeployment(namespace, name string, healthy bool) *appsv1.Deployment {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
			Labels:    roleLabels("controlplane"),
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

func newEtcd(namespace, name string, healthy bool) *druidcorev1alpha1.Etcd {
	return &druidcorev1alpha1.Etcd{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
			Labels:    roleLabels("controlplane"),
		},
		Status: druidcorev1alpha1.EtcdStatus{
			Ready: ptr.To(healthy),
		},
	}
}

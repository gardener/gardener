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

package care_test

import (
	"context"
	"strings"
	"time"

	druidv1alpha1 "github.com/gardener/etcd-druid/api/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	testclock "k8s.io/utils/clock/testing"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component/etcd"
	"github.com/gardener/gardener/pkg/component/gardeneraccess"
	"github.com/gardener/gardener/pkg/component/gardeneradmissioncontroller"
	"github.com/gardener/gardener/pkg/component/gardenerapiserver"
	"github.com/gardener/gardener/pkg/component/gardenercontrollermanager"
	"github.com/gardener/gardener/pkg/component/gardenermetricsexporter"
	"github.com/gardener/gardener/pkg/component/gardenerscheduler"
	runtimegardensystem "github.com/gardener/gardener/pkg/component/gardensystem/runtime"
	virtualgardensystem "github.com/gardener/gardener/pkg/component/gardensystem/virtual"
	"github.com/gardener/gardener/pkg/component/hvpa"
	"github.com/gardener/gardener/pkg/component/kubecontrollermanager"
	"github.com/gardener/gardener/pkg/component/kubestatemetrics"
	"github.com/gardener/gardener/pkg/component/logging/fluentoperator"
	"github.com/gardener/gardener/pkg/component/logging/vali"
	"github.com/gardener/gardener/pkg/component/monitoring/prometheusoperator"
	"github.com/gardener/gardener/pkg/component/plutono"
	"github.com/gardener/gardener/pkg/component/resourcemanager"
	"github.com/gardener/gardener/pkg/component/vpa"
	"github.com/gardener/gardener/pkg/features"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
	. "github.com/gardener/gardener/pkg/operator/controller/garden/care"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var (
	gardenManagedResources = []string{
		etcd.Druid,
		runtimegardensystem.ManagedResourceName,
		"istio-system",
		"virtual-garden-istio",
	}

	virtualGardenManagedResources = []string{
		resourcemanager.ManagedResourceName,
		gardeneraccess.ManagedResourceName,
		kubecontrollermanager.ManagedResourceName,
		gardenerapiserver.ManagedResourceNameRuntime,
		gardenerapiserver.ManagedResourceNameVirtual,
		gardeneradmissioncontroller.ManagedResourceNameRuntime,
		gardeneradmissioncontroller.ManagedResourceNameVirtual,
		gardenercontrollermanager.ManagedResourceNameRuntime,
		gardenercontrollermanager.ManagedResourceNameVirtual,
		gardenerscheduler.ManagedResourceNameRuntime,
		gardenerscheduler.ManagedResourceNameVirtual,
		virtualgardensystem.ManagedResourceName,
	}

	observabilityManagedResources = []string{
		hvpa.ManagedResourceName,
		kubestatemetrics.ManagedResourceName,
		fluentoperator.OperatorManagedResourceName,
		fluentoperator.CustomResourcesManagedResourceName + "-garden",
		fluentoperator.FluentBitManagedResourceName,
		vali.ManagedResourceNameRuntime,
		plutono.ManagedResourceName,
		gardenermetricsexporter.ManagedResourceNameRuntime,
		gardenermetricsexporter.ManagedResourceNameVirtual,
		prometheusoperator.ManagedResourceName,
	}

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

		allManagedResources []string

		apiserverAvailabilityCondition          gardencorev1beta1.Condition
		runtimeComponentsHealthyCondition       gardencorev1beta1.Condition
		virtualComponentsHealthyCondition       gardencorev1beta1.Condition
		observabilityComponentsHealthyCondition gardencorev1beta1.Condition
	)

	BeforeEach(func() {
		DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.HVPA, true))

		ctx = context.Background()
		runtimeClient = fakeclient.NewClientBuilder().WithScheme(operatorclient.RuntimeScheme).Build()

		garden = &operatorv1alpha1.Garden{
			ObjectMeta: metav1.ObjectMeta{
				Name: "foo",
			},
		}
		gardenNamespace = "garden"

		fakeClock = testclock.NewFakeClock(time.Now())

		allManagedResources = sets.New[string]().
			Insert(gardenManagedResources...).
			Insert(virtualGardenManagedResources...).
			Insert(observabilityManagedResources...).
			UnsortedList()

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
				for _, name := range allManagedResources {
					Expect(runtimeClient.Create(ctx, healthyManagedResource(name))).To(Succeed())
				}
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

		Context("when optional managed resources are turned off, and required resources are deployed successfully", func() {
			JustBeforeEach(func() {
				DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.HVPA, false))
				garden.Spec.RuntimeCluster.Settings = &operatorv1alpha1.Settings{
					VerticalPodAutoscaler: &operatorv1alpha1.SettingVerticalPodAutoscaler{
						Enabled: ptr.To(false),
					},
				}

				for _, name := range allManagedResources {
					Expect(runtimeClient.Create(ctx, healthyManagedResource(name))).To(Succeed())
				}
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

		Context("when optional managed resources are turned on, and all resources are deployed successfully", func() {
			JustBeforeEach(func() {
				garden.Spec.RuntimeCluster.Settings = &operatorv1alpha1.Settings{
					VerticalPodAutoscaler: &operatorv1alpha1.SettingVerticalPodAutoscaler{
						Enabled: ptr.To(true),
					},
				}

				for _, name := range append(allManagedResources, vpa.ManagedResourceControlName) {
					Expect(runtimeClient.Create(ctx, healthyManagedResource(name))).To(Succeed())
				}
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

			Context("when managed resources are not deployed", func() {
				JustBeforeEach(func() {
					for _, name := range virtualGardenDeployments {
						Expect(runtimeClient.Create(ctx, newDeployment(gardenNamespace, name, true))).To(Succeed())
					}
					for _, name := range virtualGardenETCDs {
						Expect(runtimeClient.Create(ctx, newEtcd(gardenNamespace, name, true))).To(Succeed())
					}
				})

				tests("ResourceNotFound", "not found")
			})

			Context("when all managed resources are deployed, but not healthy", func() {
				JustBeforeEach(func() {
					for _, name := range append(gardenManagedResources, virtualGardenManagedResources...) {
						Expect(runtimeClient.Create(ctx, notHealthyManagedResource(name))).To(Succeed())
					}
					for _, name := range virtualGardenDeployments {
						Expect(runtimeClient.Create(ctx, newDeployment(gardenNamespace, name, true))).To(Succeed())
					}
					for _, name := range virtualGardenETCDs {
						Expect(runtimeClient.Create(ctx, newEtcd(gardenNamespace, name, true))).To(Succeed())
					}
				})

				tests("NotHealthy", "Resources are not healthy")
			})

			Context("when all managed resources are deployed but their resources are not applied", func() {
				JustBeforeEach(func() {
					for _, name := range append(gardenManagedResources, virtualGardenManagedResources...) {
						Expect(runtimeClient.Create(ctx, notAppliedManagedResource(name))).To(Succeed())
					}
					for _, name := range virtualGardenDeployments {
						Expect(runtimeClient.Create(ctx, newDeployment(gardenNamespace, name, true))).To(Succeed())
					}
					for _, name := range virtualGardenETCDs {
						Expect(runtimeClient.Create(ctx, newEtcd(gardenNamespace, name, true))).To(Succeed())
					}
				})

				tests("NotApplied", "Resources are not applied")
			})

			Context("when all managed resources are deployed but their resources are still progressing", func() {
				JustBeforeEach(func() {
					for _, name := range append(gardenManagedResources, virtualGardenManagedResources...) {
						Expect(runtimeClient.Create(ctx, progressingManagedResource(name))).To(Succeed())
					}
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
					for _, name := range append(gardenManagedResources, virtualGardenManagedResources...) {
						Expect(runtimeClient.Create(ctx, managedResource(name, []gardencorev1beta1.Condition{{
							Type:   resourcesv1alpha1.ResourcesApplied,
							Status: gardencorev1beta1.ConditionTrue}},
						))).To(Succeed())
					}
					for _, name := range virtualGardenDeployments {
						Expect(runtimeClient.Create(ctx, newDeployment(gardenNamespace, name, true))).To(Succeed())
					}
					for _, name := range virtualGardenETCDs {
						Expect(runtimeClient.Create(ctx, newEtcd(gardenNamespace, name, true))).To(Succeed())
					}
				})

				tests("MissingManagedResourceCondition", "is missing the following condition(s)")
			})
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
			It("should set ObservabilityComponentsHealthy conditions to false when no ManagedResources are deployed", func() {
				updatedConditions := NewHealth(
					garden,
					runtimeClient,
					gardenClientSet,
					fakeClock,
					nil,
					gardenNamespace,
				).Check(ctx, gardenConditions)

				Expect(updatedConditions).ToNot(BeEmpty())
				Expect(updatedConditions).To(ContainCondition(OfType(operatorv1alpha1.ObservabilityComponentsHealthy), WithReason("ResourceNotFound")))
			})

			It("should set ObservabilityComponentsHealthy conditions to false when ManagedResources are unhealthy", func() {
				for _, name := range observabilityManagedResources {
					Expect(runtimeClient.Create(ctx, notHealthyManagedResource(name))).To(Succeed())
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
				Expect(updatedConditions).To(ContainCondition(OfType(operatorv1alpha1.ObservabilityComponentsHealthy), WithReason("NotHealthy")))
			})

			It("should set ObservabilityComponentsHealthy conditions to false when ManagedResources are not applied", func() {
				for _, name := range observabilityManagedResources {
					Expect(runtimeClient.Create(ctx, notAppliedManagedResource(name))).To(Succeed())
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
				Expect(updatedConditions).To(ContainCondition(OfType(operatorv1alpha1.ObservabilityComponentsHealthy), WithReason("NotApplied")))
			})

			It("should set ObservabilityComponentsHealthy conditions to false when ManagedResources are progressing", func() {
				for _, name := range observabilityManagedResources {
					Expect(runtimeClient.Create(ctx, progressingManagedResource(name))).To(Succeed())
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
				Expect(updatedConditions).To(ContainCondition(OfType(operatorv1alpha1.ObservabilityComponentsHealthy), WithReason("ResourcesProgressing")))
			})

			It("should set ObservabilityComponentsHealthy conditions to false when some ManagedResources conditions are missing", func() {
				for _, name := range observabilityManagedResources {
					Expect(runtimeClient.Create(ctx, managedResource(name, []gardencorev1beta1.Condition{{
						Type:   resourcesv1alpha1.ResourcesApplied,
						Status: gardencorev1beta1.ConditionTrue}},
					))).To(Succeed())
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
				Expect(updatedConditions).To(ContainCondition(OfType(operatorv1alpha1.ObservabilityComponentsHealthy), WithReason("MissingManagedResourceCondition")))
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

func newEtcd(namespace, name string, healthy bool) *druidv1alpha1.Etcd {
	return &druidv1alpha1.Etcd{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
			Labels:    roleLabels("controlplane"),
		},
		Status: druidv1alpha1.EtcdStatus{
			Ready: ptr.To(healthy),
		},
	}
}

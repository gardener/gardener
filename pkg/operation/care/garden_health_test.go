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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	testclock "k8s.io/utils/clock/testing"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component/etcd"
	"github.com/gardener/gardener/pkg/component/gardeneraccess"
	"github.com/gardener/gardener/pkg/component/gardensystem"
	"github.com/gardener/gardener/pkg/component/hvpa"
	"github.com/gardener/gardener/pkg/component/kubecontrollermanager"
	"github.com/gardener/gardener/pkg/component/kubestatemetrics"
	"github.com/gardener/gardener/pkg/component/logging/fluentoperator"
	"github.com/gardener/gardener/pkg/component/logging/vali"
	"github.com/gardener/gardener/pkg/component/resourcemanager"
	"github.com/gardener/gardener/pkg/component/vpa"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/operation/care"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
	"github.com/gardener/gardener/pkg/utils/test"
)

var (
	gardenManagedResources = []string{
		etcd.Druid,
		gardensystem.ManagedResourceName,
		hvpa.ManagedResourceName,
		"istio-system",
		"virtual-garden-istio",
		kubestatemetrics.ManagedResourceName,
		fluentoperator.OperatorManagedResourceName,
		fluentoperator.CustomResourcesManagedResourceName + "-garden",
		fluentoperator.FluentBitManagedResourceName,
		vali.ManagedResourceNameRuntime,
	}

	virtualGardenManagedResources = []string{
		resourcemanager.ManagedResourceName,
		gardeneraccess.ManagedResourceName,
		kubecontrollermanager.ManagedResourceName,
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

		garden          *operatorv1alpha1.Garden
		gardenNamespace string

		apiserverAvailabilityCondition    gardencorev1beta1.Condition
		runtimeComponentsHealthyCondition gardencorev1beta1.Condition
		virtualComponentsHealthyCondition gardencorev1beta1.Condition
	)

	BeforeEach(func() {
		DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.HVPA, true))

		ctx = context.TODO()
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
	})

	Describe("#CheckGarden", func() {
		Context("when all managed resources, deployments and ETCDs are deployed successfully", func() {
			JustBeforeEach(func() {
				for _, name := range append(gardenManagedResources, virtualGardenManagedResources...) {
					Expect(runtimeClient.Create(ctx, healthyManagedResource(name))).To(Succeed())
				}
				for _, name := range virtualGardenDeployments {
					Expect(runtimeClient.Create(ctx, newDeployment(gardenNamespace, name, "controlplane", true))).To(Succeed())
				}
				for _, name := range virtualGardenETCDs {
					Expect(runtimeClient.Create(ctx, newEtcd(gardenNamespace, name, "controlplane", true, nil))).To(Succeed())
				}
			})

			It("should set RuntimeComponentsHealthy and VirtualComponentsHealthy conditions to true", func() {
				healthCheck := care.NewHealthForGarden(garden, runtimeClient, gardenClientSet, fakeClock, gardenNamespace)
				updatedConditions := healthCheck.CheckGarden(ctx, []gardencorev1beta1.Condition{
					apiserverAvailabilityCondition,
					runtimeComponentsHealthyCondition,
					virtualComponentsHealthyCondition,
				}, nil, nil)
				Expect(len(updatedConditions)).ToNot(BeZero())
				Expect(updatedConditions).To(ContainElements(
					beConditionWithStatusReasonAndMessage(gardencorev1beta1.ConditionTrue, "RuntimeComponentsRunning", "All runtime components are healthy."),
					beConditionWithStatusReasonAndMessage(gardencorev1beta1.ConditionTrue, "VirtualComponentsRunning", "All virtual garden components are healthy."),
				))
			})
		})

		Context("when optional managed resources are turned off, and required resources are deployed successfully", func() {
			JustBeforeEach(func() {
				DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.HVPA, false))
				garden.Spec.RuntimeCluster.Settings = &operatorv1alpha1.Settings{
					VerticalPodAutoscaler: &operatorv1alpha1.SettingVerticalPodAutoscaler{
						Enabled: pointer.Bool(false),
					},
				}

				for _, name := range append(gardenManagedResources, virtualGardenManagedResources...) {
					Expect(runtimeClient.Create(ctx, healthyManagedResource(name))).To(Succeed())
				}
				for _, name := range virtualGardenDeployments {
					Expect(runtimeClient.Create(ctx, newDeployment(gardenNamespace, name, "controlplane", true))).To(Succeed())
				}
				for _, name := range virtualGardenETCDs {
					Expect(runtimeClient.Create(ctx, newEtcd(gardenNamespace, name, "controlplane", true, nil))).To(Succeed())
				}
			})

			It("should set RuntimeComponentsHealthy and VirtualComponentsHealthy conditions to true", func() {
				healthCheck := care.NewHealthForGarden(garden, runtimeClient, gardenClientSet, fakeClock, gardenNamespace)
				updatedConditions := healthCheck.CheckGarden(ctx, []gardencorev1beta1.Condition{
					apiserverAvailabilityCondition,
					runtimeComponentsHealthyCondition,
					virtualComponentsHealthyCondition,
				}, nil, nil)
				Expect(len(updatedConditions)).ToNot(BeZero())
				Expect(updatedConditions).To(ContainElements(
					beConditionWithStatusReasonAndMessage(gardencorev1beta1.ConditionTrue, "RuntimeComponentsRunning", "All runtime components are healthy."),
					beConditionWithStatusReasonAndMessage(gardencorev1beta1.ConditionTrue, "VirtualComponentsRunning", "All virtual garden components are healthy."),
				))
			})
		})

		Context("when optional managed resources are turned on, and all resources are deployed successfully", func() {
			JustBeforeEach(func() {
				garden.Spec.RuntimeCluster.Settings = &operatorv1alpha1.Settings{
					VerticalPodAutoscaler: &operatorv1alpha1.SettingVerticalPodAutoscaler{
						Enabled: pointer.Bool(true),
					},
				}

				resources := append(gardenManagedResources, virtualGardenManagedResources...)
				resources = append(resources, vpa.ManagedResourceControlName)
				for _, name := range resources {
					Expect(runtimeClient.Create(ctx, healthyManagedResource(name))).To(Succeed())
				}
				for _, name := range virtualGardenDeployments {
					Expect(runtimeClient.Create(ctx, newDeployment(gardenNamespace, name, "controlplane", true))).To(Succeed())
				}
				for _, name := range virtualGardenETCDs {
					Expect(runtimeClient.Create(ctx, newEtcd(gardenNamespace, name, "controlplane", true, nil))).To(Succeed())
				}
			})

			It("should set RuntimeComponentsHealthy and VirtualComponentsHealthy conditions to true", func() {
				healthCheck := care.NewHealthForGarden(garden, runtimeClient, gardenClientSet, fakeClock, gardenNamespace)
				updatedConditions := healthCheck.CheckGarden(ctx, []gardencorev1beta1.Condition{
					apiserverAvailabilityCondition,
					runtimeComponentsHealthyCondition,
					virtualComponentsHealthyCondition,
				}, nil, nil)
				Expect(len(updatedConditions)).ToNot(BeZero())
				Expect(updatedConditions).To(ContainElements(
					beConditionWithStatusReasonAndMessage(gardencorev1beta1.ConditionTrue, "RuntimeComponentsRunning", "All runtime components are healthy."),
					beConditionWithStatusReasonAndMessage(gardencorev1beta1.ConditionTrue, "VirtualComponentsRunning", "All virtual garden components are healthy."),
				))
			})
		})

		Context("when there are issues with garden managed resources", func() {
			var (
				tests = func(reason, message string) {
					It("should set RuntimeComponentsHealthy and VirtualComponentsHealthy conditions to False if there is no Progressing threshold duration mapping", func() {
						healthCheck := care.NewHealthForGarden(garden, runtimeClient, gardenClientSet, fakeClock, gardenNamespace)
						updatedConditions := healthCheck.CheckGarden(ctx, []gardencorev1beta1.Condition{
							apiserverAvailabilityCondition,
							runtimeComponentsHealthyCondition,
							virtualComponentsHealthyCondition,
						}, nil, nil)
						Expect(len(updatedConditions)).ToNot(BeZero())
						Expect(updatedConditions).To(ContainElements(
							beConditionWithStatusReasonAndMessage(gardencorev1beta1.ConditionFalse, reason, message),
							beConditionWithStatusReasonAndMessage(gardencorev1beta1.ConditionFalse, reason, message),
						))
					})

					It("should set RuntimeComponentsHealthy and VirtualComponentsHealthy conditions to Progressing if time is within threshold duration and condition is currently False", func() {
						runtimeComponentsHealthyCondition.Status = gardencorev1beta1.ConditionFalse
						virtualComponentsHealthyCondition.Status = gardencorev1beta1.ConditionFalse
						fakeClock.Step(30 * time.Second)

						healthCheck := care.NewHealthForGarden(garden, runtimeClient, gardenClientSet, fakeClock, gardenNamespace)
						updatedConditions := healthCheck.CheckGarden(
							ctx,
							[]gardencorev1beta1.Condition{
								apiserverAvailabilityCondition,
								runtimeComponentsHealthyCondition,
								virtualComponentsHealthyCondition,
							},
							map[gardencorev1beta1.ConditionType]time.Duration{
								operatorv1alpha1.RuntimeComponentsHealthy: time.Minute,
								operatorv1alpha1.VirtualComponentsHealthy: time.Minute,
							},
							nil,
						)

						Expect(len(updatedConditions)).ToNot(BeZero())
						Expect(updatedConditions).To(ContainElements(
							beConditionWithStatusReasonAndMessage(gardencorev1beta1.ConditionProgressing, reason, message),
							beConditionWithStatusReasonAndMessage(gardencorev1beta1.ConditionProgressing, reason, message),
						))
					})

					It("should set RuntimeComponentsHealthy and VirtualComponentsHealthy conditions to Progressing if time is within threshold duration and condition is currently True", func() {
						runtimeComponentsHealthyCondition.Status = gardencorev1beta1.ConditionTrue
						virtualComponentsHealthyCondition.Status = gardencorev1beta1.ConditionTrue
						fakeClock.Step(30 * time.Second)

						healthCheck := care.NewHealthForGarden(garden, runtimeClient, gardenClientSet, fakeClock, gardenNamespace)
						updatedConditions := healthCheck.CheckGarden(
							ctx,
							[]gardencorev1beta1.Condition{
								apiserverAvailabilityCondition,
								runtimeComponentsHealthyCondition,
								virtualComponentsHealthyCondition,
							},
							map[gardencorev1beta1.ConditionType]time.Duration{
								operatorv1alpha1.RuntimeComponentsHealthy: time.Minute,
								operatorv1alpha1.VirtualComponentsHealthy: time.Minute,
							},
							nil,
						)

						Expect(len(updatedConditions)).ToNot(BeZero())
						Expect(updatedConditions).To(ContainElements(
							beConditionWithStatusReasonAndMessage(gardencorev1beta1.ConditionProgressing, reason, message),
							beConditionWithStatusReasonAndMessage(gardencorev1beta1.ConditionProgressing, reason, message),
						))
					})

					It("should not set RuntimeComponentsHealthy and VirtualComponentsHealthy conditions to Progressing if Progressing threshold duration has not expired", func() {
						runtimeComponentsHealthyCondition.Status = gardencorev1beta1.ConditionProgressing
						virtualComponentsHealthyCondition.Status = gardencorev1beta1.ConditionProgressing
						fakeClock.Step(30 * time.Second)

						healthCheck := care.NewHealthForGarden(garden, runtimeClient, gardenClientSet, fakeClock, gardenNamespace)
						updatedConditions := healthCheck.CheckGarden(
							ctx,
							[]gardencorev1beta1.Condition{
								apiserverAvailabilityCondition,
								runtimeComponentsHealthyCondition,
								virtualComponentsHealthyCondition,
							},
							map[gardencorev1beta1.ConditionType]time.Duration{
								operatorv1alpha1.RuntimeComponentsHealthy: time.Minute,
								operatorv1alpha1.VirtualComponentsHealthy: time.Minute,
							},
							nil,
						)

						Expect(len(updatedConditions)).ToNot(BeZero())
						Expect(updatedConditions).To(ContainElements(
							beConditionWithStatusReasonAndMessage(gardencorev1beta1.ConditionProgressing, reason, message),
							beConditionWithStatusReasonAndMessage(gardencorev1beta1.ConditionProgressing, reason, message),
						))
					})

					It("should set RuntimeComponentsHealthy and VirtualComponentsHealthy conditions to false if Progressing threshold duration has expired", func() {
						runtimeComponentsHealthyCondition.Status = gardencorev1beta1.ConditionProgressing
						virtualComponentsHealthyCondition.Status = gardencorev1beta1.ConditionProgressing
						fakeClock.Step(90 * time.Second)

						healthCheck := care.NewHealthForGarden(garden, runtimeClient, gardenClientSet, fakeClock, gardenNamespace)
						updatedConditions := healthCheck.CheckGarden(
							ctx,
							[]gardencorev1beta1.Condition{
								apiserverAvailabilityCondition,
								runtimeComponentsHealthyCondition,
								virtualComponentsHealthyCondition,
							},
							map[gardencorev1beta1.ConditionType]time.Duration{
								operatorv1alpha1.RuntimeComponentsHealthy: time.Minute,
								operatorv1alpha1.VirtualComponentsHealthy: time.Minute,
							},
							nil,
						)

						Expect(len(updatedConditions)).ToNot(BeZero())
						Expect(updatedConditions).To(ContainElements(
							beConditionWithStatusReasonAndMessage(gardencorev1beta1.ConditionFalse, reason, message),
							beConditionWithStatusReasonAndMessage(gardencorev1beta1.ConditionFalse, reason, message),
						))
					})
				}
			)

			Context("when managed resources are not deployed", func() {
				JustBeforeEach(func() {
					for _, name := range virtualGardenDeployments {
						Expect(runtimeClient.Create(ctx, newDeployment(gardenNamespace, name, "controlplane", true))).To(Succeed())
					}
					for _, name := range virtualGardenETCDs {
						Expect(runtimeClient.Create(ctx, newEtcd(gardenNamespace, name, "controlplane", true, nil))).To(Succeed())
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
						Expect(runtimeClient.Create(ctx, newDeployment(gardenNamespace, name, "controlplane", true))).To(Succeed())
					}
					for _, name := range virtualGardenETCDs {
						Expect(runtimeClient.Create(ctx, newEtcd(gardenNamespace, name, "controlplane", true, nil))).To(Succeed())
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
						Expect(runtimeClient.Create(ctx, newDeployment(gardenNamespace, name, "controlplane", true))).To(Succeed())
					}
					for _, name := range virtualGardenETCDs {
						Expect(runtimeClient.Create(ctx, newEtcd(gardenNamespace, name, "controlplane", true, nil))).To(Succeed())
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
						Expect(runtimeClient.Create(ctx, newDeployment(gardenNamespace, name, "controlplane", true))).To(Succeed())
					}
					for _, name := range virtualGardenETCDs {
						Expect(runtimeClient.Create(ctx, newEtcd(gardenNamespace, name, "controlplane", true, nil))).To(Succeed())
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
						Expect(runtimeClient.Create(ctx, newDeployment(gardenNamespace, name, "controlplane", true))).To(Succeed())
					}
					for _, name := range virtualGardenETCDs {
						Expect(runtimeClient.Create(ctx, newEtcd(gardenNamespace, name, "controlplane", true, nil))).To(Succeed())
					}
				})

				tests("MissingManagedResourceCondition", "is missing the following condition(s)")
			})
		})

		Context("when there are issues with deployments for virtual garden", func() {
			It("should set VirtualComponentsHealthy conditions to false when the deployments are missing", func() {
				healthCheck := care.NewHealthForGarden(garden, runtimeClient, gardenClientSet, fakeClock, gardenNamespace)
				updatedConditions := healthCheck.CheckGarden(ctx, []gardencorev1beta1.Condition{
					apiserverAvailabilityCondition,
					runtimeComponentsHealthyCondition,
					virtualComponentsHealthyCondition,
				}, nil, nil)
				Expect(len(updatedConditions)).ToNot(BeZero())
				Expect(updatedConditions).To(ContainElements(
					beConditionWithStatusReasonAndMessage(gardencorev1beta1.ConditionFalse, "DeploymentMissing", "Missing required deployments: [virtual-garden-gardener-resource-manager virtual-garden-kube-apiserver virtual-garden-kube-controller-manager]"),
				))
			})

			It("should set VirtualComponentsHealthy conditions to false when the deployments are existing but unhealthy", func() {
				for _, name := range virtualGardenDeployments {
					Expect(runtimeClient.Create(ctx, newDeployment(gardenNamespace, name, "controlplane", false))).To(Succeed())
				}

				healthCheck := care.NewHealthForGarden(garden, runtimeClient, gardenClientSet, fakeClock, gardenNamespace)
				updatedConditions := healthCheck.CheckGarden(ctx, []gardencorev1beta1.Condition{
					apiserverAvailabilityCondition,
					runtimeComponentsHealthyCondition,
					virtualComponentsHealthyCondition,
				}, nil, nil)
				Expect(len(updatedConditions)).ToNot(BeZero())
				Expect(updatedConditions).To(ContainElements(
					beConditionWithStatusReasonAndMessage(gardencorev1beta1.ConditionFalse, "DeploymentUnhealthy", "is unhealthy: condition \"Available\" is missing"),
				))
			})
		})

		Context("when there are issues with ETCDs for virtual garden", func() {
			JustBeforeEach(func() {
				for _, name := range virtualGardenDeployments {
					Expect(runtimeClient.Create(ctx, newDeployment(gardenNamespace, name, "controlplane", true))).To(Succeed())
				}
			})

			It("should set VirtualComponentsHealthy conditions to false when the ETCDs are missing", func() {
				healthCheck := care.NewHealthForGarden(garden, runtimeClient, gardenClientSet, fakeClock, gardenNamespace)
				updatedConditions := healthCheck.CheckGarden(ctx, []gardencorev1beta1.Condition{
					apiserverAvailabilityCondition,
					runtimeComponentsHealthyCondition,
					virtualComponentsHealthyCondition,
				}, nil, nil)
				Expect(len(updatedConditions)).ToNot(BeZero())
				Expect(updatedConditions).To(ContainElements(
					beConditionWithStatusReasonAndMessage(gardencorev1beta1.ConditionFalse, "EtcdMissing", "Missing required etcds: [virtual-garden-etcd-events virtual-garden-etcd-main]"),
				))
			})

			It("should set VirtualComponentsHealthy conditions to false when the ETCDs are existing but unhealthy", func() {
				for _, name := range virtualGardenETCDs {
					Expect(runtimeClient.Create(ctx, newEtcd(gardenNamespace, name, "controlplane", false, nil))).To(Succeed())
				}

				healthCheck := care.NewHealthForGarden(garden, runtimeClient, gardenClientSet, fakeClock, gardenNamespace)
				updatedConditions := healthCheck.CheckGarden(ctx, []gardencorev1beta1.Condition{
					apiserverAvailabilityCondition,
					runtimeComponentsHealthyCondition,
					virtualComponentsHealthyCondition,
				}, nil, nil)
				Expect(len(updatedConditions)).ToNot(BeZero())
				Expect(updatedConditions).To(ContainElements(
					beConditionWithStatusReasonAndMessage(gardencorev1beta1.ConditionFalse, "EtcdUnhealthy", "Etcd extension resource \"virtual-garden-etcd-events\" is unhealthy: etcd \"virtual-garden-etcd-events\" is not ready yet"),
				))
			})
		})
	})
})

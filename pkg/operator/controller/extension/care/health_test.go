// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package care_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	testclock "k8s.io/utils/clock/testing"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/gardener/gardener/pkg/api/indexer"
	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	operatorclient "github.com/gardener/gardener/pkg/operator/client"
	. "github.com/gardener/gardener/pkg/operator/controller/extension/care"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Extension health", func() {
	var (
		ctx           context.Context
		runtimeClient client.Client
		virtualClient client.Client
		fakeClock     *testclock.FakeClock

		controllerRegistration *gardencorev1beta1.ControllerRegistration
		extension              *operatorv1alpha1.Extension
		extensionConditions    ExtensionConditions
		gardenNamespace        string

		controllerInstallationsHealthyCondition gardencorev1beta1.Condition
		extensionHealthyCondition               gardencorev1beta1.Condition
		extensionAdmissionHealthyCondition      gardencorev1beta1.Condition
	)

	BeforeEach(func() {
		ctx = context.Background()
		runtimeClient = fakeclient.NewClientBuilder().WithScheme(operatorclient.RuntimeScheme).Build()
		virtualClient = fakeclient.NewClientBuilder().
			WithScheme(operatorclient.VirtualScheme).
			WithIndex(&gardencorev1beta1.ControllerInstallation{}, core.RegistrationRefName, indexer.ControllerInstallationRegistrationRefNameIndexerFunc).
			Build()

		fakeClock = testclock.NewFakeClock(time.Now())

		controllerRegistration = &gardencorev1beta1.ControllerRegistration{
			ObjectMeta: metav1.ObjectMeta{
				Name: "foo",
			},
		}
		Expect(virtualClient.Create(ctx, controllerRegistration)).To(Succeed())

		extension = &operatorv1alpha1.Extension{
			ObjectMeta: metav1.ObjectMeta{
				Name: "foo",
			},
			Spec: operatorv1alpha1.ExtensionSpec{
				Deployment: &operatorv1alpha1.Deployment{
					AdmissionDeployment: &operatorv1alpha1.AdmissionDeploymentSpec{},
				},
			},
			Status: operatorv1alpha1.ExtensionStatus{
				Conditions: []gardencorev1beta1.Condition{
					{
						Type:               operatorv1alpha1.ExtensionRequiredRuntime,
						Status:             gardencorev1beta1.ConditionTrue,
						LastTransitionTime: metav1.Time{Time: fakeClock.Now()},
					},
					{
						Type:               operatorv1alpha1.ExtensionRequiredVirtual,
						Status:             gardencorev1beta1.ConditionTrue,
						LastTransitionTime: metav1.Time{Time: fakeClock.Now()},
					},
				},
			},
		}

		gardenNamespace = "garden"

		controllerInstallationsHealthyCondition = gardencorev1beta1.Condition{
			Type:               operatorv1alpha1.ControllerInstallationsHealthy,
			LastTransitionTime: metav1.Time{Time: fakeClock.Now()},
		}
		extensionHealthyCondition = gardencorev1beta1.Condition{
			Type:               operatorv1alpha1.ExtensionHealthy,
			LastTransitionTime: metav1.Time{Time: fakeClock.Now()},
		}
		extensionAdmissionHealthyCondition = gardencorev1beta1.Condition{
			Type:               operatorv1alpha1.ExtensionAdmissionHealthy,
			LastTransitionTime: metav1.Time{Time: fakeClock.Now()},
		}
	})

	Describe("#Check", func() {
		JustBeforeEach(func() {
			extension.Status.Conditions = append(extension.Status.Conditions, controllerInstallationsHealthyCondition, extensionHealthyCondition, extensionAdmissionHealthyCondition)

			extensionConditions = NewExtensionConditions(fakeClock, extension)
		})

		Context("when all managed resources are deployed successfully and controller installations are healthy", func() {
			JustBeforeEach(func() {
				Expect(runtimeClient.Create(ctx, healthyManagedResource(gardenNamespace, "extension-foo-garden", true))).To(Succeed())
				Expect(runtimeClient.Create(ctx, healthyManagedResource(gardenNamespace, "extension-admission-virtual-foo", false))).To(Succeed())
				Expect(runtimeClient.Create(ctx, healthyManagedResource(gardenNamespace, "extension-admission-runtime-foo", true))).To(Succeed())
				Expect(virtualClient.Create(ctx, healthyControllerInstallation("foo", controllerRegistration.Name, controllerRegistration.ResourceVersion))).To(Succeed())
			})

			It("should set ExtensionComponentsRunning condition to true", func() {
				updatedConditions := NewHealth(
					extension,
					runtimeClient,
					virtualClient,
					fakeClock,
					nil,
					gardenNamespace,
				).Check(ctx, extensionConditions)

				Expect(updatedConditions).ToNot(BeEmpty())
				Expect(updatedConditions).To(ContainElements(
					beConditionWithStatusReasonAndMessage(gardencorev1beta1.ConditionTrue, "ControllerInstallationsRunning", "All controller installations are healthy."),
					beConditionWithStatusReasonAndMessage(gardencorev1beta1.ConditionTrue, "ExtensionComponentsRunning", "All extension components are healthy."),
					beConditionWithStatusReasonAndMessage(gardencorev1beta1.ConditionTrue, "ExtensionAdmissionComponentsRunning", "All extension admission components are healthy."),
				))
			})
		})

		Context("when there are issues with extension managed resources", func() {
			var (
				tests = func(reason, message string) {
					It("should set ExtensionComponentsRunning condition to False if there is no Progressing threshold duration mapping", func() {
						updatedConditions := NewHealth(
							extension,
							runtimeClient,
							virtualClient,
							fakeClock,
							nil,
							gardenNamespace,
						).Check(ctx, extensionConditions)

						Expect(updatedConditions).ToNot(BeEmpty())
						Expect(updatedConditions).To(ContainElements(
							beConditionWithStatusReasonAndMessage(gardencorev1beta1.ConditionFalse, reason, message),
							beConditionWithStatusReasonAndMessage(gardencorev1beta1.ConditionFalse, reason, message),
						))
					})

					Context("condition is currently False", func() {
						BeforeEach(func() {
							extensionHealthyCondition.Status = gardencorev1beta1.ConditionFalse
							extensionAdmissionHealthyCondition.Status = gardencorev1beta1.ConditionFalse
						})

						It("should set ExtensionComponentsRunning condition to Progressing if time is within threshold duration", func() {
							fakeClock.Step(30 * time.Second)

							updatedConditions := NewHealth(
								extension,
								runtimeClient,
								virtualClient,
								fakeClock,
								map[gardencorev1beta1.ConditionType]time.Duration{
									operatorv1alpha1.ExtensionHealthy:          time.Minute,
									operatorv1alpha1.ExtensionAdmissionHealthy: time.Minute,
								},
								gardenNamespace,
							).Check(ctx, extensionConditions)

							Expect(updatedConditions).ToNot(BeEmpty())
							Expect(updatedConditions).To(ContainElements(
								beConditionWithStatusReasonAndMessage(gardencorev1beta1.ConditionProgressing, reason, message),
								beConditionWithStatusReasonAndMessage(gardencorev1beta1.ConditionProgressing, reason, message),
							))
						})
					})

					Context("condition is currently True", func() {
						BeforeEach(func() {
							extensionHealthyCondition.Status = gardencorev1beta1.ConditionTrue
							extensionAdmissionHealthyCondition.Status = gardencorev1beta1.ConditionTrue
						})

						It("should set ExtensionComponentsRunning condition to Progressing if time is within threshold duration", func() {
							fakeClock.Step(30 * time.Second)

							updatedConditions := NewHealth(
								extension,
								runtimeClient,
								virtualClient,
								fakeClock,
								map[gardencorev1beta1.ConditionType]time.Duration{
									operatorv1alpha1.ExtensionHealthy:          time.Minute,
									operatorv1alpha1.ExtensionAdmissionHealthy: time.Minute,
								},
								gardenNamespace,
							).Check(ctx, extensionConditions)

							Expect(updatedConditions).ToNot(BeEmpty())
							Expect(updatedConditions).To(ContainElements(
								beConditionWithStatusReasonAndMessage(gardencorev1beta1.ConditionProgressing, reason, message),
								beConditionWithStatusReasonAndMessage(gardencorev1beta1.ConditionProgressing, reason, message),
							))
						})
					})

					Context("condition is currently Progressing", func() {
						BeforeEach(func() {
							extensionHealthyCondition.Status = gardencorev1beta1.ConditionProgressing
							extensionAdmissionHealthyCondition.Status = gardencorev1beta1.ConditionProgressing
						})

						It("should not set ExtensionComponentsRunning condition to Progressing if Progressing threshold duration has not expired", func() {
							fakeClock.Step(30 * time.Second)

							updatedConditions := NewHealth(
								extension,
								runtimeClient,
								virtualClient,
								fakeClock,
								map[gardencorev1beta1.ConditionType]time.Duration{
									operatorv1alpha1.ExtensionHealthy:          time.Minute,
									operatorv1alpha1.ExtensionAdmissionHealthy: time.Minute,
								},
								gardenNamespace,
							).Check(ctx, extensionConditions)

							Expect(updatedConditions).ToNot(BeEmpty())
							Expect(updatedConditions).To(ContainElements(
								beConditionWithStatusReasonAndMessage(gardencorev1beta1.ConditionProgressing, reason, message),
								beConditionWithStatusReasonAndMessage(gardencorev1beta1.ConditionProgressing, reason, message),
							))
						})

						It("should set ExtensionComponentsRunning condition to false if Progressing threshold duration has expired", func() {
							fakeClock.Step(90 * time.Second)

							updatedConditions := NewHealth(
								extension,
								runtimeClient,
								virtualClient,
								fakeClock,
								map[gardencorev1beta1.ConditionType]time.Duration{
									operatorv1alpha1.ExtensionHealthy:          time.Minute,
									operatorv1alpha1.ExtensionAdmissionHealthy: time.Minute,
								},
								gardenNamespace,
							).Check(ctx, extensionConditions)

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
					Expect(runtimeClient.Create(ctx, unhealthyManagedResource(gardenNamespace, "extension-foo-garden", true))).To(Succeed())
					Expect(runtimeClient.Create(ctx, unhealthyManagedResource(gardenNamespace, "extension-admission-virtual-foo", false))).To(Succeed())
					Expect(runtimeClient.Create(ctx, unhealthyManagedResource(gardenNamespace, "extension-admission-runtime-foo", true))).To(Succeed())
				})

				tests("NotHealthy", "Resources are not healthy")
			})

			Context("when all managed resources are not applied", func() {
				JustBeforeEach(func() {
					Expect(runtimeClient.Create(ctx, unappliedManagedResource(gardenNamespace, "extension-foo-garden", true))).To(Succeed())
					Expect(runtimeClient.Create(ctx, unappliedManagedResource(gardenNamespace, "extension-admission-virtual-foo", false))).To(Succeed())
					Expect(runtimeClient.Create(ctx, unappliedManagedResource(gardenNamespace, "extension-admission-runtime-foo", true))).To(Succeed())
				})

				tests("NotApplied", "Resources are not applied")
			})

			Context("when all managed resources are still progressing", func() {
				JustBeforeEach(func() {
					Expect(runtimeClient.Create(ctx, progressingManagedResource(gardenNamespace, "extension-foo-garden", true))).To(Succeed())
					Expect(runtimeClient.Create(ctx, progressingManagedResource(gardenNamespace, "extension-admission-virtual-foo", false))).To(Succeed())
					Expect(runtimeClient.Create(ctx, progressingManagedResource(gardenNamespace, "extension-admission-runtime-foo", true))).To(Succeed())
				})

				tests("ResourcesProgressing", "Resources are progressing")
			})

			Context("when all managed resources are deployed but not all required conditions are present", func() {
				JustBeforeEach(func() {
					Expect(runtimeClient.Create(ctx, managedResource(gardenNamespace, "extension-foo-garden", true, []gardencorev1beta1.Condition{{
						Type:   resourcesv1alpha1.ResourcesApplied,
						Status: gardencorev1beta1.ConditionTrue}},
					))).To(Succeed())
					Expect(runtimeClient.Create(ctx, managedResource(gardenNamespace, "extension-admission-virtual-foo", false, []gardencorev1beta1.Condition{{
						Type:   resourcesv1alpha1.ResourcesApplied,
						Status: gardencorev1beta1.ConditionTrue}},
					))).To(Succeed())
					Expect(runtimeClient.Create(ctx, managedResource(gardenNamespace, "extension-admission-runtime-foo", true, []gardencorev1beta1.Condition{{
						Type:   resourcesv1alpha1.ResourcesApplied,
						Status: gardencorev1beta1.ConditionTrue}},
					))).To(Succeed())
				})

				tests("MissingManagedResourceCondition", "is missing the following condition(s)")
			})
		})

		Context("when there are issues with controller installations", func() {
			var (
				tests = func(reason, message string) {
					It("should set ControllerInstallationsRunning condition to False if there is no Progressing threshold duration mapping", func() {
						updatedConditions := NewHealth(
							extension,
							runtimeClient,
							virtualClient,
							fakeClock,
							nil,
							gardenNamespace,
						).Check(ctx, extensionConditions)

						Expect(updatedConditions).ToNot(BeEmpty())
						Expect(updatedConditions).To(ContainElements(
							beConditionWithStatusReasonAndMessage(gardencorev1beta1.ConditionFalse, reason, message),
						))
					})

					Context("condition is currently False", func() {
						BeforeEach(func() {
							controllerInstallationsHealthyCondition.Status = gardencorev1beta1.ConditionFalse
						})

						It("should set ControllerInstallationsRunning condition to Progressing if time is within threshold duration", func() {
							fakeClock.Step(30 * time.Second)

							updatedConditions := NewHealth(
								extension,
								runtimeClient,
								virtualClient,
								fakeClock,
								map[gardencorev1beta1.ConditionType]time.Duration{
									operatorv1alpha1.ControllerInstallationsHealthy: time.Minute,
								},
								gardenNamespace,
							).Check(ctx, extensionConditions)

							Expect(updatedConditions).ToNot(BeEmpty())
							Expect(updatedConditions).To(ContainElements(
								beConditionWithStatusReasonAndMessage(gardencorev1beta1.ConditionProgressing, reason, message),
							))
						})
					})

					Context("condition is currently True", func() {
						BeforeEach(func() {
							controllerInstallationsHealthyCondition.Status = gardencorev1beta1.ConditionTrue
						})

						It("should set ControllerInstallationsRunning condition to Progressing if time is within threshold duration", func() {
							fakeClock.Step(30 * time.Second)

							updatedConditions := NewHealth(
								extension,
								runtimeClient,
								virtualClient,
								fakeClock,
								map[gardencorev1beta1.ConditionType]time.Duration{
									operatorv1alpha1.ControllerInstallationsHealthy: time.Minute,
								},
								gardenNamespace,
							).Check(ctx, extensionConditions)

							Expect(updatedConditions).ToNot(BeEmpty())
							Expect(updatedConditions).To(ContainElements(
								beConditionWithStatusReasonAndMessage(gardencorev1beta1.ConditionProgressing, reason, message),
							))
						})
					})

					Context("condition is currently Progressing", func() {
						BeforeEach(func() {
							controllerInstallationsHealthyCondition.Status = gardencorev1beta1.ConditionProgressing
						})

						It("should not set ExtensionComponentsRunning condition to Progressing if Progressing threshold duration has not expired", func() {
							fakeClock.Step(30 * time.Second)

							updatedConditions := NewHealth(
								extension,
								runtimeClient,
								virtualClient,
								fakeClock,
								map[gardencorev1beta1.ConditionType]time.Duration{
									operatorv1alpha1.ControllerInstallationsHealthy: time.Minute,
								},
								gardenNamespace,
							).Check(ctx, extensionConditions)

							Expect(updatedConditions).ToNot(BeEmpty())
							Expect(updatedConditions).To(ContainElements(
								beConditionWithStatusReasonAndMessage(gardencorev1beta1.ConditionProgressing, reason, message),
							))
						})

						It("should set ControllerInstallationsRunning condition to false if Progressing threshold duration has expired", func() {
							fakeClock.Step(90 * time.Second)

							updatedConditions := NewHealth(
								extension,
								runtimeClient,
								virtualClient,
								fakeClock,
								map[gardencorev1beta1.ConditionType]time.Duration{
									operatorv1alpha1.ControllerInstallationsHealthy: time.Minute,
									operatorv1alpha1.ExtensionAdmissionHealthy:      time.Minute,
								},
								gardenNamespace,
							).Check(ctx, extensionConditions)

							Expect(updatedConditions).ToNot(BeEmpty())
							Expect(updatedConditions).To(ContainElements(
								beConditionWithStatusReasonAndMessage(gardencorev1beta1.ConditionFalse, reason, message),
							))
						})
					})
				}
			)

			Context("when all controller installations are unhealthy", func() {
				JustBeforeEach(func() {
					Expect(virtualClient.Create(ctx, unhealthyControllerInstallation("foo", controllerRegistration.Name, controllerRegistration.ResourceVersion))).To(Succeed())
				})

				tests("NotHealthy", "Controller installation is not healthy")
			})

			Context("when all controller installations are invalid", func() {
				JustBeforeEach(func() {
					Expect(virtualClient.Create(ctx, inValidControllerInstallation("foo", controllerRegistration.Name, controllerRegistration.ResourceVersion))).To(Succeed())
				})

				tests("Invalid", "Controller installation is invalid")
			})

			Context("when all controller installations are not installed", func() {
				JustBeforeEach(func() {
					Expect(virtualClient.Create(ctx, notInstalledControllerInstallation("foo", controllerRegistration.Name, controllerRegistration.ResourceVersion))).To(Succeed())
				})

				tests("NotInstalled", "Controller installation is not installed")
			})

			Context("when all controller installations are outdated", func() {
				JustBeforeEach(func() {
					Expect(virtualClient.Create(ctx, outdatedControllerInstallation("foo", controllerRegistration.Name))).To(Succeed())
				})

				tests("OutdatedControllerRegistration", "observed resource version of controller registration 'foo' in controller installation 'foo' outdated (0/1)")
			})

			Context("when all controller installations are still progressing", func() {
				JustBeforeEach(func() {
					Expect(virtualClient.Create(ctx, progressingControllerInstallation("foo", controllerRegistration.Name, controllerRegistration.ResourceVersion))).To(Succeed())
				})

				tests("Progressing", "Controller installation is progressing")
			})

			Context("when all controller installations are deployed but not all required conditions are present", func() {
				JustBeforeEach(func() {
					Expect(virtualClient.Create(ctx, controllerInstallation("foo", controllerRegistration.Name, controllerRegistration.ResourceVersion, []gardencorev1beta1.Condition{{
						Type:   resourcesv1alpha1.ResourcesApplied,
						Status: gardencorev1beta1.ConditionTrue}},
					))).To(Succeed())
				})

				tests("MissingControllerInstallationCondition", "is missing the following condition(s)")
			})
		})
	})

	Describe("ExtensionConditions", func() {
		Describe("#NewExtensionConditions", func() {
			It("should initialize nothing if extension is not required", func() {
				extension.Spec.Deployment = nil
				extension.Status.Conditions = nil
				conditions := NewExtensionConditions(fakeClock, extension)

				Expect(conditions.ConvertToSlice()).To(BeEmpty())
			})

			It("should initialize all conditions", func() {
				conditions := NewExtensionConditions(fakeClock, extension)

				Expect(conditions.ConvertToSlice()).To(ConsistOf(
					beConditionWithStatusReasonAndMessage("Unknown", "ConditionInitialized", "The condition has been initialized but its semantic check has not been performed yet."),
					beConditionWithStatusReasonAndMessage("Unknown", "ConditionInitialized", "The condition has been initialized but its semantic check has not been performed yet."),
					beConditionWithStatusReasonAndMessage("Unknown", "ConditionInitialized", "The condition has been initialized but its semantic check has not been performed yet."),
				))
			})

			It("should only initialize missing conditions", func() {
				extension.Status.Conditions = append(extension.Status.Conditions, gardencorev1beta1.Condition{Type: "RuntimeHealthy"}, gardencorev1beta1.Condition{Type: "Foo"})
				conditions := NewExtensionConditions(fakeClock, extension)

				Expect(conditions.ConvertToSlice()).To(HaveExactElements(
					beConditionWithStatusReasonAndMessage("Unknown", "ConditionInitialized", "The condition has been initialized but its semantic check has not been performed yet."),
					OfType("RuntimeHealthy"),
					beConditionWithStatusReasonAndMessage("Unknown", "ConditionInitialized", "The condition has been initialized but its semantic check has not been performed yet."),
				))
			})
		})

		Describe("#ConvertToSlice", func() {
			It("should return the expected conditions", func() {
				conditions := NewExtensionConditions(fakeClock, extension)

				Expect(conditions.ConvertToSlice()).To(HaveExactElements(
					OfType("ControllerInstallationsHealthy"),
					OfType("RuntimeHealthy"),
					OfType("AdmissionHealthy"),
				))
			})
		})

		Describe("#ConditionTypes", func() {
			It("should return the expected condition types", func() {
				Expect(ConditionTypes()).To(HaveExactElements(
					gardencorev1beta1.ConditionType("ControllerInstallationsHealthy"),
					gardencorev1beta1.ConditionType("RuntimeHealthy"),
					gardencorev1beta1.ConditionType("AdmissionHealthy"),
				))
			})
		})
	})
})

func beConditionWithStatusReasonAndMessage(status gardencorev1beta1.ConditionStatus, reason, message string) types.GomegaMatcher {
	return And(WithStatus(status), WithReason(reason), WithMessage(message))
}

func healthyManagedResource(namespace, name string, classSeed bool) *resourcesv1alpha1.ManagedResource {
	return managedResource(
		namespace,
		name,
		classSeed,
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

func unhealthyManagedResource(namespace, name string, classSeed bool) *resourcesv1alpha1.ManagedResource {
	return managedResource(
		namespace,
		name,
		classSeed,
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

func unappliedManagedResource(namespace, name string, classSeed bool) *resourcesv1alpha1.ManagedResource {
	return managedResource(
		namespace,
		name,
		classSeed,
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

func progressingManagedResource(namespace, name string, classSeed bool) *resourcesv1alpha1.ManagedResource {
	return managedResource(
		namespace,
		name,
		classSeed,
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

func managedResource(namespace, name string, classSeed bool, conditions []gardencorev1beta1.Condition) *resourcesv1alpha1.ManagedResource {
	var (
		class *string
	)

	if classSeed {
		class = ptr.To("seed")
	}

	return &resourcesv1alpha1.ManagedResource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: resourcesv1alpha1.ManagedResourceSpec{
			Class: class,
		},
		Status: resourcesv1alpha1.ManagedResourceStatus{
			Conditions: conditions,
		},
	}
}

func healthyControllerInstallation(name, controllerRegistrationName, controllerRegistrationResourceVersion string) *gardencorev1beta1.ControllerInstallation {
	return controllerInstallation(
		name,
		controllerRegistrationName,
		controllerRegistrationResourceVersion,
		[]gardencorev1beta1.Condition{
			{
				Type:   gardencorev1beta1.ControllerInstallationValid,
				Status: gardencorev1beta1.ConditionTrue,
			},
			{
				Type:   gardencorev1beta1.ControllerInstallationInstalled,
				Status: gardencorev1beta1.ConditionTrue,
			},
			{
				Type:   gardencorev1beta1.ControllerInstallationHealthy,
				Status: gardencorev1beta1.ConditionTrue,
			},
			{
				Type:   gardencorev1beta1.ControllerInstallationProgressing,
				Status: gardencorev1beta1.ConditionFalse,
			},
		})
}

func inValidControllerInstallation(name, controllerRegistrationName, controllerRegistrationResourceVersion string) *gardencorev1beta1.ControllerInstallation {
	return controllerInstallation(
		name,
		controllerRegistrationName,
		controllerRegistrationResourceVersion,
		[]gardencorev1beta1.Condition{
			{
				Type:    gardencorev1beta1.ControllerInstallationValid,
				Reason:  "Invalid",
				Message: "Controller installation is invalid",
				Status:  gardencorev1beta1.ConditionFalse,
			},
			{
				Type:   gardencorev1beta1.ControllerInstallationInstalled,
				Status: gardencorev1beta1.ConditionTrue,
			},
			{
				Type:   gardencorev1beta1.ControllerInstallationHealthy,
				Status: gardencorev1beta1.ConditionTrue,
			},
			{
				Type:   gardencorev1beta1.ControllerInstallationProgressing,
				Status: gardencorev1beta1.ConditionFalse,
			},
		})
}

func notInstalledControllerInstallation(name, controllerRegistrationName, controllerRegistrationResourceVersion string) *gardencorev1beta1.ControllerInstallation {
	return controllerInstallation(
		name,
		controllerRegistrationName,
		controllerRegistrationResourceVersion,
		[]gardencorev1beta1.Condition{
			{
				Type:   gardencorev1beta1.ControllerInstallationValid,
				Status: gardencorev1beta1.ConditionTrue,
			},
			{
				Type:    gardencorev1beta1.ControllerInstallationInstalled,
				Reason:  "NotInstalled",
				Message: "Controller installation is not installed",
				Status:  gardencorev1beta1.ConditionFalse,
			},
			{
				Type:   gardencorev1beta1.ControllerInstallationHealthy,
				Status: gardencorev1beta1.ConditionTrue,
			},
			{
				Type:   gardencorev1beta1.ControllerInstallationProgressing,
				Status: gardencorev1beta1.ConditionFalse,
			},
		})
}

func outdatedControllerInstallation(name, controllerRegistrationName string) *gardencorev1beta1.ControllerInstallation {
	return controllerInstallation(
		name,
		controllerRegistrationName,
		"0",
		[]gardencorev1beta1.Condition{
			{
				Type:   gardencorev1beta1.ControllerInstallationValid,
				Status: gardencorev1beta1.ConditionTrue,
			},
			{
				Type:   gardencorev1beta1.ControllerInstallationInstalled,
				Status: gardencorev1beta1.ConditionTrue,
			},
			{
				Type:   gardencorev1beta1.ControllerInstallationHealthy,
				Status: gardencorev1beta1.ConditionTrue,
			},
			{
				Type:   gardencorev1beta1.ControllerInstallationProgressing,
				Status: gardencorev1beta1.ConditionFalse,
			},
		})
}

func progressingControllerInstallation(name, controllerRegistrationName, controllerRegistrationResourceVersion string) *gardencorev1beta1.ControllerInstallation {
	return controllerInstallation(
		name,
		controllerRegistrationName,
		controllerRegistrationResourceVersion,
		[]gardencorev1beta1.Condition{
			{
				Type:   gardencorev1beta1.ControllerInstallationValid,
				Status: gardencorev1beta1.ConditionTrue,
			},
			{
				Type:   gardencorev1beta1.ControllerInstallationInstalled,
				Status: gardencorev1beta1.ConditionTrue,
			},
			{
				Type:   gardencorev1beta1.ControllerInstallationHealthy,
				Status: gardencorev1beta1.ConditionTrue,
			},
			{
				Type:    gardencorev1beta1.ControllerInstallationProgressing,
				Reason:  "Progressing",
				Message: "Controller installation is progressing",
				Status:  gardencorev1beta1.ConditionTrue,
			},
		})
}

func unhealthyControllerInstallation(name, controllerRegistrationName, controllerRegistrationResourceVersion string) *gardencorev1beta1.ControllerInstallation {
	return controllerInstallation(
		name,
		controllerRegistrationName,
		controllerRegistrationResourceVersion,
		[]gardencorev1beta1.Condition{
			{
				Type:   gardencorev1beta1.ControllerInstallationValid,
				Status: gardencorev1beta1.ConditionTrue,
			},
			{
				Type:   gardencorev1beta1.ControllerInstallationInstalled,
				Status: gardencorev1beta1.ConditionTrue,
			},
			{
				Type:    gardencorev1beta1.ControllerInstallationHealthy,
				Reason:  "NotHealthy",
				Message: "Controller installation is not healthy",
				Status:  gardencorev1beta1.ConditionFalse,
			},
			{
				Type:   gardencorev1beta1.ControllerInstallationProgressing,
				Status: gardencorev1beta1.ConditionFalse,
			},
		})
}

func controllerInstallation(name, controllerRegistrationName, controllerRegistrationResourceVersion string, conditions []gardencorev1beta1.Condition) *gardencorev1beta1.ControllerInstallation {
	return &gardencorev1beta1.ControllerInstallation{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: gardencorev1beta1.ControllerInstallationSpec{
			RegistrationRef: corev1.ObjectReference{
				Name:            controllerRegistrationName,
				ResourceVersion: controllerRegistrationResourceVersion,
			},
		},
		Status: gardencorev1beta1.ControllerInstallationStatus{
			Conditions: conditions,
		},
	}
}

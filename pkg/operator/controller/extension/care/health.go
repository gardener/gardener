// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package care

import (
	"context"
	"fmt"
	"time"

	"k8s.io/utils/clock"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/utils/flow"
	"github.com/gardener/gardener/pkg/utils/gardener"
	healthchecker "github.com/gardener/gardener/pkg/utils/kubernetes/health/checker"
)

// health contains information needed to execute health checks for the extension.
type health struct {
	extension           *operatorv1alpha1.Extension
	gardenNamespace     string
	runtimeClient       client.Client
	virtualClient       client.Client
	clock               clock.Clock
	conditionThresholds map[gardencorev1beta1.ConditionType]time.Duration
	healthChecker       *healthchecker.HealthChecker
}

// NewHealth creates a new Health instance with the given parameters.
func NewHealth(
	extension *operatorv1alpha1.Extension,
	runtimeClient client.Client,
	virtualClient client.Client,
	clock clock.Clock,
	conditionThresholds map[gardencorev1beta1.ConditionType]time.Duration,
	gardenNamespace string,
) HealthCheck {
	return &health{
		extension:           extension,
		gardenNamespace:     gardenNamespace,
		runtimeClient:       runtimeClient,
		virtualClient:       virtualClient,
		clock:               clock,
		conditionThresholds: conditionThresholds,
		healthChecker:       healthchecker.NewHealthChecker(runtimeClient, clock, conditionThresholds, nil),
	}
}

// Check conducts the health checks on all the given conditions.
func (h *health) Check(ctx context.Context, conditions ExtensionConditions) []gardencorev1beta1.Condition {
	var taskFns []flow.TaskFn

	if conditions.controllerInstallationsHealthy != nil {
		taskFns = append(taskFns, func(_ context.Context) error {
			newControllerInstallationsCondition, err := h.checkControllerInstallations(ctx, *conditions.controllerInstallationsHealthy)
			conditions.controllerInstallationsHealthy = ptr.To(v1beta1helper.NewConditionOrError(h.clock, *conditions.controllerInstallationsHealthy, newControllerInstallationsCondition, err))
			return nil
		})
	}

	if conditions.extensionHealthy != nil {
		taskFns = append(taskFns, func(_ context.Context) error {
			newExtensionComponentsCondition, err := h.checkExtension(ctx, *conditions.extensionHealthy)
			conditions.extensionHealthy = ptr.To(v1beta1helper.NewConditionOrError(h.clock, *conditions.extensionHealthy, newExtensionComponentsCondition, err))
			return nil
		})
	}

	if conditions.extensionAdmissionHealthy != nil {
		taskFns = append(taskFns, func(_ context.Context) error {
			newExtensionAdmissionComponentsCondition, err := h.checkExtensionAdmission(ctx, *conditions.extensionAdmissionHealthy)
			conditions.extensionAdmissionHealthy = ptr.To(v1beta1helper.NewConditionOrError(h.clock, *conditions.extensionAdmissionHealthy, newExtensionAdmissionComponentsCondition, err))
			return nil
		})
	}

	_ = flow.Parallel(taskFns...)(ctx)

	return conditions.ConvertToSlice()
}

func (h *health) checkControllerInstallations(ctx context.Context, condition gardencorev1beta1.Condition) (*gardencorev1beta1.Condition, error) {
	controllerInstallations := &gardencorev1beta1.ControllerInstallationList{}
	if err := h.virtualClient.List(ctx, controllerInstallations, client.MatchingFields{gardencore.RegistrationRefName: h.extension.Name}); err != nil {
		return nil, fmt.Errorf("failed to list controller installations: %w", err)
	}

	if exitCondition, err := h.healthChecker.CheckControllerInstallations(ctx, h.virtualClient, condition, controllerInstallations.Items, func(ci gardencorev1beta1.ControllerInstallation) bool {
		return ci.Spec.RegistrationRef.Name == h.extension.Name
	}, nil); err != nil {
		return nil, fmt.Errorf("failed to check controller installations: %w", err)
	} else if exitCondition != nil {
		return exitCondition, nil
	}

	return ptr.To(v1beta1helper.UpdatedConditionWithClock(h.clock, condition, gardencorev1beta1.ConditionTrue, "ControllerInstallationsRunning", "All controller installations are healthy.")), nil
}

func (h *health) checkExtension(ctx context.Context, condition gardencorev1beta1.Condition) (*gardencorev1beta1.Condition, error) {
	managedResource := &resourcesv1alpha1.ManagedResource{}
	managedResourceName := gardener.ExtensionRuntimeManagedResourceName(h.extension.Name)

	if err := h.runtimeClient.Get(ctx, client.ObjectKey{Namespace: h.gardenNamespace, Name: managedResourceName}, managedResource); err != nil {
		return nil, fmt.Errorf("failed to get managed resource %s: %w", managedResourceName, err)
	}

	if exitCondition := h.healthChecker.CheckManagedResource(condition, managedResource, nil); exitCondition != nil {
		return exitCondition, nil
	}

	return ptr.To(v1beta1helper.UpdatedConditionWithClock(h.clock, condition, gardencorev1beta1.ConditionTrue, "ExtensionComponentsRunning", "All extension components are healthy.")), nil
}

func (h *health) checkExtensionAdmission(ctx context.Context, condition gardencorev1beta1.Condition) (*gardencorev1beta1.Condition, error) {
	managedResourceNames := []string{
		gardener.ExtensionAdmissionVirtualManagedResourceName(h.extension.Name),
		gardener.ExtensionAdmissionRuntimeManagedResourceName(h.extension.Name),
	}

	for _, managedResourceName := range managedResourceNames {
		managedResource := &resourcesv1alpha1.ManagedResource{}
		if err := h.runtimeClient.Get(ctx, client.ObjectKey{Namespace: h.gardenNamespace, Name: managedResourceName}, managedResource); err != nil {
			return nil, fmt.Errorf("failed to get managed resource %s: %w", managedResourceName, err)
		}

		if exitCondition := h.healthChecker.CheckManagedResource(condition, managedResource, nil); exitCondition != nil {
			return exitCondition, nil
		}
	}

	return ptr.To(v1beta1helper.UpdatedConditionWithClock(h.clock, condition, gardencorev1beta1.ConditionTrue, "ExtensionAdmissionComponentsRunning", "All extension admission components are healthy.")), nil
}

// ExtensionConditions contains all conditions of the extension status subresource.
type ExtensionConditions struct {
	controllerInstallationsHealthy *gardencorev1beta1.Condition
	extensionHealthy               *gardencorev1beta1.Condition
	extensionAdmissionHealthy      *gardencorev1beta1.Condition
}

// ConvertToSlice returns the extension conditions as a slice.
func (e ExtensionConditions) ConvertToSlice() []gardencorev1beta1.Condition {
	var conditions []gardencorev1beta1.Condition

	if e.controllerInstallationsHealthy != nil {
		conditions = append(conditions, *e.controllerInstallationsHealthy)
	}

	if e.extensionHealthy != nil {
		conditions = append(conditions, *e.extensionHealthy)
	}

	if e.extensionAdmissionHealthy != nil {
		conditions = append(conditions, *e.extensionAdmissionHealthy)
	}

	return conditions
}

// ConditionTypes returns all extension condition types.
func ConditionTypes() []gardencorev1beta1.ConditionType {
	return []gardencorev1beta1.ConditionType{
		operatorv1alpha1.ControllerInstallationsHealthy,
		operatorv1alpha1.ExtensionHealthy,
		operatorv1alpha1.ExtensionAdmissionHealthy,
	}
}

// NewExtensionConditions returns a new instance of ExtensionConditions.
// All conditions are retrieved from the given 'status' or newly initialized.
func NewExtensionConditions(clock clock.Clock, extension *operatorv1alpha1.Extension) ExtensionConditions {
	var extensionConditions ExtensionConditions

	if gardener.IsControllerInstallationInVirtualRequired(extension) {
		extensionConditions.controllerInstallationsHealthy = ptr.To(v1beta1helper.GetOrInitConditionWithClock(clock, extension.Status.Conditions, operatorv1alpha1.ControllerInstallationsHealthy))
	}

	if gardener.IsExtensionInRuntimeRequired(extension) {
		extensionConditions.extensionHealthy = ptr.To(v1beta1helper.GetOrInitConditionWithClock(clock, extension.Status.Conditions, operatorv1alpha1.ExtensionHealthy))
	}

	if extension.Spec.Deployment != nil && extension.Spec.Deployment.AdmissionDeployment != nil {
		extensionConditions.extensionAdmissionHealthy = ptr.To(v1beta1helper.GetOrInitConditionWithClock(clock, extension.Status.Conditions, operatorv1alpha1.ExtensionAdmissionHealthy))
	}

	return extensionConditions
}

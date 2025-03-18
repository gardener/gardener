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

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/apis/operator/v1alpha1/helper"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils/flow"
	healthchecker "github.com/gardener/gardener/pkg/utils/kubernetes/health/checker"
)

// health contains information needed to execute health checks for the extension.
type health struct {
	extension           *operatorv1alpha1.Extension
	gardenNamespace     string
	runtimeClient       client.Client
	gardenClientSet     kubernetes.Interface
	clock               clock.Clock
	conditionThresholds map[gardencorev1beta1.ConditionType]time.Duration
	healthChecker       *healthchecker.HealthChecker
}

// NewHealth creates a new Health instance with the given parameters.
func NewHealth(
	extension *operatorv1alpha1.Extension,
	runtimeClient client.Client,
	gardenClientSet kubernetes.Interface,
	clock clock.Clock,
	conditionThresholds map[gardencorev1beta1.ConditionType]time.Duration,
	gardenNamespace string,
) HealthCheck {
	return &health{
		extension:           extension,
		gardenNamespace:     gardenNamespace,
		runtimeClient:       runtimeClient,
		gardenClientSet:     gardenClientSet,
		clock:               clock,
		conditionThresholds: conditionThresholds,
		healthChecker:       healthchecker.NewHealthChecker(runtimeClient, clock, conditionThresholds, nil),
	}
}

// Check conducts the health checks on all the given conditions.
func (h *health) Check(ctx context.Context, conditions ExtensionConditions) []gardencorev1beta1.Condition {
	var taskFns []flow.TaskFn

	if conditions.extensionHealthy != nil {
		taskFns = append(taskFns, func(_ context.Context) error {
			newExtensionComponentsCondition, err := h.checkExtension(ctx, *conditions.extensionHealthy)
			conditions.extensionHealthy = ptr.To(v1beta1helper.NewConditionOrError(h.clock, *conditions.extensionHealthy, newExtensionComponentsCondition, err))
			return nil
		})
	}

	_ = flow.Parallel(taskFns...)(ctx)

	return conditions.ConvertToSlice()
}

func (h *health) checkExtension(ctx context.Context, condition gardencorev1beta1.Condition) (*gardencorev1beta1.Condition, error) {
	managedResource := &resourcesv1alpha1.ManagedResource{}
	managedResourceName := helper.ExtensionRuntimeManagedResourceName(h.extension.Name)

	if err := h.runtimeClient.Get(ctx, client.ObjectKey{Namespace: h.gardenNamespace, Name: managedResourceName}, managedResource); err != nil {
		return nil, fmt.Errorf("failed to get managed resource %s: %w", managedResourceName, err)
	}

	if exitCondition := h.healthChecker.CheckManagedResource(condition, managedResource, nil); exitCondition != nil {
		return exitCondition, nil
	}

	return ptr.To(v1beta1helper.UpdatedConditionWithClock(h.clock, condition, gardencorev1beta1.ConditionTrue, "ExtensionComponentsRunning", "All extension components are healthy.")), nil
}

// ExtensionConditions contains all conditions of the extension status subresource.
type ExtensionConditions struct {
	extensionHealthy *gardencorev1beta1.Condition
}

// ConvertToSlice returns the extension conditions as a slice.
func (e ExtensionConditions) ConvertToSlice() []gardencorev1beta1.Condition {
	var conditions []gardencorev1beta1.Condition

	if e.extensionHealthy != nil {
		conditions = append(conditions, *e.extensionHealthy)
	}

	return conditions
}

// ConditionTypes returns all extension condition types.
func (e ExtensionConditions) ConditionTypes() []gardencorev1beta1.ConditionType {
	return []gardencorev1beta1.ConditionType{
		operatorv1alpha1.ExtensionHealthy,
	}
}

// NewExtensionConditions returns a new instance of ExtensionConditions.
// All conditions are retrieved from the given 'status' or newly initialized.
func NewExtensionConditions(clock clock.Clock, extension *operatorv1alpha1.Extension) ExtensionConditions {
	var extensionConditions ExtensionConditions

	if helper.IsDeploymentInRuntimeRequired(extension) {
		extensionConditions.extensionHealthy = ptr.To(v1beta1helper.GetOrInitConditionWithClock(clock, extension.Status.Conditions, operatorv1alpha1.ExtensionHealthy))
	}

	return extensionConditions
}

// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package care

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/clock"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils/flow"
	kuberneteshealth "github.com/gardener/gardener/pkg/utils/kubernetes/health"
	healthchecker "github.com/gardener/gardener/pkg/utils/kubernetes/health/checker"
)

const virtualGardenPrefix = "virtual-garden-"

// health contains information needed to execute health checks for garden.
type health struct {
	garden              *operatorv1alpha1.Garden
	gardenNamespace     string
	runtimeClient       client.Client
	gardenClientSet     kubernetes.Interface
	clock               clock.Clock
	conditionThresholds map[gardencorev1beta1.ConditionType]time.Duration
	healthChecker       *healthchecker.HealthChecker
}

// NewHealth creates a new Health instance with the given parameters.
func NewHealth(
	garden *operatorv1alpha1.Garden,
	runtimeClient client.Client,
	gardenClientSet kubernetes.Interface,
	clock clock.Clock,
	conditionThresholds map[gardencorev1beta1.ConditionType]time.Duration,
	gardenNamespace string,
) HealthCheck {
	return &health{
		garden:              garden,
		gardenNamespace:     gardenNamespace,
		runtimeClient:       runtimeClient,
		gardenClientSet:     gardenClientSet,
		clock:               clock,
		conditionThresholds: conditionThresholds,
		healthChecker:       healthchecker.NewHealthChecker(runtimeClient, clock, conditionThresholds, garden.Status.LastOperation),
	}
}

// Check conducts the health checks on all the given conditions.
func (h *health) Check(ctx context.Context, conditions GardenConditions) []gardencorev1beta1.Condition {
	managedResources, err := h.listManagedResources(ctx)
	if err != nil {
		conditions.virtualGardenAPIServerAvailable = v1beta1helper.NewConditionOrError(h.clock, conditions.virtualGardenAPIServerAvailable, nil, err)
		conditions.runtimeComponentsHealthy = v1beta1helper.NewConditionOrError(h.clock, conditions.runtimeComponentsHealthy, nil, err)
		conditions.virtualComponentsHealthy = v1beta1helper.NewConditionOrError(h.clock, conditions.virtualComponentsHealthy, nil, err)
		conditions.observabilityComponentsHealthy = v1beta1helper.NewConditionOrError(h.clock, conditions.observabilityComponentsHealthy, nil, err)
		return conditions.ConvertToSlice()
	}

	taskFns := []flow.TaskFn{
		func(ctx context.Context) error {
			conditions.virtualGardenAPIServerAvailable = h.checkAPIServerAvailability(ctx, conditions.virtualGardenAPIServerAvailable)
			return nil
		},
		func(_ context.Context) error {
			newRuntimeComponentsCondition := h.checkRuntimeComponents(conditions.runtimeComponentsHealthy, managedResources)
			conditions.runtimeComponentsHealthy = v1beta1helper.NewConditionOrError(h.clock, conditions.runtimeComponentsHealthy, newRuntimeComponentsCondition, nil)
			return nil
		},
		func(ctx context.Context) error {
			newVirtualComponentsCondition, err := h.checkVirtualComponents(ctx, conditions.virtualComponentsHealthy, managedResources)
			conditions.virtualComponentsHealthy = v1beta1helper.NewConditionOrError(h.clock, conditions.virtualComponentsHealthy, newVirtualComponentsCondition, err)
			return nil
		},
		func(_ context.Context) error {
			newObservabilityCondition := h.checkObservabilityComponents(conditions.observabilityComponentsHealthy, managedResources)
			conditions.observabilityComponentsHealthy = v1beta1helper.NewConditionOrError(h.clock, conditions.observabilityComponentsHealthy, newObservabilityCondition, nil)
			return nil
		},
	}

	_ = flow.Parallel(taskFns...)(ctx)

	return conditions.ConvertToSlice()
}

func (h *health) listManagedResources(ctx context.Context) ([]resourcesv1alpha1.ManagedResource, error) {
	managedResourceListGarden := &resourcesv1alpha1.ManagedResourceList{}
	if err := h.runtimeClient.List(ctx, managedResourceListGarden, client.InNamespace(h.gardenNamespace)); err != nil {
		return nil, fmt.Errorf("failed listing ManagedResources in namespace %s: %w", h.gardenNamespace, err)
	}

	managedResourceListIstioSystem := &resourcesv1alpha1.ManagedResourceList{}
	if err := h.runtimeClient.List(ctx, managedResourceListIstioSystem, client.InNamespace(v1beta1constants.IstioSystemNamespace)); err != nil {
		return nil, fmt.Errorf("failed listing ManagedResources in namespace %s: %w", v1beta1constants.IstioSystemNamespace, err)
	}

	return append(managedResourceListGarden.Items, managedResourceListIstioSystem.Items...), nil
}

// checkAPIServerAvailability checks if the API server of a virtual garden is reachable and measures the response time.
func (h *health) checkAPIServerAvailability(ctx context.Context, condition gardencorev1beta1.Condition) gardencorev1beta1.Condition {
	if h.gardenClientSet == nil {
		return v1beta1helper.FailedCondition(h.clock, h.garden.Status.LastOperation, h.conditionThresholds, condition, "VirtualGardenAPIServerDown", "Could not reach virtual garden API server during client initialization.")
	}
	log := logf.FromContext(ctx)
	return kuberneteshealth.CheckAPIServerAvailability(ctx, h.clock, log, condition, h.gardenClientSet.RESTClient(), func(conditionType, message string) gardencorev1beta1.Condition {
		return v1beta1helper.FailedCondition(h.clock, h.garden.Status.LastOperation, h.conditionThresholds, condition, conditionType, message)
	})
}

func (h *health) checkRuntimeComponents(condition gardencorev1beta1.Condition, managedResources []resourcesv1alpha1.ManagedResource) *gardencorev1beta1.Condition {
	if exitCondition := h.healthChecker.CheckManagedResources(condition, managedResources, func(managedResource resourcesv1alpha1.ManagedResource) bool {
		return managedResource.Spec.Class != nil &&
			sets.New("", string(operatorv1alpha1.RuntimeComponentsHealthy)).Has(managedResource.Labels[v1beta1constants.LabelCareConditionType])
	}, nil); exitCondition != nil {
		return exitCondition
	}

	return ptr.To(v1beta1helper.UpdatedConditionWithClock(h.clock, condition, gardencorev1beta1.ConditionTrue, "RuntimeComponentsRunning", "All runtime components are healthy."))
}

func (h *health) checkVirtualComponents(ctx context.Context, condition gardencorev1beta1.Condition, managedResources []resourcesv1alpha1.ManagedResource) (*gardencorev1beta1.Condition, error) {
	if exitCondition, err := h.healthChecker.CheckControlPlane(
		ctx,
		h.gardenNamespace,
		sets.New(virtualGardenPrefix+v1beta1constants.DeploymentNameGardenerResourceManager, virtualGardenPrefix+v1beta1constants.DeploymentNameKubeAPIServer, virtualGardenPrefix+v1beta1constants.DeploymentNameKubeControllerManager),
		sets.New(virtualGardenPrefix+v1beta1constants.ETCDMain, virtualGardenPrefix+v1beta1constants.ETCDEvents),
		condition,
	); err != nil || exitCondition != nil {
		return exitCondition, err
	}

	if exitCondition := h.healthChecker.CheckManagedResources(condition, managedResources, func(managedResource resourcesv1alpha1.ManagedResource) bool {
		return managedResource.Spec.Class == nil ||
			managedResource.Labels[v1beta1constants.LabelCareConditionType] == string(operatorv1alpha1.VirtualComponentsHealthy)
	}, nil); exitCondition != nil {
		return exitCondition, nil
	}

	return ptr.To(v1beta1helper.UpdatedConditionWithClock(h.clock, condition, gardencorev1beta1.ConditionTrue, "VirtualComponentsRunning", "All virtual garden components are healthy.")), nil
}

// checkObservabilityComponents checks whether the observability components are healthy.
func (h *health) checkObservabilityComponents(condition gardencorev1beta1.Condition, managedResources []resourcesv1alpha1.ManagedResource) *gardencorev1beta1.Condition {
	if exitCondition := h.healthChecker.CheckManagedResources(condition, managedResources, func(managedResource resourcesv1alpha1.ManagedResource) bool {
		return managedResource.Labels[v1beta1constants.LabelCareConditionType] == string(operatorv1alpha1.ObservabilityComponentsHealthy)
	}, nil); exitCondition != nil {
		return exitCondition
	}

	return ptr.To(v1beta1helper.UpdatedConditionWithClock(h.clock, condition, gardencorev1beta1.ConditionTrue, "ObservabilityComponentsRunning", "All observability components are healthy."))
}

// GardenConditions contains all conditions of the garden status subresource.
type GardenConditions struct {
	virtualGardenAPIServerAvailable gardencorev1beta1.Condition
	runtimeComponentsHealthy        gardencorev1beta1.Condition
	virtualComponentsHealthy        gardencorev1beta1.Condition
	observabilityComponentsHealthy  gardencorev1beta1.Condition
}

// ConvertToSlice returns the garden conditions as a slice.
func (g GardenConditions) ConvertToSlice() []gardencorev1beta1.Condition {
	return []gardencorev1beta1.Condition{
		g.virtualGardenAPIServerAvailable,
		g.runtimeComponentsHealthy,
		g.virtualComponentsHealthy,
		g.observabilityComponentsHealthy,
	}
}

// ConditionTypes returns all garden condition types.
func (g GardenConditions) ConditionTypes() []gardencorev1beta1.ConditionType {
	return []gardencorev1beta1.ConditionType{
		g.virtualGardenAPIServerAvailable.Type,
		g.runtimeComponentsHealthy.Type,
		g.virtualComponentsHealthy.Type,
		g.observabilityComponentsHealthy.Type,
	}
}

// NewGardenConditions returns a new instance of GardenConditions.
// All conditions are retrieved from the given 'status' or newly initialized.
func NewGardenConditions(clock clock.Clock, status operatorv1alpha1.GardenStatus) GardenConditions {
	return GardenConditions{
		virtualGardenAPIServerAvailable: v1beta1helper.GetOrInitConditionWithClock(clock, status.Conditions, operatorv1alpha1.VirtualGardenAPIServerAvailable),
		runtimeComponentsHealthy:        v1beta1helper.GetOrInitConditionWithClock(clock, status.Conditions, operatorv1alpha1.RuntimeComponentsHealthy),
		virtualComponentsHealthy:        v1beta1helper.GetOrInitConditionWithClock(clock, status.Conditions, operatorv1alpha1.VirtualComponentsHealthy),
		observabilityComponentsHealthy:  v1beta1helper.GetOrInitConditionWithClock(clock, status.Conditions, operatorv1alpha1.ObservabilityComponentsHealthy),
	}
}

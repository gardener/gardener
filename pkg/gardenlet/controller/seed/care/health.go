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

package care

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/clock"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/component/autoscaling/clusterautoscaler"
	"github.com/gardener/gardener/pkg/component/autoscaling/hvpa"
	"github.com/gardener/gardener/pkg/component/autoscaling/vpa"
	"github.com/gardener/gardener/pkg/component/clusteridentity"
	"github.com/gardener/gardener/pkg/component/etcd/etcd"
	"github.com/gardener/gardener/pkg/component/networking/istio"
	"github.com/gardener/gardener/pkg/component/networking/nginxingress"
	"github.com/gardener/gardener/pkg/component/nodemanagement/dependencywatchdog"
	"github.com/gardener/gardener/pkg/component/observability/logging/fluentoperator"
	valiconstants "github.com/gardener/gardener/pkg/component/observability/logging/vali/constants"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/kubestatemetrics"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/prometheusoperator"
	seedsystem "github.com/gardener/gardener/pkg/component/seed/system"
	"github.com/gardener/gardener/pkg/features"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	healthchecker "github.com/gardener/gardener/pkg/utils/kubernetes/health/checker"
)

var requiredManagedResourcesSeed = sets.New(
	etcd.Druid,
	clusterautoscaler.ManagedResourceControlName,
	kubestatemetrics.ManagedResourceName,
	nginxingress.ManagedResourceName,
	seedsystem.ManagedResourceName,
	vpa.ManagedResourceControlName,
	prometheusoperator.ManagedResourceName,
	"prometheus-cache",
	"prometheus-seed",
	"prometheus-aggregate",
)

// health contains information needed to execute health checks for a seed.
type health struct {
	seed                *gardencorev1beta1.Seed
	seedClient          client.Client
	clock               clock.Clock
	namespace           *string
	seedIsGarden        bool
	loggingEnabled      bool
	valiEnabled         bool
	alertManagerEnabled bool
	conditionThresholds map[gardencorev1beta1.ConditionType]time.Duration
	healthChecker       *healthchecker.HealthChecker
}

// NewHealth creates a new Health instance with the given parameters.
func NewHealth(
	seed *gardencorev1beta1.Seed,
	seedClient client.Client,
	clock clock.Clock,
	namespace *string,
	seedIsGarden bool,
	loggingEnabled bool,
	valiEnabled bool,
	alertManagerEnabled bool,
	conditionThresholds map[gardencorev1beta1.ConditionType]time.Duration,
) HealthCheck {
	return &health{
		seedClient:          seedClient,
		seed:                seed,
		clock:               clock,
		namespace:           namespace,
		seedIsGarden:        seedIsGarden,
		loggingEnabled:      loggingEnabled,
		valiEnabled:         valiEnabled,
		alertManagerEnabled: alertManagerEnabled,
		conditionThresholds: conditionThresholds,
		healthChecker:       healthchecker.NewHealthChecker(seedClient, clock, conditionThresholds, seed.Status.LastOperation),
	}
}

// Check conducts the health checks on all the given conditions.
func (h *health) Check(
	ctx context.Context,
	conditions SeedConditions,
) []gardencorev1beta1.Condition {
	managedResources, err := h.listManagedResources(ctx)
	if err != nil {
		conditions.systemComponentsHealthy = v1beta1helper.NewConditionOrError(h.clock, conditions.systemComponentsHealthy, nil, err)
		return conditions.ConvertToSlice()
	}

	newSystemComponentsCondition := h.checkSystemComponents(conditions.systemComponentsHealthy, managedResources)
	return []gardencorev1beta1.Condition{v1beta1helper.NewConditionOrError(h.clock, conditions.systemComponentsHealthy, newSystemComponentsCondition, nil)}
}

func (h *health) listManagedResources(ctx context.Context) ([]resourcesv1alpha1.ManagedResource, error) {
	managedResourceListGarden := &resourcesv1alpha1.ManagedResourceList{}
	if err := h.seedClient.List(ctx, managedResourceListGarden, client.InNamespace(ptr.Deref(h.namespace, v1beta1constants.GardenNamespace))); err != nil {
		return nil, fmt.Errorf("failed listing ManagedResources in namespace %s: %w", ptr.Deref(h.namespace, v1beta1constants.GardenNamespace), err)
	}

	managedResourceListIstioSystem := &resourcesv1alpha1.ManagedResourceList{}
	if err := h.seedClient.List(ctx, managedResourceListIstioSystem, client.InNamespace(ptr.Deref(h.namespace, v1beta1constants.IstioSystemNamespace))); err != nil {
		return nil, fmt.Errorf("failed listing ManagedResources in namespace %s: %w", ptr.Deref(h.namespace, v1beta1constants.IstioSystemNamespace), err)
	}

	return append(managedResourceListGarden.Items, managedResourceListIstioSystem.Items...), nil
}

func (h *health) checkSystemComponents(condition gardencorev1beta1.Condition, managedResources []resourcesv1alpha1.ManagedResource) *gardencorev1beta1.Condition {
	if exitCondition := h.healthChecker.CheckManagedResources(condition, managedResources, func(managedResource resourcesv1alpha1.ManagedResource) bool {
		return managedResource.Spec.Class != nil
	}, nil); exitCondition != nil {
		return exitCondition
	}

	return ptr.To(v1beta1helper.UpdatedConditionWithClock(h.clock, condition, gardencorev1beta1.ConditionTrue, "SystemComponentsRunning", "All system components are healthy."))
}

// SeedConditions contains all seed related conditions of the seed status subresource.
type SeedConditions struct {
	systemComponentsHealthy gardencorev1beta1.Condition
}

// ConvertToSlice returns the seed conditions as a slice.
func (s SeedConditions) ConvertToSlice() []gardencorev1beta1.Condition {
	return []gardencorev1beta1.Condition{
		s.systemComponentsHealthy,
	}
}

// ConditionTypes returns all seed condition types.
func (s SeedConditions) ConditionTypes() []gardencorev1beta1.ConditionType {
	return []gardencorev1beta1.ConditionType{
		s.systemComponentsHealthy.Type,
	}
}

// NewSeedConditions returns a new instance of SeedConditions.
// All conditions are retrieved from the given 'status' or newly initialized.
func NewSeedConditions(clock clock.Clock, status gardencorev1beta1.SeedStatus) SeedConditions {
	return SeedConditions{
		systemComponentsHealthy: v1beta1helper.GetOrInitConditionWithClock(clock, status.Conditions, gardencorev1beta1.SeedSystemComponentsHealthy),
	}
}

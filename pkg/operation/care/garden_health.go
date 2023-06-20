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

package care

import (
	"context"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/clock"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/component/etcd"
	"github.com/gardener/gardener/pkg/component/gardeneraccess"
	"github.com/gardener/gardener/pkg/component/gardensystem"
	"github.com/gardener/gardener/pkg/component/hvpa"
	"github.com/gardener/gardener/pkg/component/istio"
	"github.com/gardener/gardener/pkg/component/kubecontrollermanager"
	"github.com/gardener/gardener/pkg/component/kubestatemetrics"
	"github.com/gardener/gardener/pkg/component/resourcemanager"
	"github.com/gardener/gardener/pkg/component/vpa"
	"github.com/gardener/gardener/pkg/features"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

var (
	requiredGardenRuntimeManagedResources = sets.New(
		etcd.Druid,
		kubestatemetrics.ManagedResourceName,
		gardensystem.ManagedResourceName,
	)

	requiredVirtualGardenManagedResources = sets.New(
		resourcemanager.ManagedResourceName,
		gardeneraccess.ManagedResourceName,
		kubecontrollermanager.ManagedResourceName,
	)
)

// GardenHealth contains information needed to execute health checks for garden.
type GardenHealth struct {
	garden          *operatorv1alpha1.Garden
	gardenNamespace string
	runtimeClient   client.Client
	clock           clock.Clock
}

// NewHealthForGarden creates a new Health instance with the given parameters.
func NewHealthForGarden(garden *operatorv1alpha1.Garden, runtimeClient client.Client, clock clock.Clock, gardenNamespace string) *GardenHealth {
	return &GardenHealth{
		runtimeClient:   runtimeClient,
		garden:          garden,
		clock:           clock,
		gardenNamespace: gardenNamespace,
	}
}

// CheckGarden conducts the health checks on all the given conditions.
func (h *GardenHealth) CheckGarden(
	ctx context.Context,
	conditions []gardencorev1beta1.Condition,
	thresholdMappings map[gardencorev1beta1.ConditionType]time.Duration,
) []gardencorev1beta1.Condition {

	var (
		systemComponentsCondition        gardencorev1beta1.Condition
		virtualGardenComponentsCondition gardencorev1beta1.Condition
	)
	for _, cond := range conditions {
		switch cond.Type {
		case operatorv1alpha1.GardenSystemComponentsHealthy:
			systemComponentsCondition = cond
		case operatorv1alpha1.VirtualGardenComponentsHealthy:
			virtualGardenComponentsCondition = cond
		}
	}

	checker := NewHealthChecker(h.runtimeClient, h.clock, thresholdMappings, nil, nil, nil, nil, nil)
	newSystemComponentsCondition, err := h.checkGardenSystemComponents(ctx, checker, systemComponentsCondition)
	newVirtualGardenComponentsCondition, err2 := h.checkVirtualGardenComponents(ctx, checker, virtualGardenComponentsCondition)

	return []gardencorev1beta1.Condition{
		NewConditionOrError(h.clock, systemComponentsCondition, newSystemComponentsCondition, err),
		NewConditionOrError(h.clock, virtualGardenComponentsCondition, newVirtualGardenComponentsCondition, err2),
	}
}

func (h *GardenHealth) checkGardenSystemComponents(
	ctx context.Context,
	checker *HealthChecker,
	condition gardencorev1beta1.Condition,
) (
	*gardencorev1beta1.Condition,
	error,
) {
	managedResources := sets.List(requiredGardenRuntimeManagedResources)
	managedResources = append(managedResources, istio.ManagedResourceNames(true, "virtual-garden-")...)

	if features.DefaultFeatureGate.Enabled(features.HVPA) {
		managedResources = append(managedResources, hvpa.ManagedResourceName)
	}
	if h.garden.Spec.RuntimeCluster.Settings != nil &&
		h.garden.Spec.RuntimeCluster.Settings.VerticalPodAutoscaler != nil &&
		pointer.BoolDeref(h.garden.Spec.RuntimeCluster.Settings.VerticalPodAutoscaler.Enabled, false) {
		managedResources = append(managedResources, vpa.ManagedResourceControlName)
	}

	for _, name := range managedResources {
		namespace := h.gardenNamespace
		if sets.New(istio.ManagedResourceNames(true, "virtual-garden-")...).Has(name) {
			namespace = v1beta1constants.IstioSystemNamespace
		}

		mr := &resourcesv1alpha1.ManagedResource{}
		if err := h.runtimeClient.Get(ctx, kubernetesutils.Key(namespace, name), mr); err != nil {
			if apierrors.IsNotFound(err) {
				exitCondition := checker.FailedCondition(condition, "ResourceNotFound", err.Error())
				return &exitCondition, nil
			}
			return nil, err
		}

		if exitCondition := checkManagedResourceForGarden(checker, condition, mr); exitCondition != nil {
			return exitCondition, nil
		}
	}

	c := v1beta1helper.UpdatedConditionWithClock(h.clock, condition, gardencorev1beta1.ConditionTrue, "SystemComponentsRunning", "All system components are healthy.")
	return &c, nil
}

func (h *GardenHealth) checkVirtualGardenComponents(
	ctx context.Context,
	checker *HealthChecker,
	condition gardencorev1beta1.Condition,
) (
	*gardencorev1beta1.Condition,
	error,
) {
	managedResources := sets.List(requiredVirtualGardenManagedResources)

	for _, name := range managedResources {
		namespace := h.gardenNamespace
		mr := &resourcesv1alpha1.ManagedResource{}
		if err := h.runtimeClient.Get(ctx, kubernetesutils.Key(namespace, name), mr); err != nil {
			if apierrors.IsNotFound(err) {
				exitCondition := checker.FailedCondition(condition, "ResourceNotFound", err.Error())
				return &exitCondition, nil
			}
			return nil, err
		}

		if exitCondition := checkManagedResourceForGarden(checker, condition, mr); exitCondition != nil {
			return exitCondition, nil
		}
	}

	c := v1beta1helper.UpdatedConditionWithClock(h.clock, condition, gardencorev1beta1.ConditionTrue, "VirtualGardenComponentsRunning", "All virtual garden components are healthy.")
	return &c, nil
}

func checkManagedResourceForGarden(checker *HealthChecker, condition gardencorev1beta1.Condition, managedResource *resourcesv1alpha1.ManagedResource) *gardencorev1beta1.Condition {
	conditionsToCheck := map[gardencorev1beta1.ConditionType]func(condition gardencorev1beta1.Condition) bool{
		resourcesv1alpha1.ResourcesApplied:     defaultSuccessfulCheck(),
		resourcesv1alpha1.ResourcesHealthy:     defaultSuccessfulCheck(),
		resourcesv1alpha1.ResourcesProgressing: resourcesNotProgressingCheck(checker.clock, nil),
	}

	return checker.checkManagedResourceConditions(condition, managedResource, conditionsToCheck)
}

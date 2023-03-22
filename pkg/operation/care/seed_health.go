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
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/clock"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/features"
	gardenletfeatures "github.com/gardener/gardener/pkg/gardenlet/features"
	"github.com/gardener/gardener/pkg/operation/botanist/component/clusterautoscaler"
	"github.com/gardener/gardener/pkg/operation/botanist/component/clusteridentity"
	"github.com/gardener/gardener/pkg/operation/botanist/component/dependencywatchdog"
	"github.com/gardener/gardener/pkg/operation/botanist/component/etcd"
	"github.com/gardener/gardener/pkg/operation/botanist/component/hvpa"
	"github.com/gardener/gardener/pkg/operation/botanist/component/istio"
	"github.com/gardener/gardener/pkg/operation/botanist/component/kubestatemetrics"
	"github.com/gardener/gardener/pkg/operation/botanist/component/nginxingress"
	"github.com/gardener/gardener/pkg/operation/botanist/component/seedsystem"
	"github.com/gardener/gardener/pkg/operation/botanist/component/vpa"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

var requiredManagedResourcesSeed = sets.New[string](
	etcd.Druid,
	clusterautoscaler.ManagedResourceControlName,
	kubestatemetrics.ManagedResourceName,
	seedsystem.ManagedResourceName,
	vpa.ManagedResourceControlName,
)

// SeedHealth contains information needed to execute health checks for seed.
type SeedHealth struct {
	seed       *gardencorev1beta1.Seed
	seedClient client.Client
	clock      clock.Clock
	namespace  *string
}

// NewHealthForSeed creates a new Health instance with the given parameters.
func NewHealthForSeed(seed *gardencorev1beta1.Seed, seedClient client.Client, clock clock.Clock, namespace *string) *SeedHealth {
	return &SeedHealth{
		seedClient: seedClient,
		seed:       seed,
		clock:      clock,
		namespace:  namespace,
	}
}

// CheckSeed conducts the health checks on all the given conditions.
func (h *SeedHealth) CheckSeed(
	ctx context.Context,
	conditions []gardencorev1beta1.Condition,
	thresholdMappings map[gardencorev1beta1.ConditionType]time.Duration,
) []gardencorev1beta1.Condition {

	var systemComponentsCondition gardencorev1beta1.Condition
	for _, cond := range conditions {
		switch cond.Type {
		case gardencorev1beta1.SeedSystemComponentsHealthy:
			systemComponentsCondition = cond
		}
	}

	checker := NewHealthChecker(h.seedClient, h.clock, thresholdMappings, nil, nil, nil, nil, nil)
	newSystemComponentsCondition, err := h.checkSeedSystemComponents(ctx, checker, systemComponentsCondition)
	return []gardencorev1beta1.Condition{NewConditionOrError(h.clock, systemComponentsCondition, newSystemComponentsCondition, err)}
}

func (h *SeedHealth) checkSeedSystemComponents(
	ctx context.Context,
	checker *HealthChecker,
	condition gardencorev1beta1.Condition,
) (
	*gardencorev1beta1.Condition,
	error,
) {
	managedResources := sets.List(requiredManagedResourcesSeed)

	seedIsOriginOfClusterIdentity, err := clusteridentity.IsClusterIdentityEmptyOrFromOrigin(ctx, h.seedClient, v1beta1constants.ClusterIdentityOriginSeed)
	if err != nil {
		return nil, err
	}
	if seedIsOriginOfClusterIdentity {
		managedResources = append(managedResources, clusteridentity.ManagedResourceControlName)
	}

	if gardenletfeatures.FeatureGate.Enabled(features.ManagedIstio) {
		managedResources = append(managedResources, istio.ManagedResourceControlName)
		managedResources = append(managedResources, istio.ManagedResourceIstioSystemName)
	}
	if gardenletfeatures.FeatureGate.Enabled(features.HVPA) {
		managedResources = append(managedResources, hvpa.ManagedResourceName)
	}
	if v1beta1helper.SeedSettingDependencyWatchdogEndpointEnabled(h.seed.Spec.Settings) {
		managedResources = append(managedResources, dependencywatchdog.ManagedResourceDependencyWatchdogEndpoint)
	}
	if v1beta1helper.SeedSettingDependencyWatchdogProbeEnabled(h.seed.Spec.Settings) {
		managedResources = append(managedResources, dependencywatchdog.ManagedResourceDependencyWatchdogProbe)
	}
	if v1beta1helper.SeedUsesNginxIngressController(h.seed) {
		managedResources = append(managedResources, nginxingress.ManagedResourceName)
	}

	for _, name := range managedResources {
		namespace := v1beta1constants.GardenNamespace
		if name == istio.ManagedResourceControlName || name == istio.ManagedResourceIstioSystemName {
			namespace = v1beta1constants.IstioSystemNamespace
		}
		namespace = pointer.StringDeref(h.namespace, namespace)

		mr := &resourcesv1alpha1.ManagedResource{}
		if err := h.seedClient.Get(ctx, kubernetesutils.Key(namespace, name), mr); err != nil {
			if apierrors.IsNotFound(err) {
				exitCondition := checker.FailedCondition(condition, "ResourceNotFound", err.Error())
				return &exitCondition, nil
			}
			return nil, err
		}

		if exitCondition := checkManagedResourceForSeed(checker, condition, mr); exitCondition != nil {
			return exitCondition, nil
		}
	}

	c := v1beta1helper.UpdatedConditionWithClock(h.clock, condition, gardencorev1beta1.ConditionTrue, "SystemComponentsRunning", "All system components are healthy.")
	return &c, nil
}

func checkManagedResourceForSeed(checker *HealthChecker, condition gardencorev1beta1.Condition, managedResource *resourcesv1alpha1.ManagedResource) *gardencorev1beta1.Condition {
	conditionsToCheck := map[gardencorev1beta1.ConditionType]func(condition gardencorev1beta1.Condition) bool{
		resourcesv1alpha1.ResourcesApplied:     defaultSuccessfulCheck(),
		resourcesv1alpha1.ResourcesHealthy:     defaultSuccessfulCheck(),
		resourcesv1alpha1.ResourcesProgressing: resourcesNotProgressingCheck(checker.clock, nil),
	}

	return checker.checkManagedResourceConditions(condition, managedResource, conditionsToCheck)
}

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
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
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
	"github.com/gardener/gardener/pkg/utils/flow"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
)

const virtualGardenPrefix = "virtual-garden-"

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

	requiredVirtualGardenControlPlaneDeployments = sets.New(
		virtualGardenPrefix+v1beta1constants.DeploymentNameGardenerResourceManager,
		virtualGardenPrefix+v1beta1constants.DeploymentNameKubeAPIServer,
		virtualGardenPrefix+v1beta1constants.DeploymentNameKubeControllerManager,
	)

	requiredVirtualGardenControlPlaneEtcds = sets.New(
		virtualGardenPrefix+v1beta1constants.ETCDMain,
		virtualGardenPrefix+v1beta1constants.ETCDEvents,
	)
)

// GardenHealth contains information needed to execute health checks for garden.
type GardenHealth struct {
	garden          *operatorv1alpha1.Garden
	gardenNamespace string
	runtimeClient   client.Client
	gardenClientSet kubernetes.Interface
	clock           clock.Clock
}

// NewHealthForGarden creates a new Health instance with the given parameters.
func NewHealthForGarden(garden *operatorv1alpha1.Garden, runtimeClient client.Client, gardenClientSet kubernetes.Interface, clock clock.Clock, gardenNamespace string) *GardenHealth {
	return &GardenHealth{
		garden:          garden,
		gardenNamespace: gardenNamespace,
		runtimeClient:   runtimeClient,
		gardenClientSet: gardenClientSet,
		clock:           clock,
	}
}

// CheckGarden conducts the health checks on all the given conditions.
func (h *GardenHealth) CheckGarden(
	ctx context.Context,
	conditions []gardencorev1beta1.Condition,
	thresholdMappings map[gardencorev1beta1.ConditionType]time.Duration,
) []gardencorev1beta1.Condition {

	var (
		apiServerAvailability            gardencorev1beta1.Condition
		controlPlaneHealthy              gardencorev1beta1.Condition
		systemComponentsCondition        gardencorev1beta1.Condition
		virtualGardenComponentsCondition gardencorev1beta1.Condition
	)
	for _, cond := range conditions {
		switch cond.Type {
		case operatorv1alpha1.VirtualGardenAPIServerAvailable:
			apiServerAvailability = cond
		case operatorv1alpha1.VirtualGardenControlPlaneHealthy:
			controlPlaneHealthy = cond
		case operatorv1alpha1.GardenSystemComponentsHealthy:
			systemComponentsCondition = cond
		case operatorv1alpha1.VirtualGardenComponentsHealthy:
			virtualGardenComponentsCondition = cond
		}
	}

	checker := NewHealthChecker(h.runtimeClient, h.clock, thresholdMappings, nil, nil, nil, nil, nil)

	taskFns := []flow.TaskFn{
		func(ctx context.Context) error {
			apiServerAvailability = h.checkAPIServerAvailability(ctx, checker, apiServerAvailability)
			return nil
		},
		func(ctx context.Context) error {
			newSystemComponentsCondition, err := h.checkGardenSystemComponents(ctx, checker, systemComponentsCondition)
			systemComponentsCondition = NewConditionOrError(h.clock, systemComponentsCondition, newSystemComponentsCondition, err)
			return nil
		},
		func(ctx context.Context) error {
			newVirtualGardenComponentsCondition, err := h.checkVirtualGardenComponents(ctx, checker, virtualGardenComponentsCondition)
			virtualGardenComponentsCondition = NewConditionOrError(h.clock, virtualGardenComponentsCondition, newVirtualGardenComponentsCondition, err)
			return nil
		},
		func(ctx context.Context) error {
			newControlPlaneHealthy, err := h.checkControlPlane(ctx, checker, controlPlaneHealthy)
			controlPlaneHealthy = NewConditionOrError(h.clock, controlPlaneHealthy, newControlPlaneHealthy, err)
			return nil
		},
	}

	_ = flow.Parallel(taskFns...)(ctx)

	return []gardencorev1beta1.Condition{
		systemComponentsCondition,
		virtualGardenComponentsCondition,
		controlPlaneHealthy,
		apiServerAvailability,
	}
}

// checkAPIServerAvailability checks if the API server of a virtual garden is reachable and measure the response time.
func (h *GardenHealth) checkAPIServerAvailability(ctx context.Context, checker *HealthChecker, condition gardencorev1beta1.Condition) gardencorev1beta1.Condition {
	if h.gardenClientSet == nil {
		return checker.FailedCondition(condition, "VirtualGardenAPIServerDown", "Could not reach virtual garden API server during client initialization.")
	}
	log := logf.FromContext(ctx)
	return health.CheckAPIServerAvailability(ctx, h.clock, log, condition, h.gardenClientSet.RESTClient(), func(conditionType, message string) gardencorev1beta1.Condition {
		return checker.FailedCondition(condition, conditionType, message)
	})
}

// checkControlPlane checks whether the core components of the virtual garden controlplane (ETCD, KAPI, KCM..) are healthy.
func (h *GardenHealth) checkControlPlane(ctx context.Context, checker *HealthChecker, condition gardencorev1beta1.Condition) (*gardencorev1beta1.Condition, error) {
	if exitCondition, err := checker.CheckControlPlane(
		ctx,
		h.gardenNamespace,
		requiredVirtualGardenControlPlaneDeployments,
		requiredVirtualGardenControlPlaneEtcds,
		condition,
	); err != nil || exitCondition != nil {
		return exitCondition, err
	}

	c := v1beta1helper.UpdatedConditionWithClock(h.clock, condition, gardencorev1beta1.ConditionTrue, "VirtualGardenControlPlaneRunning", "All control plane components are healthy.")
	return &c, nil
}

func (h *GardenHealth) checkGardenSystemComponents(ctx context.Context, checker *HealthChecker, condition gardencorev1beta1.Condition) (*gardencorev1beta1.Condition, error) {
	managedResources := sets.List(requiredGardenRuntimeManagedResources)
	managedResources = append(managedResources, istio.ManagedResourceNames(true, "virtual-garden-")...)

	if features.DefaultFeatureGate.Enabled(features.HVPA) {
		managedResources = append(managedResources, hvpa.ManagedResourceName)
	}
	if h.isVPAEnabled() {
		managedResources = append(managedResources, vpa.ManagedResourceControlName)
	}

	return h.checkManagedResources(ctx, checker, condition, managedResources, "SystemComponentsRunning", "All system components are healthy.")
}

func (h *GardenHealth) checkVirtualGardenComponents(ctx context.Context, checker *HealthChecker, condition gardencorev1beta1.Condition) (*gardencorev1beta1.Condition, error) {
	managedResources := sets.List(requiredVirtualGardenManagedResources)

	return h.checkManagedResources(ctx, checker, condition, managedResources, "VirtualGardenComponentsRunning", "All virtual garden components are healthy.")
}

func (h *GardenHealth) checkManagedResources(
	ctx context.Context,
	checker *HealthChecker,
	condition gardencorev1beta1.Condition,
	managedResources []string,
	successReason string,
	successMessage string,
) (
	*gardencorev1beta1.Condition,
	error,
) {
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
	c := v1beta1helper.UpdatedConditionWithClock(h.clock, condition, gardencorev1beta1.ConditionTrue, successReason, successMessage)
	return &c, nil
}

func (h *GardenHealth) isVPAEnabled() bool {
	if h.garden.Spec.RuntimeCluster.Settings != nil &&
		h.garden.Spec.RuntimeCluster.Settings.VerticalPodAutoscaler != nil &&
		pointer.BoolDeref(h.garden.Spec.RuntimeCluster.Settings.VerticalPodAutoscaler.Enabled, false) {
		return true
	}
	return false
}

func checkManagedResourceForGarden(checker *HealthChecker, condition gardencorev1beta1.Condition, managedResource *resourcesv1alpha1.ManagedResource) *gardencorev1beta1.Condition {
	conditionsToCheck := map[gardencorev1beta1.ConditionType]func(condition gardencorev1beta1.Condition) bool{
		resourcesv1alpha1.ResourcesApplied:     defaultSuccessfulCheck(),
		resourcesv1alpha1.ResourcesHealthy:     defaultSuccessfulCheck(),
		resourcesv1alpha1.ResourcesProgressing: resourcesNotProgressingCheck(checker.clock, nil),
	}

	return checker.checkManagedResourceConditions(condition, managedResource, conditionsToCheck)
}

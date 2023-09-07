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
	"k8s.io/apimachinery/pkg/labels"
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
	"github.com/gardener/gardener/pkg/component/gardeneradmissioncontroller"
	"github.com/gardener/gardener/pkg/component/gardenerapiserver"
	"github.com/gardener/gardener/pkg/component/gardenercontrollermanager"
	"github.com/gardener/gardener/pkg/component/gardenerscheduler"
	runtimegardensystem "github.com/gardener/gardener/pkg/component/gardensystem/runtime"
	virtualgardensystem "github.com/gardener/gardener/pkg/component/gardensystem/virtual"
	"github.com/gardener/gardener/pkg/component/hvpa"
	"github.com/gardener/gardener/pkg/component/istio"
	"github.com/gardener/gardener/pkg/component/kubecontrollermanager"
	"github.com/gardener/gardener/pkg/component/kubestatemetrics"
	"github.com/gardener/gardener/pkg/component/logging/fluentoperator"
	"github.com/gardener/gardener/pkg/component/logging/vali"
	"github.com/gardener/gardener/pkg/component/resourcemanager"
	"github.com/gardener/gardener/pkg/component/vpa"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/utils/flow"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	kuberneteshealth "github.com/gardener/gardener/pkg/utils/kubernetes/health"
	healthchecker "github.com/gardener/gardener/pkg/utils/kubernetes/health/checker"
)

const virtualGardenPrefix = "virtual-garden-"

var (
	requiredGardenRuntimeManagedResources = sets.New(
		etcd.Druid,
		runtimegardensystem.ManagedResourceName,
		kubestatemetrics.ManagedResourceName,
		fluentoperator.OperatorManagedResourceName,
		fluentoperator.CustomResourcesManagedResourceName+"-garden",
		fluentoperator.FluentBitManagedResourceName,
		vali.ManagedResourceNameRuntime,
	)

	requiredVirtualGardenManagedResources = sets.New(
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

	requiredMonitoringDeployments = sets.New(
		v1beta1constants.DeploymentNameKubeStateMetrics,
		v1beta1constants.DeploymentNamePlutono,
		v1beta1constants.DeploymentNameGardenerMetricsExporter,
	)

	virtualGardenMonitoringSelector = labels.SelectorFromSet(map[string]string{v1beta1constants.LabelRole: v1beta1constants.LabelMonitoring})
)

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
	taskFns := []flow.TaskFn{
		func(ctx context.Context) error {
			conditions.virtualGardenAPIServerAvailable = h.checkAPIServerAvailability(ctx, conditions.virtualGardenAPIServerAvailable)
			return nil
		},
		func(ctx context.Context) error {
			newRuntimeComponentsCondition, err := h.checkRuntimeComponents(ctx, conditions.runtimeComponentsHealthy)
			conditions.runtimeComponentsHealthy = v1beta1helper.NewConditionOrError(h.clock, conditions.runtimeComponentsHealthy, newRuntimeComponentsCondition, err)
			return nil
		},
		func(ctx context.Context) error {
			newVirtualComponentsCondition, err := h.checkVirtualComponents(ctx, conditions.virtualComponentsHealthy)
			conditions.virtualComponentsHealthy = v1beta1helper.NewConditionOrError(h.clock, conditions.virtualComponentsHealthy, newVirtualComponentsCondition, err)
			return nil
		},
		func(ctx context.Context) error {
			newObservabilityCondition, err := h.checkObservabilityComponents(ctx, conditions.observabilityComponentsHealthy)
			conditions.observabilityComponentsHealthy = v1beta1helper.NewConditionOrError(h.clock, conditions.observabilityComponentsHealthy, newObservabilityCondition, err)
			return nil
		},
	}

	_ = flow.Parallel(taskFns...)(ctx)

	return conditions.ConvertToSlice()
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

func (h *health) checkRuntimeComponents(ctx context.Context, condition gardencorev1beta1.Condition) (*gardencorev1beta1.Condition, error) {
	managedResources := sets.List(requiredGardenRuntimeManagedResources)
	managedResources = append(managedResources, istio.ManagedResourceNames(true, "virtual-garden-")...)

	if features.DefaultFeatureGate.Enabled(features.HVPA) {
		managedResources = append(managedResources, hvpa.ManagedResourceName)
	}
	if h.isVPAEnabled() {
		managedResources = append(managedResources, vpa.ManagedResourceControlName)
	}

	return h.checkManagedResources(ctx, condition, managedResources, "RuntimeComponentsRunning", "All runtime components are healthy.")
}

func (h *health) checkVirtualComponents(ctx context.Context, condition gardencorev1beta1.Condition) (*gardencorev1beta1.Condition, error) {
	if exitCondition, err := h.healthChecker.CheckControlPlane(
		ctx,
		h.gardenNamespace,
		requiredVirtualGardenControlPlaneDeployments,
		requiredVirtualGardenControlPlaneEtcds,
		condition,
	); err != nil || exitCondition != nil {
		return exitCondition, err
	}

	managedResources := sets.List(requiredVirtualGardenManagedResources)

	return h.checkManagedResources(ctx, condition, managedResources, "VirtualComponentsRunning", "All virtual garden components are healthy.")
}

func (h *health) checkManagedResources(
	ctx context.Context,
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
				exitCondition := v1beta1helper.FailedCondition(h.clock, h.garden.Status.LastOperation, h.conditionThresholds, condition, "ResourceNotFound", err.Error())
				return &exitCondition, nil
			}
			return nil, err
		}

		if exitCondition := h.healthChecker.CheckManagedResource(condition, mr, nil); exitCondition != nil {
			return exitCondition, nil
		}
	}
	c := v1beta1helper.UpdatedConditionWithClock(h.clock, condition, gardencorev1beta1.ConditionTrue, successReason, successMessage)
	return &c, nil
}

// checkObservabilityComponents checks whether the  observability components of the virtual garden control plane (Prometheus, Vali, Plutono..) are healthy.
func (h *health) checkObservabilityComponents(ctx context.Context, condition gardencorev1beta1.Condition) (*gardencorev1beta1.Condition, error) {
	requiredDeployments := requiredMonitoringDeployments.Clone()

	if exitCondition, err := h.healthChecker.CheckMonitoringControlPlane(ctx, h.gardenNamespace, requiredDeployments, sets.New[string](), virtualGardenMonitoringSelector, condition); err != nil || exitCondition != nil {
		return exitCondition, err
	}

	if exitCondition, err := h.healthChecker.CheckLoggingControlPlane(ctx, h.gardenNamespace, false, true, condition); err != nil || exitCondition != nil {
		return exitCondition, err
	}

	c := v1beta1helper.UpdatedConditionWithClock(h.clock, condition, gardencorev1beta1.ConditionTrue, "ObservabilityComponentsRunning", "All observability components are healthy.")
	return &c, nil
}

func (h *health) isVPAEnabled() bool {
	return h.garden.Spec.RuntimeCluster.Settings != nil &&
		h.garden.Spec.RuntimeCluster.Settings.VerticalPodAutoscaler != nil &&
		pointer.BoolDeref(h.garden.Spec.RuntimeCluster.Settings.VerticalPodAutoscaler.Enabled, false)
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

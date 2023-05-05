// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	"github.com/go-logr/logr"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	apiextensions "github.com/gardener/gardener/pkg/api/extensions"
	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/extensions"
	gardenletconfig "github.com/gardener/gardener/pkg/gardenlet/apis/config"
	gardenlethelper "github.com/gardener/gardener/pkg/gardenlet/apis/config/helper"
	"github.com/gardener/gardener/pkg/operation"
	"github.com/gardener/gardener/pkg/operation/botanist"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/operation/shoot"
	"github.com/gardener/gardener/pkg/utils/flow"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
)

// Health contains information needed to execute shoot health checks.
type Health struct {
	shoot        *shoot.Shoot
	gardenClient client.Client
	seedClient   kubernetes.Interface

	initializeShootClients ShootClientInit
	shootClient            kubernetes.Interface

	log logr.Logger

	gardenletConfiguration                    *gardenletconfig.GardenletConfiguration
	clock                                     clock.Clock
	controllerRegistrationToLastHeartbeatTime map[string]*metav1.MicroTime
}

// ShootClientInit is a function that initializes a kubernetes client for a Shoot.
type ShootClientInit func() (kubernetes.Interface, bool, error)

// NewHealth creates a new Health instance with the given parameters.
func NewHealth(op *operation.Operation, shootClientInit ShootClientInit, clock clock.Clock) *Health {
	return &Health{
		shoot:                  op.Shoot,
		gardenClient:           op.GardenClient,
		seedClient:             op.SeedClientSet,
		initializeShootClients: shootClientInit,
		shootClient:            op.ShootClientSet,
		clock:                  clock,
		log:                    op.Logger,
		gardenletConfiguration: op.Config,
		controllerRegistrationToLastHeartbeatTime: map[string]*metav1.MicroTime{},
	}
}

// Check conducts the health checks on all the given conditions.
func (h *Health) Check(
	ctx context.Context,
	thresholdMappings map[gardencorev1beta1.ConditionType]time.Duration,
	healthCheckOutdatedThreshold *metav1.Duration,
	conditions []gardencorev1beta1.Condition,
) []gardencorev1beta1.Condition {
	updatedConditions := h.healthChecks(ctx, thresholdMappings, healthCheckOutdatedThreshold, conditions)

	lastOp := h.shoot.GetInfo().Status.LastOperation
	lastErrors := h.shoot.GetInfo().Status.LastErrors
	return PardonConditions(h.clock, updatedConditions, lastOp, lastErrors)
}

// ExtensionCondition contains information about the extension type, name, namespace and the respective condition object.
type ExtensionCondition struct {
	Condition          gardencorev1beta1.Condition
	ExtensionType      string
	ExtensionName      string
	ExtensionNamespace string
	LastHeartbeatTime  *metav1.MicroTime
}

func (h *Health) getAllExtensionConditions(ctx context.Context) ([]ExtensionCondition, []ExtensionCondition, []ExtensionCondition, error) {
	objs, err := h.retrieveExtensions(ctx)
	if err != nil {
		return nil, nil, nil, err
	}

	controllerInstallations := &gardencorev1beta1.ControllerInstallationList{}
	if err := h.gardenClient.List(ctx, controllerInstallations, client.MatchingFields{core.SeedRefName: h.gardenletConfiguration.SeedConfig.Name}); err != nil {
		return nil, nil, nil, err
	}

	controllerRegistrations := &gardencorev1beta1.ControllerRegistrationList{}
	if err := h.gardenClient.List(ctx, controllerRegistrations); err != nil {
		return nil, nil, nil, err
	}

	var (
		conditionsControlPlaneHealthy     []ExtensionCondition
		conditionsEveryNodeReady          []ExtensionCondition
		conditionsSystemComponentsHealthy []ExtensionCondition
	)

	for _, obj := range objs {
		acc, err := apiextensions.Accessor(obj)
		if err != nil {
			return nil, nil, nil, err
		}

		gvk, err := apiutil.GVKForObject(obj, kubernetes.SeedScheme)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to identify GVK for object: %w", err)
		}

		kind := gvk.Kind
		name := acc.GetName()
		extensionType := acc.GetExtensionSpec().GetExtensionType()
		namespace := acc.GetNamespace()

		lastHeartbeatTime, err := h.getLastHeartbeatTimeForExtension(ctx, controllerInstallations, controllerRegistrations, kind, extensionType)
		if err != nil {
			return nil, nil, nil, err
		}

		for _, condition := range acc.GetExtensionStatus().GetConditions() {
			switch condition.Type {
			case gardencorev1beta1.ShootControlPlaneHealthy:
				conditionsControlPlaneHealthy = append(conditionsControlPlaneHealthy, ExtensionCondition{condition, kind, name, namespace, lastHeartbeatTime})
			case gardencorev1beta1.ShootEveryNodeReady:
				conditionsEveryNodeReady = append(conditionsEveryNodeReady, ExtensionCondition{condition, kind, name, namespace, lastHeartbeatTime})
			case gardencorev1beta1.ShootSystemComponentsHealthy:
				conditionsSystemComponentsHealthy = append(conditionsSystemComponentsHealthy, ExtensionCondition{condition, kind, name, namespace, lastHeartbeatTime})
			}
		}
	}

	return conditionsControlPlaneHealthy, conditionsEveryNodeReady, conditionsSystemComponentsHealthy, nil
}

func (h *Health) retrieveExtensions(ctx context.Context) ([]runtime.Object, error) {
	var (
		allExtensions       []runtime.Object
		extensionObjectList = []client.ObjectList{
			&extensionsv1alpha1.ExtensionList{},
		}
	)

	if !h.shoot.IsWorkerless {
		extensionObjectList = append(extensionObjectList,
			&extensionsv1alpha1.ControlPlaneList{},
			&extensionsv1alpha1.ContainerRuntimeList{},
			&extensionsv1alpha1.InfrastructureList{},
			&extensionsv1alpha1.NetworkList{},
			&extensionsv1alpha1.OperatingSystemConfigList{},
			&extensionsv1alpha1.WorkerList{},
		)
	}

	for _, listObj := range extensionObjectList {
		if err := h.seedClient.Client().List(ctx, listObj, client.InNamespace(h.shoot.SeedNamespace)); err != nil {
			return nil, err
		}

		if err := meta.EachListItem(listObj, func(obj runtime.Object) error {
			allExtensions = append(allExtensions, obj)
			return nil
		}); err != nil {
			return nil, fmt.Errorf("error during evaluation of kind %T for extensions health check: %w", listObj, err)
		}
	}

	// Get BackupEntries separately as they are not namespaced i.e., they cannot be narrowed down
	// to a shoot namespace like other extension resources above.
	be := &extensionsv1alpha1.BackupEntry{}
	if err := h.seedClient.Client().Get(ctx, kubernetesutils.Key(h.shoot.BackupEntryName), be); err == nil {
		allExtensions = append(allExtensions, be)
	} else if !apierrors.IsNotFound(err) {
		return nil, err
	}

	return allExtensions, nil
}

func (h *Health) getLastHeartbeatTimeForExtension(ctx context.Context, controllerInstallations *gardencorev1beta1.ControllerInstallationList, controllerRegistrations *gardencorev1beta1.ControllerRegistrationList, extensionKind, extensionType string) (*metav1.MicroTime, error) {
	controllerRegistration, err := getControllerRegistrationForExtensionKindAndType(controllerRegistrations, extensionKind, extensionType)
	if err != nil {
		return nil, err
	}

	if lastHeartbeatTime, exists := h.controllerRegistrationToLastHeartbeatTime[controllerRegistration.Name]; exists {
		return lastHeartbeatTime, nil
	}

	controllerInstallation, err := getControllerInstallationForControllerRegistration(controllerInstallations, controllerRegistration)
	if err != nil {
		return nil, err
	}

	heartBeatLease := &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      extensions.HeartBeatResourceName,
			Namespace: gardenerutils.NamespaceNameForControllerInstallation(controllerInstallation),
		},
	}

	if err := h.seedClient.Client().Get(ctx, client.ObjectKeyFromObject(heartBeatLease), heartBeatLease); err != nil {
		if apierrors.IsNotFound(err) {
			h.controllerRegistrationToLastHeartbeatTime[controllerRegistration.Name] = nil
			return nil, nil
		}
		return nil, err
	}

	h.controllerRegistrationToLastHeartbeatTime[controllerRegistration.Name] = heartBeatLease.Spec.RenewTime
	return heartBeatLease.Spec.RenewTime, nil
}

func getControllerRegistrationForExtensionKindAndType(controllerRegistrations *gardencorev1beta1.ControllerRegistrationList, extensionKind, extensionType string) (*gardencorev1beta1.ControllerRegistration, error) {
	for _, controllerRegistration := range controllerRegistrations.Items {
		for _, resource := range controllerRegistration.Spec.Resources {
			if resource.Kind == extensionKind && resource.Type == extensionType {
				return &controllerRegistration, nil
			}
		}
	}
	return nil, fmt.Errorf("could not find ControllerRegistration for extension kind %s and type %s", extensionKind, extensionType)
}

func getControllerInstallationForControllerRegistration(controllerInstallations *gardencorev1beta1.ControllerInstallationList, controllerRegistration *gardencorev1beta1.ControllerRegistration) (*gardencorev1beta1.ControllerInstallation, error) {
	for _, controllerInstallation := range controllerInstallations.Items {
		if controllerInstallation.Spec.RegistrationRef.Name == controllerRegistration.Name {
			return &controllerInstallation, nil
		}
	}
	return nil, fmt.Errorf("could not find ControllerInstallation for ControllerRegistration %s", client.ObjectKeyFromObject(controllerRegistration))
}

func (h *Health) healthChecks(
	ctx context.Context,
	thresholdMappings map[gardencorev1beta1.ConditionType]time.Duration,
	healthCheckOutdatedThreshold *metav1.Duration,
	conditions []gardencorev1beta1.Condition,
) []gardencorev1beta1.Condition {
	if h.shoot.HibernationEnabled || h.shoot.GetInfo().Status.IsHibernated {
		return shootHibernatedConditions(h.clock, conditions)
	}

	var apiserverAvailability, controlPlane, observabilityComponents, nodes, systemComponents gardencorev1beta1.Condition
	for _, cond := range conditions {
		switch cond.Type {
		case gardencorev1beta1.ShootAPIServerAvailable:
			apiserverAvailability = cond
		case gardencorev1beta1.ShootControlPlaneHealthy:
			controlPlane = cond
		case gardencorev1beta1.ShootObservabilityComponentsHealthy:
			observabilityComponents = cond
		case gardencorev1beta1.ShootEveryNodeReady:
			nodes = cond
		case gardencorev1beta1.ShootSystemComponentsHealthy:
			systemComponents = cond
		}
	}

	extensionConditionsControlPlaneHealthy, extensionConditionsEveryNodeReady, extensionConditionsSystemComponentsHealthy, err := h.getAllExtensionConditions(ctx)
	if err != nil {
		h.log.Error(err, "Error getting extension conditions")
	}

	checker := NewHealthChecker(h.seedClient.Client(), h.clock, thresholdMappings, healthCheckOutdatedThreshold, gardenlethelper.GetManagedResourceProgressingThreshold(h.gardenletConfiguration), h.shoot.GetInfo().Status.LastOperation, h.shoot.KubernetesVersion, h.shoot.GardenerVersion)

	shootClient, apiServerRunning, err := h.initializeShootClients()
	if err != nil || !apiServerRunning {
		// don't execute health checks if API server has already been deleted or has not been created yet
		message := shootControlPlaneNotRunningMessage(h.shoot.GetInfo().Status.LastOperation)
		if err != nil {
			h.log.Error(err, "Could not initialize Shoot client for health check")
			message = fmt.Sprintf("Could not initialize Shoot client for health check: %+v", err)
		}

		apiserverAvailability = checker.FailedCondition(apiserverAvailability, "APIServerDown", "Could not reach API server during client initialization.")

		newControlPlane, err := h.checkControlPlane(ctx, checker, controlPlane, extensionConditionsControlPlaneHealthy)
		controlPlane = NewConditionOrError(h.clock, controlPlane, newControlPlane, err)

		newObservabilityComponents, err := h.checkObservabilityComponents(ctx, checker, observabilityComponents)
		observabilityComponents = NewConditionOrError(h.clock, observabilityComponents, newObservabilityComponents, err)

		if h.shoot.IsWorkerless {
			return []gardencorev1beta1.Condition{apiserverAvailability, controlPlane, observabilityComponents}
		}

		nodes = v1beta1helper.UpdatedConditionUnknownErrorMessageWithClock(h.clock, nodes, message)
		systemComponents = v1beta1helper.UpdatedConditionUnknownErrorMessageWithClock(h.clock, systemComponents, message)

		return []gardencorev1beta1.Condition{apiserverAvailability, controlPlane, observabilityComponents, nodes, systemComponents}
	}

	h.shootClient = shootClient
	taskFns := []flow.TaskFn{
		func(ctx context.Context) error {
			apiserverAvailability = h.checkAPIServerAvailability(ctx, checker, apiserverAvailability)
			return nil
		}, func(ctx context.Context) error {
			newControlPlane, err := h.checkControlPlane(ctx, checker, controlPlane, extensionConditionsControlPlaneHealthy)
			controlPlane = NewConditionOrError(h.clock, controlPlane, newControlPlane, err)
			return nil
		}, func(ctx context.Context) error {
			newObservabilityComponents, err := h.checkObservabilityComponents(ctx, checker, observabilityComponents)
			observabilityComponents = NewConditionOrError(h.clock, observabilityComponents, newObservabilityComponents, err)
			return nil
		},
	}

	if h.shoot.IsWorkerless {
		_ = flow.Parallel(taskFns...)(ctx)

		return []gardencorev1beta1.Condition{apiserverAvailability, controlPlane, observabilityComponents}
	}

	taskFns = append(taskFns,
		func(ctx context.Context) error {
			newNodes, err := h.checkClusterNodes(ctx, h.shootClient.Client(), checker, nodes, extensionConditionsEveryNodeReady)
			nodes = NewConditionOrError(h.clock, nodes, newNodes, err)
			return nil
		}, func(ctx context.Context) error {
			newSystemComponents, err := h.checkSystemComponents(ctx, checker, systemComponents, extensionConditionsSystemComponentsHealthy)
			systemComponents = NewConditionOrError(h.clock, systemComponents, newSystemComponents, err)
			return nil
		},
	)

	_ = flow.Parallel(taskFns...)(ctx)

	return []gardencorev1beta1.Condition{apiserverAvailability, controlPlane, observabilityComponents, nodes, systemComponents}
}

// checkAPIServerAvailability checks if the API server of a Shoot cluster is reachable and measure the response time.
func (h *Health) checkAPIServerAvailability(ctx context.Context, checker *HealthChecker, condition gardencorev1beta1.Condition) gardencorev1beta1.Condition {
	return health.CheckAPIServerAvailability(ctx, h.clock, h.log, condition, h.shootClient.RESTClient(), func(conditionType, message string) gardencorev1beta1.Condition {
		return checker.FailedCondition(condition, conditionType, message)
	})
}

// checkControlPlane checks whether the core components of the Shoot controlplane (ETCD, KAPI, KCM..) are healthy.
func (h *Health) checkControlPlane(
	ctx context.Context,
	checker *HealthChecker,
	condition gardencorev1beta1.Condition,
	extensionConditions []ExtensionCondition,
) (*gardencorev1beta1.Condition, error) {
	if exitCondition, err := checker.CheckControlPlane(ctx, h.shoot.GetInfo(), h.shoot.SeedNamespace, condition); err != nil || exitCondition != nil {
		return exitCondition, err
	}

	if exitCondition := checker.CheckExtensionCondition(condition, extensionConditions); exitCondition != nil {
		return exitCondition, nil
	}

	c := v1beta1helper.UpdatedConditionWithClock(h.clock, condition, gardencorev1beta1.ConditionTrue, "ControlPlaneRunning", "All control plane components are healthy.")
	return &c, nil
}

// checkObservabilityComponents checks whether the  observability components of the Shoot control plane (Prometheus, Vali, Plutono..) are healthy.
func (h *Health) checkObservabilityComponents(
	ctx context.Context,
	checker *HealthChecker,
	condition gardencorev1beta1.Condition,
) (*gardencorev1beta1.Condition, error) {
	wantsAlertmanager := h.shoot.WantsAlertmanager
	wantsShootMonitoring := gardenlethelper.IsMonitoringEnabled(h.gardenletConfiguration) && h.shoot.Purpose != gardencorev1beta1.ShootPurposeTesting
	if exitCondition, err := checker.CheckMonitoringControlPlane(ctx, h.shoot.GetInfo(), h.shoot.SeedNamespace, wantsShootMonitoring, wantsAlertmanager, condition); err != nil || exitCondition != nil {
		return exitCondition, err
	}

	valiEnabled := gardenlethelper.IsValiEnabled(h.gardenletConfiguration)
	loggingEnabled := gardenlethelper.IsLoggingEnabled(h.gardenletConfiguration)
	eventLoggingEnabled := gardenlethelper.IsEventLoggingEnabled(h.gardenletConfiguration)

	if loggingEnabled {
		if exitCondition, err := checker.CheckLoggingControlPlane(ctx, h.shoot.SeedNamespace, h.shoot.Purpose == gardencorev1beta1.ShootPurposeTesting, eventLoggingEnabled, valiEnabled, condition); err != nil || exitCondition != nil {
			return exitCondition, err
		}
	}

	c := v1beta1helper.UpdatedConditionWithClock(h.clock, condition, gardencorev1beta1.ConditionTrue, "ObservabilityComponentsRunning", "All observability components are healthy.")
	return &c, nil
}

// checkSystemComponents checks whether the system components of a Shoot are running.
func (h *Health) checkSystemComponents(
	ctx context.Context,
	checker *HealthChecker,
	condition gardencorev1beta1.Condition,
	extensionConditions []ExtensionCondition,
) (
	*gardencorev1beta1.Condition,
	error,
) {
	mrList := &resourcesv1alpha1.ManagedResourceList{}
	if err := h.seedClient.Client().List(ctx, mrList, client.InNamespace(h.shoot.SeedNamespace)); err != nil {
		return nil, err
	}

	for _, mr := range mrList.Items {
		if mr.Spec.Class != nil {
			continue
		}

		if exitCondition := checker.CheckManagedResource(condition, &mr); exitCondition != nil {
			return exitCondition, nil
		}
	}

	if exitCondition := checker.CheckExtensionCondition(condition, extensionConditions); exitCondition != nil {
		return exitCondition, nil
	}

	podsList := &corev1.PodList{}
	if err := h.shootClient.Client().List(ctx, podsList, client.InNamespace(metav1.NamespaceSystem), client.MatchingLabels{"type": "tunnel"}); err != nil {
		return nil, err
	}

	if len(podsList.Items) == 0 {
		c := checker.FailedCondition(condition, "NoTunnelDeployed", "no tunnels are currently deployed to perform health-check on")
		return &c, nil
	}

	if established, err := botanist.CheckTunnelConnection(ctx, logr.Discard(), h.shootClient, common.VPNTunnel); err != nil || !established {
		msg := "Tunnel connection has not been established"
		if err != nil {
			msg += fmt.Sprintf(" (%+v)", err)
		}
		c := checker.FailedCondition(condition, "TunnelConnectionBroken", msg)
		return &c, nil
	}

	c := v1beta1helper.UpdatedConditionWithClock(h.clock, condition, gardencorev1beta1.ConditionTrue, "SystemComponentsRunning", "All system components are healthy.")
	return &c, nil
}

// checkClusterNodes checks whether every node registered at the Shoot cluster is in "Ready" state, that
// as many nodes are registered as desired, and that every machine is running.
func (h *Health) checkClusterNodes(
	ctx context.Context,
	shootClient client.Client,
	checker *HealthChecker,
	condition gardencorev1beta1.Condition,
	extensionConditions []ExtensionCondition,
) (*gardencorev1beta1.Condition, error) {
	if exitCondition, err := checker.CheckClusterNodes(ctx, shootClient, h.shoot.GetInfo().Spec.Provider.Workers, condition); err != nil || exitCondition != nil {
		return exitCondition, err
	}
	if exitCondition := checker.CheckExtensionCondition(condition, extensionConditions); exitCondition != nil {
		return exitCondition, nil
	}

	c := v1beta1helper.UpdatedConditionWithClock(h.clock, condition, gardencorev1beta1.ConditionTrue, "EveryNodeReady", "All nodes are ready.")
	return &c, nil
}

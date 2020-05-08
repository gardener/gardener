// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package healthcheck

import (
	"context"
	"fmt"

	"github.com/gardener/gardener/extensions/pkg/controller"
	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/extensions/pkg/util"
	"github.com/gardener/gardener/pkg/api/extensions"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardenv1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"
)

type reconciler struct {
	logger              logr.Logger
	actuator            HealthCheckActuator
	ctx                 context.Context
	client              client.Client
	recorder            record.EventRecorder
	registeredExtension RegisteredExtension
	syncPeriod          metav1.Duration
}

const (
	// HealthCheckUnsuccessful is the reason phrase for the health check condition if one or more of its tests failed.
	HealthCheckUnsuccessful = "HealthCheckUnsuccessful"

	// HealthCheckSuccessful is the reason phrase for the health check condition if all tests are successful.
	HealthCheckSuccessful = "HealthCheckSuccessful"
)

// NewReconciler creates a new performHealthCheck.Reconciler that reconciles
// the registered extension resources (Gardener's `extensions.gardener.cloud` API group).
func NewReconciler(mgr manager.Manager, actuator HealthCheckActuator, registeredExtension RegisteredExtension, syncPeriod metav1.Duration) reconcile.Reconciler {
	return &reconciler{
		logger:              log.Log.WithName(ControllerName),
		actuator:            actuator,
		recorder:            mgr.GetEventRecorderFor(ControllerName),
		registeredExtension: registeredExtension,
		syncPeriod:          syncPeriod,
	}
}

func (r *reconciler) InjectFunc(f inject.Func) error {
	return f(r.actuator)
}

func (r *reconciler) InjectClient(client client.Client) error {
	r.client = client
	return nil
}

func (r *reconciler) InjectStopChannel(stopCh <-chan struct{}) error {
	r.ctx = util.ContextFromStopChannel(stopCh)
	return nil
}

func (r *reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	extension := r.registeredExtension.getExtensionObjFunc()

	if err := r.client.Get(r.ctx, request.NamespacedName, extension); err != nil {
		if errors.IsNotFound(err) {
			return r.resultWithRequeue(), nil
		}
		return r.resultWithRequeue(), err
	}

	acc, err := extensions.Accessor(extension.DeepCopyObject())
	if err != nil {
		return r.resultWithRequeue(), err
	}

	if acc.GetDeletionTimestamp() != nil {
		r.logger.V(6).Info("Do not perform HealthCheck for extension resource. Extension is being deleted.", "name", acc.GetName(), "namespace", acc.GetNamespace())
		return reconcile.Result{}, nil
	}

	if isInMigration(acc) {
		r.logger.Info("Do not perform HealthCheck for extension resource. Extension is being migrated.", "name", acc.GetName(), "namespace", acc.GetNamespace())
		return reconcile.Result{}, nil
	}

	cluster, err := extensionscontroller.GetCluster(r.ctx, r.client, acc.GetNamespace())
	if err != nil {
		return reconcile.Result{}, err
	}
	if controller.IsHibernated(cluster) {
		for _, healthConditionType := range r.registeredExtension.healthConditionTypes {
			if err := r.updateExtensionConditionHibernated(r.ctx, healthConditionType, r.registeredExtension.extension, extension); err != nil {
				return reconcile.Result{}, err
			}
		}
		r.logger.V(6).Info("Do not perform HealthCheck for extension resource. Shoot is hibernated.", "name", acc.GetName(), "namespace", acc.GetNamespace(), "kind", acc.GetObjectKind().GroupVersionKind().Kind)
		return reconcile.Result{}, nil
	}

	r.logger.V(6).Info("Performing health check", "name", acc.GetName(), "namespace", acc.GetNamespace(), "kind", acc.GetObjectKind().GroupVersionKind().Kind)
	return r.performHealthCheck(r.ctx, request, extension)
}

func (r *reconciler) performHealthCheck(ctx context.Context, request reconcile.Request, extension runtime.Object) (reconcile.Result, error) {
	healthCheckResults, err := r.actuator.ExecuteHealthCheckFunctions(ctx, types.NamespacedName{Namespace: request.Namespace, Name: request.Name})
	if err != nil {
		r.logger.Info("Failed to execute healthChecks. Updating each HealthCheckCondition for the extension resource to ConditionCheckError.", "kind", r.registeredExtension.groupVersionKind.Kind, "health condition types", r.registeredExtension.healthConditionTypes, "name", request.Name, "namespace", request.Namespace, "error", err.Error())
		for _, healthConditionType := range r.registeredExtension.healthConditionTypes {
			if err := r.updateExtensionConditionFailedToExecute(ctx, healthConditionType, r.registeredExtension.extension, extension, r.registeredExtension.groupVersionKind.Kind, err); err != nil {
				return r.resultWithRequeue(), err
			}
		}
		return r.resultWithRequeue(), nil
	}

	for _, healthCheckResult := range *healthCheckResults {
		conditionBuilder, err := gardencorev1beta1helper.NewConditionBuilder(gardencorev1beta1.ConditionType(healthCheckResult.HealthConditionType))
		if err != nil {
			return r.resultWithRequeue(), err
		}

		if !healthCheckResult.IsHealthy && healthCheckResult.FailedChecks > 0 {
			r.logger.Info("Updating HealthCheckCondition for extension resource to ConditionCheckError.", "kind", r.registeredExtension.groupVersionKind.Kind, "health condition type", healthCheckResult.HealthConditionType, "name", request.Name, "namespace", request.Namespace)
			if err := r.updateExtensionConditionToConditionCheckError(ctx, conditionBuilder, healthCheckResult.HealthConditionType, r.registeredExtension.extension, extension, r.registeredExtension.groupVersionKind.Kind, healthCheckResult); err != nil {
				return r.resultWithRequeue(), err
			}
			continue
		}

		if !healthCheckResult.IsHealthy {
			r.logger.Info("Health check for extension resource unsuccessful.", "kind", fmt.Sprintf("%s.%s.%s", r.registeredExtension.groupVersionKind.Kind, r.registeredExtension.groupVersionKind.Group, r.registeredExtension.groupVersionKind.Version), "name", request.Name, "namespace", request.Namespace, "failed", healthCheckResult.FailedChecks, "successful", healthCheckResult.SuccessfulChecks, "details", healthCheckResult.GetDetails())
			if err := r.updateExtensionConditionToError(ctx, conditionBuilder, healthCheckResult.HealthConditionType, r.registeredExtension.extension, extension, healthCheckResult); err != nil {
				return r.resultWithRequeue(), err
			}
			continue
		}

		r.logger.V(6).Info("Health check for extension resource successful.", "kind", r.registeredExtension.groupVersionKind.Kind, "health condition type", healthCheckResult.HealthConditionType, "name", request.Name, "namespace", request.Namespace)
		if err := r.updateExtensionConditionToSuccessful(ctx, conditionBuilder, healthCheckResult.HealthConditionType, r.registeredExtension.extension, extension, healthCheckResult); err != nil {
			return r.resultWithRequeue(), err
		}
	}
	return r.resultWithRequeue(), nil
}

func (r *reconciler) updateExtensionConditionFailedToExecute(ctx context.Context, healthConditionType string, extensionResource extensionsv1alpha1.Object, extension runtime.Object, kind string, executionError error) error {
	conditionBuilder, err := gardencorev1beta1helper.NewConditionBuilder(gardencorev1beta1.ConditionType(healthConditionType))
	if err != nil {
		return err
	}

	msg := fmt.Sprintf("failed to execute health checks for '%s': %v", kind, executionError.Error())
	conditionBuilder.
		WithStatus(gardencorev1beta1.ConditionUnknown).
		WithReason(gardencorev1beta1.ConditionCheckError).
		WithMessage(msg)
	return r.updateExtensionCondition(ctx, conditionBuilder, healthConditionType, extension, extensionResource)
}

func (r *reconciler) updateExtensionConditionToConditionCheckError(ctx context.Context, conditionBuilder gardencorev1beta1helper.ConditionBuilder, healthConditionType string, extensionResource extensionsv1alpha1.Object, extension runtime.Object, kind string, healthCheckResult Result) error {
	msg := fmt.Sprintf("failed to execute %d/%d health checks for '%s': %v", healthCheckResult.FailedChecks, healthCheckResult.SuccessfulChecks+healthCheckResult.UnsuccessfulChecks+healthCheckResult.FailedChecks, kind, healthCheckResult.GetDetails())
	conditionBuilder.
		WithStatus(gardencorev1beta1.ConditionUnknown).
		WithReason(gardencorev1beta1.ConditionCheckError).
		WithMessage(msg)
	return r.updateExtensionCondition(ctx, conditionBuilder, healthConditionType, extension, extensionResource)
}

func (r *reconciler) updateExtensionConditionToError(ctx context.Context, conditionBuilder gardencorev1beta1helper.ConditionBuilder, healthConditionType string, extensionResource extensionsv1alpha1.Object, extension runtime.Object, healthCheckResult Result) error {
	msg := fmt.Sprintf("Health check for %d/%d component(s) unsuccessful: %v", healthCheckResult.UnsuccessfulChecks, healthCheckResult.UnsuccessfulChecks+healthCheckResult.SuccessfulChecks, healthCheckResult.GetDetails())
	conditionBuilder.
		WithStatus(gardencorev1beta1.ConditionFalse).
		WithReason(HealthCheckUnsuccessful).
		WithMessage(msg)
	return r.updateExtensionCondition(ctx, conditionBuilder, healthConditionType, extension, extensionResource)
}

func (r *reconciler) updateExtensionConditionToSuccessful(ctx context.Context, conditionBuilder gardencorev1beta1helper.ConditionBuilder, healthConditionType string, extensionResource extensionsv1alpha1.Object, extension runtime.Object, healthCheckResult Result) error {
	msg := fmt.Sprintf("(%d/%d) Health checks successful", healthCheckResult.SuccessfulChecks, healthCheckResult.SuccessfulChecks)
	conditionBuilder.
		WithStatus(gardencorev1beta1.ConditionTrue).
		WithReason(HealthCheckSuccessful).
		WithMessage(msg)
	return r.updateExtensionCondition(ctx, conditionBuilder, healthConditionType, extension, extensionResource)
}

func (r *reconciler) updateExtensionConditionHibernated(ctx context.Context, healthConditionType string, extensionResource extensionsv1alpha1.Object, extension runtime.Object) error {
	conditionBuilder, err := gardencorev1beta1helper.NewConditionBuilder(gardencorev1beta1.ConditionType(healthConditionType))
	if err != nil {
		return err
	}
	msg := "Shoot is hibernated"
	conditionBuilder.
		WithStatus(gardencorev1beta1.ConditionTrue).
		WithReason(HealthCheckSuccessful).
		WithMessage(msg)
	return r.updateExtensionCondition(ctx, conditionBuilder, healthConditionType, extension, extensionResource)
}

func (r *reconciler) updateExtensionCondition(ctx context.Context, conditionBuilder gardencorev1beta1helper.ConditionBuilder, healthConditionType string, extension runtime.Object, extensionResource extensionsv1alpha1.Object) error {
	return extensionscontroller.TryUpdateStatus(ctx, retry.DefaultBackoff, r.client, extension, func() error {
		acc, err := extensions.Accessor(extension)
		if err != nil {
			return fmt.Errorf("error updating health check condition (type: %s, name: %s, ns %s) - failed to create an extensionsv1alpha1.Object from the extension object: %v", healthConditionType, extensionResource.GetName(), extensionResource.GetNamespace(), err)
		}

		if c := gardencorev1beta1helper.GetCondition(acc.GetExtensionStatus().GetConditions(), gardencorev1beta1.ConditionType(healthConditionType)); c != nil {
			conditionBuilder.WithOldCondition(*c)
		}

		updatedCondition, _ := conditionBuilder.WithNowFunc(metav1.Now).Build()

		// always update - the Gardenlet expects a recent health check
		updatedCondition.LastUpdateTime = metav1.Now()

		newConditions := gardencorev1beta1helper.MergeConditions(acc.GetExtensionStatus().GetConditions(), updatedCondition)
		acc.GetExtensionStatus().SetConditions(newConditions)
		return nil
	})
}

func (r *reconciler) resultWithRequeue() reconcile.Result {
	return reconcile.Result{RequeueAfter: r.syncPeriod.Duration}
}

func isInMigration(accessor extensionsv1alpha1.Object) bool {
	annotations := accessor.GetAnnotations()
	if annotations != nil &&
		annotations[gardenv1beta1constants.GardenerOperation] == gardenv1beta1constants.GardenerOperationMigrate {
		return true
	}

	status := accessor.GetExtensionStatus()
	if status == nil {
		return false
	}

	lastOperation := status.GetLastOperation()
	return lastOperation != nil && lastOperation.Type == gardencorev1beta1.LastOperationTypeMigrate
}

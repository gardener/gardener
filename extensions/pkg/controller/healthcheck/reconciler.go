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
	"time"

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
	// ReasonUnsuccessful is the reason phrase for the health check condition if one or more of its tests failed.
	ReasonUnsuccessful = "HealthCheckUnsuccessful"
	// ReasonProgressing is the reason phrase for the health check condition if one or more of its tests are progressing.
	ReasonProgressing = "HealthCheckProgressing"
	// ReasonSuccessful is the reason phrase for the health check condition if all tests are successful.
	ReasonSuccessful = "HealthCheckSuccessful"
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
		return reconcile.Result{}, err
	}

	acc, err := extensions.Accessor(extension.DeepCopyObject())
	if err != nil {
		return reconcile.Result{}, err
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
			conditionBuilder, err := gardencorev1beta1helper.NewConditionBuilder(gardencorev1beta1.ConditionType(healthConditionType))
			if err != nil {
				return reconcile.Result{}, err
			}

			if err := r.updateExtensionConditionHibernated(r.ctx, conditionBuilder, healthConditionType, extension); err != nil {
				return reconcile.Result{}, err
			}
		}

		r.logger.V(6).Info("Do not perform HealthCheck for extension resource. Shoot is hibernated.", "name", acc.GetName(), "namespace", acc.GetNamespace(), "kind", acc.GetObjectKind().GroupVersionKind().Kind)
		return reconcile.Result{}, nil
	}

	r.logger.V(6).Info("Performing health check", "name", acc.GetName(), "namespace", acc.GetNamespace(), "kind", acc.GetObjectKind().GroupVersionKind().Kind)
	return r.performHealthCheck(r.ctx, request, extension)
}

func (r *reconciler) performHealthCheck(ctx context.Context, request reconcile.Request, extension extensionsv1alpha1.Object) (reconcile.Result, error) {
	healthCheckResults, err := r.actuator.ExecuteHealthCheckFunctions(ctx, types.NamespacedName{Namespace: request.Namespace, Name: request.Name})
	if err != nil {
		r.logger.Info("Failed to execute healthChecks. Updating each HealthCheckCondition for the extension resource to ConditionCheckError.", "kind", r.registeredExtension.groupVersionKind.Kind, "health condition types", r.registeredExtension.healthConditionTypes, "name", request.Name, "namespace", request.Namespace, "error", err.Error())
		for _, healthConditionType := range r.registeredExtension.healthConditionTypes {
			conditionBuilder, buildErr := gardencorev1beta1helper.NewConditionBuilder(gardencorev1beta1.ConditionType(healthConditionType))
			if buildErr != nil {
				return reconcile.Result{}, buildErr
			}

			if updateErr := r.updateExtensionConditionFailedToExecute(ctx, conditionBuilder, healthConditionType, extension, r.registeredExtension.groupVersionKind.Kind, err); updateErr != nil {
				return reconcile.Result{}, updateErr
			}
		}
		return r.resultWithRequeue(), nil
	}

	for _, healthCheckResult := range *healthCheckResults {
		conditionBuilder, err := gardencorev1beta1helper.NewConditionBuilder(gardencorev1beta1.ConditionType(healthCheckResult.HealthConditionType))
		if err != nil {
			return reconcile.Result{}, err
		}

		var logger logr.InfoLogger
		if healthCheckResult.Status == gardencorev1beta1.ConditionTrue || healthCheckResult.Status == gardencorev1beta1.ConditionProgressing {
			logger = r.logger.V(6)
		} else {
			logger = r.logger
		}

		if healthCheckResult.Status == gardencorev1beta1.ConditionProgressing || healthCheckResult.Status == gardencorev1beta1.ConditionFalse {
			if healthCheckResult.FailedChecks > 0 {
				r.logger.Info("Updating HealthCheckCondition for extension resource to ConditionCheckError.", "kind", r.registeredExtension.groupVersionKind.Kind, "health condition type", healthCheckResult.HealthConditionType, "name", request.Name, "namespace", request.Namespace)
				if err := r.updateExtensionConditionToConditionCheckError(ctx, conditionBuilder, healthCheckResult.HealthConditionType, extension, r.registeredExtension.groupVersionKind.Kind, healthCheckResult); err != nil {
					return reconcile.Result{}, err
				}
				continue
			}

			logger.Info("Health check for extension resource progressing or unsuccessful.", "kind", fmt.Sprintf("%s.%s.%s", r.registeredExtension.groupVersionKind.Kind, r.registeredExtension.groupVersionKind.Group, r.registeredExtension.groupVersionKind.Version), "name", request.Name, "namespace", request.Namespace, "failed", healthCheckResult.FailedChecks, "progressing", healthCheckResult.ProgressingChecks, "successful", healthCheckResult.SuccessfulChecks, "details", healthCheckResult.GetDetails())
			if err := r.updateExtensionConditionToUnsuccessful(ctx, conditionBuilder, healthCheckResult.HealthConditionType, extension, healthCheckResult); err != nil {
				return reconcile.Result{}, err
			}
			continue
		}

		logger.Info("Health check for extension resource successful.", "kind", r.registeredExtension.groupVersionKind.Kind, "health condition type", healthCheckResult.HealthConditionType, "name", request.Name, "namespace", request.Namespace)
		if err := r.updateExtensionConditionToSuccessful(ctx, conditionBuilder, healthCheckResult.HealthConditionType, extension, healthCheckResult); err != nil {
			return reconcile.Result{}, err
		}
	}

	return r.resultWithRequeue(), nil
}

func (r *reconciler) updateExtensionConditionFailedToExecute(ctx context.Context, conditionBuilder gardencorev1beta1helper.ConditionBuilder, healthConditionType string, extension extensionsv1alpha1.Object, kind string, executionError error) error {
	conditionBuilder.
		WithStatus(gardencorev1beta1.ConditionUnknown).
		WithReason(gardencorev1beta1.ConditionCheckError).
		WithMessage(fmt.Sprintf("failed to execute health checks for '%s': %v", kind, executionError.Error()))
	return r.updateExtensionCondition(ctx, conditionBuilder, healthConditionType, extension)
}

func (r *reconciler) updateExtensionConditionToConditionCheckError(ctx context.Context, conditionBuilder gardencorev1beta1helper.ConditionBuilder, healthConditionType string, extension extensionsv1alpha1.Object, kind string, healthCheckResult Result) error {
	conditionBuilder.
		WithStatus(gardencorev1beta1.ConditionUnknown).
		WithReason(gardencorev1beta1.ConditionCheckError).
		WithMessage(fmt.Sprintf("failed to execute %d/%d health checks for '%s': %v", healthCheckResult.FailedChecks, healthCheckResult.SuccessfulChecks+healthCheckResult.UnsuccessfulChecks+healthCheckResult.FailedChecks, kind, healthCheckResult.GetDetails()))
	return r.updateExtensionCondition(ctx, conditionBuilder, healthConditionType, extension)
}

func (r *reconciler) updateExtensionConditionToUnsuccessful(ctx context.Context, conditionBuilder gardencorev1beta1helper.ConditionBuilder, healthConditionType string, extension extensionsv1alpha1.Object, healthCheckResult Result) error {
	var (
		numberOfChecks = healthCheckResult.UnsuccessfulChecks + healthCheckResult.ProgressingChecks + healthCheckResult.SuccessfulChecks
		detail         = fmt.Sprintf("Health check summary: %d/%d unsuccessful, %d/%d progressing, %d/%d successful. ", healthCheckResult.UnsuccessfulChecks, numberOfChecks, healthCheckResult.ProgressingChecks, numberOfChecks, healthCheckResult.SuccessfulChecks, numberOfChecks)
		status         = gardencorev1beta1.ConditionFalse
		reason         = ReasonUnsuccessful
	)

	if healthCheckResult.ProgressingChecks > 0 && healthCheckResult.ProgressingThreshold != nil {
		if oldCondition := gardencorev1beta1helper.GetCondition(extension.GetExtensionStatus().GetConditions(), gardencorev1beta1.ConditionType(healthConditionType)); oldCondition == nil {
			status = gardencorev1beta1.ConditionProgressing
			reason = ReasonProgressing
		} else if oldCondition.Status != gardencorev1beta1.ConditionFalse {
			delta := time.Now().UTC().Sub(oldCondition.LastTransitionTime.Time.UTC())
			if oldCondition.Status == gardencorev1beta1.ConditionTrue || delta <= *healthCheckResult.ProgressingThreshold {
				status = gardencorev1beta1.ConditionProgressing
				reason = ReasonProgressing
			}
		}
	}

	conditionBuilder.
		WithStatus(status).
		WithReason(reason).
		WithCodes(healthCheckResult.Codes...).
		WithMessage(detail)
	return r.updateExtensionCondition(ctx, conditionBuilder, healthConditionType, extension)
}

func (r *reconciler) updateExtensionConditionToSuccessful(ctx context.Context, conditionBuilder gardencorev1beta1helper.ConditionBuilder, healthConditionType string, extension extensionsv1alpha1.Object, healthCheckResult Result) error {
	conditionBuilder.
		WithStatus(gardencorev1beta1.ConditionTrue).
		WithReason(ReasonUnsuccessful).
		WithMessage(fmt.Sprintf("(%d/%d) Health checks successful", healthCheckResult.SuccessfulChecks, healthCheckResult.SuccessfulChecks))
	return r.updateExtensionCondition(ctx, conditionBuilder, healthConditionType, extension)
}

func (r *reconciler) updateExtensionConditionHibernated(ctx context.Context, conditionBuilder gardencorev1beta1helper.ConditionBuilder, healthConditionType string, extension extensionsv1alpha1.Object) error {
	conditionBuilder.
		WithStatus(gardencorev1beta1.ConditionTrue).
		WithReason(ReasonSuccessful).
		WithMessage("Shoot is hibernated")
	return r.updateExtensionCondition(ctx, conditionBuilder, healthConditionType, extension)
}

func (r *reconciler) updateExtensionCondition(ctx context.Context, conditionBuilder gardencorev1beta1helper.ConditionBuilder, healthConditionType string, extension extensionsv1alpha1.Object) error {
	return extensionscontroller.TryUpdateStatus(ctx, retry.DefaultBackoff, r.client, extension, func() error {
		if c := gardencorev1beta1helper.GetCondition(extension.GetExtensionStatus().GetConditions(), gardencorev1beta1.ConditionType(healthConditionType)); c != nil {
			conditionBuilder.WithOldCondition(*c)
		}

		updatedCondition, _ := conditionBuilder.WithNowFunc(metav1.Now).Build()
		// always update - the Gardenlet expects a recent health check
		updatedCondition.LastUpdateTime = metav1.Now()

		extension.GetExtensionStatus().SetConditions(gardencorev1beta1helper.MergeConditions(extension.GetExtensionStatus().GetConditions(), updatedCondition))
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

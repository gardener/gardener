// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package healthcheck

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	"github.com/gardener/gardener/pkg/api/extensions"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

type reconciler struct {
	client client.Client

	actuator            HealthCheckActuator
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
		actuator:            actuator,
		client:              mgr.GetClient(),
		registeredExtension: registeredExtension,
		syncPeriod:          syncPeriod,
	}
}

func (r *reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	// overall timeout for all calls in this reconciler (including status updates);
	// this gives status updates a bit of headroom if the actual health checks run into timeouts,
	// so that we will still update the condition to the failed status
	var cancel context.CancelFunc
	ctx, cancel = context.WithTimeout(ctx, 2*r.syncPeriod.Duration)
	defer cancel()

	extension := r.registeredExtension.getExtensionObjFunc()
	if err := r.client.Get(ctx, request.NamespacedName, extension); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object was not found, requeuing")
			return r.resultWithRequeue(), nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	acc, err := extensions.Accessor(extension.DeepCopyObject())
	if err != nil {
		return reconcile.Result{}, err
	}

	if acc.GetDeletionTimestamp() != nil {
		log.V(1).Info("Do not perform HealthCheck for extension resource, extension is being deleted")
		return reconcile.Result{}, nil
	}

	if isInMigration(acc) {
		log.Info("Do not perform HealthCheck for extension resource, extension is being migrated")
		return reconcile.Result{}, nil
	}

	cluster, err := extensionscontroller.GetCluster(ctx, r.client, acc.GetNamespace())
	if err != nil {
		return reconcile.Result{}, err
	}

	// cleanup conditions from extension status
	if len(r.registeredExtension.conditionTypesToRemove) > 0 {
		var newConditions []gardencorev1beta1.Condition
		for _, condition := range extension.GetExtensionStatus().GetConditions() {
			if !r.registeredExtension.conditionTypesToRemove.Has(condition.Type) {
				newConditions = append(newConditions, condition)
			}
		}
		extension.GetExtensionStatus().SetConditions(newConditions)
	}

	if extensionscontroller.IsHibernationEnabled(cluster) {
		var conditions []condition
		for _, healthConditionType := range r.registeredExtension.healthConditionTypes {
			conditionBuilder, err := v1beta1helper.NewConditionBuilder(gardencorev1beta1.ConditionType(healthConditionType))
			if err != nil {
				return reconcile.Result{}, err
			}

			conditions = append(conditions, extensionConditionHibernated(conditionBuilder, healthConditionType))
		}
		if err := r.updateExtensionConditions(ctx, extension, conditions...); err != nil {
			return reconcile.Result{}, err
		}

		log.V(1).Info("Do not perform HealthCheck for extension resource, Shoot is hibernated", "groupVersionKind", r.registeredExtension.groupVersionKind)
		return reconcile.Result{}, nil
	}

	log.V(1).Info("Performing healthcheck", "groupVersionKind", r.registeredExtension.groupVersionKind)
	return r.performHealthCheck(ctx, log, request, extension)
}

func (r *reconciler) performHealthCheck(ctx context.Context, log logr.Logger, request reconcile.Request, extension extensionsv1alpha1.Object) (reconcile.Result, error) {
	// use a dedicated context for the actual health checks so that we can still update the conditions in case of timeouts
	healthCheckCtx, cancel := context.WithTimeout(ctx, r.syncPeriod.Duration)
	defer cancel()

	healthCheckResults, err := r.actuator.ExecuteHealthCheckFunctions(healthCheckCtx, log, types.NamespacedName{Namespace: request.Namespace, Name: request.Name})
	if err != nil {
		var conditions []condition
		log.Error(err, "Failed to execute healthChecks, updating each HealthCheckCondition for the extension resource to ConditionCheckError", "kind", r.registeredExtension.groupVersionKind.Kind, "conditionTypes", r.registeredExtension.healthConditionTypes)
		for _, healthConditionType := range r.registeredExtension.healthConditionTypes {
			conditionBuilder, buildErr := v1beta1helper.NewConditionBuilder(gardencorev1beta1.ConditionType(healthConditionType))
			if buildErr != nil {
				return reconcile.Result{}, buildErr
			}

			conditions = append(conditions, extensionConditionFailedToExecute(conditionBuilder, healthConditionType, err))
		}
		if updateErr := r.updateExtensionConditions(ctx, extension, conditions...); updateErr != nil {
			return reconcile.Result{}, updateErr
		}
		return r.resultWithRequeue(), nil
	}

	conditions := make([]condition, 0, len(*healthCheckResults))
	for _, healthCheckResult := range *healthCheckResults {
		conditionBuilder, err := v1beta1helper.NewConditionBuilder(gardencorev1beta1.ConditionType(healthCheckResult.HealthConditionType))
		if err != nil {
			return reconcile.Result{}, err
		}

		var logger logr.Logger
		if healthCheckResult.Status == gardencorev1beta1.ConditionTrue || healthCheckResult.Status == gardencorev1beta1.ConditionProgressing {
			logger = log.V(1)
		} else {
			logger = log
		}

		if healthCheckResult.Status == gardencorev1beta1.ConditionTrue {
			logger.Info("Health check for extension resource successful", "kind", r.registeredExtension.groupVersionKind.Kind, "conditionType", healthCheckResult.HealthConditionType)
			conditions = append(conditions, extensionConditionSuccessful(conditionBuilder, healthCheckResult.HealthConditionType))
			continue
		}

		if healthCheckResult.FailedChecks > 0 {
			logger.Info("Updating HealthCheckCondition for extension resource to ConditionCheckError", "kind", r.registeredExtension.groupVersionKind.Kind, "conditionType", healthCheckResult.HealthConditionType)
			conditions = append(conditions, extensionConditionCheckError(conditionBuilder, healthCheckResult.HealthConditionType, healthCheckResult))
			continue
		}

		logger.Info("Health check for extension resource progressing or unsuccessful", "kind", fmt.Sprintf("%s.%s.%s", r.registeredExtension.groupVersionKind.Kind, r.registeredExtension.groupVersionKind.Group, r.registeredExtension.groupVersionKind.Version), "failed", healthCheckResult.FailedChecks, "progressing", healthCheckResult.ProgressingChecks, "successful", healthCheckResult.SuccessfulChecks, "details", healthCheckResult.GetDetails())
		conditions = append(conditions, extensionConditionUnsuccessful(conditionBuilder, healthCheckResult.HealthConditionType, extension, healthCheckResult))
	}

	if err := r.updateExtensionConditions(ctx, extension, conditions...); err != nil {
		return reconcile.Result{}, err
	}

	return r.resultWithRequeue(), nil
}

func extensionConditionFailedToExecute(conditionBuilder v1beta1helper.ConditionBuilder, healthConditionType string, executionError error) condition {
	conditionBuilder.
		WithStatus(gardencorev1beta1.ConditionUnknown).
		WithReason(gardencorev1beta1.ConditionCheckError).
		WithMessage(fmt.Sprintf("unable to execute any health check: %v", executionError.Error()))
	return condition{
		builder:             conditionBuilder,
		healthConditionType: healthConditionType,
	}
}

func extensionConditionCheckError(conditionBuilder v1beta1helper.ConditionBuilder, healthConditionType string, healthCheckResult Result) condition {
	conditionBuilder.
		WithStatus(gardencorev1beta1.ConditionUnknown).
		WithReason(gardencorev1beta1.ConditionCheckError).
		WithMessage(fmt.Sprintf("failed to execute %d health %s: %v", healthCheckResult.FailedChecks, getSingularOrPlural(healthCheckResult.FailedChecks), healthCheckResult.GetDetails()))
	return condition{
		builder:             conditionBuilder,
		healthConditionType: healthConditionType,
	}
}

func extensionConditionUnsuccessful(conditionBuilder v1beta1helper.ConditionBuilder, healthConditionType string, extension extensionsv1alpha1.Object, healthCheckResult Result) condition {
	var (
		detail = getUnsuccessfulDetailMessage(healthCheckResult.UnsuccessfulChecks, healthCheckResult.ProgressingChecks, healthCheckResult.GetDetails())
		status = gardencorev1beta1.ConditionFalse
		reason = ReasonUnsuccessful
	)

	if healthCheckResult.ProgressingChecks > 0 && healthCheckResult.ProgressingThreshold != nil {
		if oldCondition := v1beta1helper.GetCondition(extension.GetExtensionStatus().GetConditions(), gardencorev1beta1.ConditionType(healthConditionType)); oldCondition == nil {
			status = gardencorev1beta1.ConditionProgressing
			reason = ReasonProgressing
		} else if oldCondition.Status != gardencorev1beta1.ConditionFalse {
			delta := time.Now().UTC().Sub(oldCondition.LastTransitionTime.UTC())
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
	return condition{
		builder:             conditionBuilder,
		healthConditionType: healthConditionType,
	}
}

func extensionConditionSuccessful(conditionBuilder v1beta1helper.ConditionBuilder, healthConditionType string) condition {
	conditionBuilder.
		WithStatus(gardencorev1beta1.ConditionTrue).
		WithReason(ReasonSuccessful).
		WithMessage("All health checks successful")
	return condition{
		builder:             conditionBuilder,
		healthConditionType: healthConditionType,
	}
}

func extensionConditionHibernated(conditionBuilder v1beta1helper.ConditionBuilder, healthConditionType string) condition {
	conditionBuilder.
		WithStatus(gardencorev1beta1.ConditionTrue).
		WithReason(ReasonSuccessful).
		WithMessage("Shoot is hibernated")
	return condition{
		builder:             conditionBuilder,
		healthConditionType: healthConditionType,
	}
}

type condition struct {
	builder             v1beta1helper.ConditionBuilder
	healthConditionType string
}

func (r *reconciler) updateExtensionConditions(ctx context.Context, extension extensionsv1alpha1.Object, conditions ...condition) error {
	for _, cond := range conditions {
		if c := v1beta1helper.GetCondition(extension.GetExtensionStatus().GetConditions(), gardencorev1beta1.ConditionType(cond.healthConditionType)); c != nil {
			cond.builder.WithOldCondition(*c)
		}
		updatedCondition, _ := cond.builder.WithClock(clock.RealClock{}).Build()
		extension.GetExtensionStatus().SetConditions(v1beta1helper.MergeConditions(extension.GetExtensionStatus().GetConditions(), updatedCondition))
	}
	return r.client.Status().Update(ctx, extension)
}

func (r *reconciler) resultWithRequeue() reconcile.Result {
	return reconcile.Result{RequeueAfter: r.syncPeriod.Duration}
}

func isInMigration(accessor extensionsv1alpha1.Object) bool {
	annotations := accessor.GetAnnotations()
	if annotations != nil &&
		annotations[v1beta1constants.GardenerOperation] == v1beta1constants.GardenerOperationMigrate {
		return true
	}

	status := accessor.GetExtensionStatus()
	if status == nil {
		return false
	}

	lastOperation := status.GetLastOperation()
	return lastOperation != nil && lastOperation.Type == gardencorev1beta1.LastOperationTypeMigrate
}

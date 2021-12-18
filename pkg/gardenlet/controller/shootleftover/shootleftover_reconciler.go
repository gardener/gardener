// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package shootleftover

import (
	"context"
	"fmt"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	v1alpha1helper "github.com/gardener/gardener/pkg/apis/core/v1alpha1/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	"github.com/gardener/gardener/pkg/logger"
	utilerrors "github.com/gardener/gardener/pkg/utils/errors"
	"github.com/gardener/gardener/pkg/utils/flow"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// reconciler implements the reconcile.Reconciler interface for ShootLeftover reconciliation.
type reconciler struct {
	gardenClient kubernetes.Interface
	actuator     Actuator
	cfg          *config.ShootLeftoverControllerConfiguration
	recorder     record.EventRecorder
	logger       logrus.FieldLogger
}

// newReconciler creates a new ShootLeftover reconciler with the given parameters.
func newReconciler(gardenClient kubernetes.Interface, actuator Actuator, cfg *config.ShootLeftoverControllerConfiguration, recorder record.EventRecorder, logger logrus.FieldLogger) reconcile.Reconciler {
	return &reconciler{
		gardenClient: gardenClient,
		actuator:     actuator,
		cfg:          cfg,
		recorder:     recorder,
		logger:       logger,
	}
}

func (r *reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	slo := &gardencorev1alpha1.ShootLeftover{}
	if err := r.gardenClient.Client().Get(ctx, request.NamespacedName, slo); err != nil {
		if apierrors.IsNotFound(err) {
			r.logger.Debugf("Skipping ShootLeftover %s because it has been deleted", request.NamespacedName)
			return reconcile.Result{}, nil
		}
		r.logger.Errorf("Could not get ShootLeftover %s from store: %+v", request.NamespacedName, err)
		return reconcile.Result{}, err
	}

	if slo.DeletionTimestamp != nil {
		return r.delete(ctx, slo)
	}
	return r.reconcile(ctx, slo)
}

func (r *reconciler) reconcile(ctx context.Context, slo *gardencorev1alpha1.ShootLeftover) (result reconcile.Result, err error) {
	// Ensure gardener finalizer
	if !controllerutil.ContainsFinalizer(slo, gardencorev1beta1.GardenerName) {
		r.getLogger(slo).Debug("Adding finalizer")
		if err := controllerutils.StrategicMergePatchAddFinalizers(ctx, r.gardenClient.Client(), slo, gardencorev1beta1.GardenerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("could not ensure gardener finalizer: %w", err)
		}
	}

	// Compute operation type
	operationType := gardencorev1beta1helper.ComputeOperationType(slo.ObjectMeta, slo.Status.LastOperation)

	// Update status to Processing
	if updateErr := r.patchStatusOperationProcessing(ctx, slo, operationType); updateErr != nil {
		return reconcile.Result{}, fmt.Errorf("could not update status: %w", updateErr)
	}

	// Reconcile creation or update
	r.normalInfoEventf(slo, gardencorev1beta1.EventReconciling, "Reconciling Shoot leftover resources")
	var resourcesExist bool
	if resourcesExist, err = r.actuator.Reconcile(ctx, slo); err != nil {
		r.warningErrorEventf(slo, gardencorev1beta1.EventReconcileError, "Could not reconcile Shoot leftover resources: %v", err)

		// Update status to Error, suppressing update errors in favor of err
		updateErr := r.patchStatusOperationError(ctx, slo, operationType, resourcesExist, gardencorev1beta1helper.FormatLastErrDescription(err), gardencorev1beta1helper.LastErrors(getError(err))...)
		return reconcile.Result{}, utilerrors.WithSuppressed(
			fmt.Errorf("could not reconcile Shoot leftover resources: %w", err),
			fmt.Errorf("could not update status: %w", updateErr),
		)
	}
	r.normalInfoEventf(slo, gardencorev1alpha1.EventReconciled, "Shoot leftover resources reconciled")

	// Update status to Succeeded
	if updateErr := r.patchStatusOperationSucceeded(ctx, slo, operationType, resourcesExist); updateErr != nil {
		return reconcile.Result{}, fmt.Errorf("could not update status: %w", updateErr)
	}

	// Return success result
	return reconcile.Result{}, nil
}

func (r *reconciler) delete(ctx context.Context, slo *gardencorev1alpha1.ShootLeftover) (result reconcile.Result, err error) {
	// Check gardener finalizer
	if !controllerutil.ContainsFinalizer(slo, gardencorev1beta1.GardenerName) {
		r.getLogger(slo).Debug("Skipping as it does not have a finalizer")
		return reconcile.Result{}, nil
	}

	// Compute operation type
	operationType := gardencorev1beta1helper.ComputeOperationType(slo.ObjectMeta, slo.Status.LastOperation)

	// Update status to Processing
	if updateErr := r.patchStatusOperationProcessing(ctx, slo, operationType); updateErr != nil {
		return reconcile.Result{}, fmt.Errorf("could not update status: %w", updateErr)
	}

	// Reconcile deletion
	r.normalInfoEventf(slo, gardencorev1beta1.EventDeleting, "Deleting Shoot leftover resources")
	var resourcesExist bool
	if resourcesExist, err = r.actuator.Delete(ctx, slo); err != nil {
		r.warningErrorEventf(slo, gardencorev1beta1.EventDeleteError, "Could not delete Shoot leftover resources: %v", err)

		// Update status to Error, suppressing update errors in favor of err
		updateErr := r.patchStatusOperationError(ctx, slo, operationType, resourcesExist, gardencorev1beta1helper.FormatLastErrDescription(err), gardencorev1beta1helper.LastErrors(getError(err))...)
		return reconcile.Result{}, utilerrors.WithSuppressed(
			fmt.Errorf("could not delete Shoot leftover resources: %w", err),
			fmt.Errorf("could not update status: %w", updateErr),
		)
	}
	r.normalInfoEventf(slo, gardencorev1alpha1.EventDeleted, "Shoot leftover resources deleted")

	// Update status to Succeeded
	if updateErr := r.patchStatusOperationSucceeded(ctx, slo, operationType, resourcesExist); updateErr != nil {
		return reconcile.Result{}, fmt.Errorf("could not update status: %w", updateErr)
	}

	// Remove gardener finalizer
	r.getLogger(slo).Debug("Removing finalizer")
	if err := controllerutils.PatchRemoveFinalizers(ctx, r.gardenClient.Client(), slo, gardencorev1beta1.GardenerName); err != nil {
		return reconcile.Result{}, fmt.Errorf("could not remove gardener finalizer: %w", err)
	}

	// Return success result
	return reconcile.Result{}, nil
}

func (r *reconciler) patchStatusOperationProcessing(ctx context.Context, slo *gardencorev1alpha1.ShootLeftover, operationType gardencorev1beta1.LastOperationType) error {
	var description, reason, message string
	switch operationType {
	case gardencorev1beta1.LastOperationTypeCreate, gardencorev1beta1.LastOperationTypeReconcile:
		description = "Reconciliation of Shoot leftover resources initialized."
		reason = gardencorev1alpha1.EventReconciling
		message = "Checking leftover resources"
	case gardencorev1beta1.LastOperationTypeDelete:
		description = "Deletion of Shoot leftover resources initialized."
		reason = gardencorev1alpha1.EventDeleting
		message = "Deleting leftover resources"
	}

	patch := client.StrategicMergeFrom(slo.DeepCopy())

	condition := v1alpha1helper.GetOrInitCondition(slo.Status.Conditions, gardencorev1alpha1.ShootLeftoverResourcesExist)
	condition = v1alpha1helper.UpdatedCondition(condition, gardencorev1alpha1.ConditionProgressing, reason, message)
	slo.Status.Conditions = v1alpha1helper.MergeConditions(slo.Status.Conditions, condition)

	slo.Status.LastOperation = &gardencorev1beta1.LastOperation{
		Type:           operationType,
		State:          gardencorev1beta1.LastOperationStateProcessing,
		Progress:       0,
		Description:    description,
		LastUpdateTime: metav1.Now(),
	}
	slo.Status.ObservedGeneration = slo.Generation

	return r.gardenClient.Client().Status().Patch(ctx, slo, patch)
}

func (r *reconciler) patchStatusOperationSucceeded(ctx context.Context, slo *gardencorev1alpha1.ShootLeftover, operationType gardencorev1beta1.LastOperationType, resourcesExist bool) error {
	var (
		description, reason, message string
		cs                           gardencorev1alpha1.ConditionStatus
	)
	switch operationType {
	case gardencorev1beta1.LastOperationTypeCreate, gardencorev1beta1.LastOperationTypeReconcile:
		description = "Reconciliation of Shoot leftover resources succeeded."
		reason = gardencorev1alpha1.EventReconciled
	case gardencorev1beta1.LastOperationTypeDelete:
		description = "Deletion of Shoot leftover resources succeeded."
		reason = gardencorev1alpha1.EventDeleted
	}
	if resourcesExist {
		cs = gardencorev1alpha1.ConditionTrue
		message = "Some leftover resources still exist"
	} else {
		cs = gardencorev1alpha1.ConditionFalse
		message = "No leftover resources exist"
	}

	patch := client.StrategicMergeFrom(slo.DeepCopy())

	condition := v1alpha1helper.GetOrInitCondition(slo.Status.Conditions, gardencorev1alpha1.ShootLeftoverResourcesExist)
	condition = v1alpha1helper.UpdatedCondition(condition, cs, reason, message)
	slo.Status.Conditions = v1alpha1helper.MergeConditions(slo.Status.Conditions, condition)

	slo.Status.LastOperation = &gardencorev1beta1.LastOperation{
		Type:           operationType,
		State:          gardencorev1beta1.LastOperationStateSucceeded,
		Progress:       100,
		Description:    description,
		LastUpdateTime: metav1.Now(),
	}
	slo.Status.LastErrors = nil

	return r.gardenClient.Client().Status().Patch(ctx, slo, patch)
}

func (r *reconciler) patchStatusOperationError(ctx context.Context, slo *gardencorev1alpha1.ShootLeftover, operationType gardencorev1beta1.LastOperationType, resourcesExist bool, description string, lastErrors ...gardencorev1beta1.LastError) error {
	var (
		reason string
		cs     gardencorev1alpha1.ConditionStatus
	)
	switch operationType {
	case gardencorev1beta1.LastOperationTypeCreate, gardencorev1beta1.LastOperationTypeReconcile:
		reason = gardencorev1alpha1.EventReconcileError
	case gardencorev1beta1.LastOperationTypeDelete:
		reason = gardencorev1alpha1.EventDeleteError
	}
	if resourcesExist {
		cs = gardencorev1alpha1.ConditionTrue
	} else {
		cs = gardencorev1alpha1.ConditionUnknown
	}

	patch := client.StrategicMergeFrom(slo.DeepCopy())

	condition := v1alpha1helper.GetOrInitCondition(slo.Status.Conditions, gardencorev1alpha1.ShootLeftoverResourcesExist)
	condition = v1alpha1helper.UpdatedCondition(condition, cs, reason, description)
	slo.Status.Conditions = v1alpha1helper.MergeConditions(slo.Status.Conditions, condition)

	slo.Status.LastOperation = &gardencorev1beta1.LastOperation{
		Type:           operationType,
		State:          gardencorev1beta1.LastOperationStateError,
		Description:    description + " Operation will be retried.",
		LastUpdateTime: metav1.Now(),
	}
	slo.Status.LastErrors = lastErrors

	return r.gardenClient.Client().Status().Patch(ctx, slo, patch)
}

func (r *reconciler) normalInfoEventf(slo *gardencorev1alpha1.ShootLeftover, reason string, fmt string, args ...interface{}) {
	r.recorder.Eventf(slo, corev1.EventTypeNormal, reason, fmt, args...)
	r.getLogger(slo).Infof(fmt, args...)
}

func (r *reconciler) warningErrorEventf(slo *gardencorev1alpha1.ShootLeftover, reason string, fmt string, args ...interface{}) {
	r.recorder.Eventf(slo, corev1.EventTypeWarning, reason, fmt, args...)
	r.getLogger(slo).Errorf(fmt, args...)
}

func (r *reconciler) getLogger(slo *gardencorev1alpha1.ShootLeftover) logrus.FieldLogger {
	return logger.NewFieldLogger(r.logger, "shootLeftover", kutil.ObjectName(slo))
}

func getError(err error) error {
	if errors := flow.Errors(err); errors != nil {
		return errors
	}
	return err
}

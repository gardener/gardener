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

package managedseed

import (
	"context"
	"fmt"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/logger"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// reconciler implements the reconcile.Reconciler interface for ManagedSeed reconciliation.
type reconciler struct {
	gardenClient kubernetes.Interface
	actuator     Actuator
	recorder     record.EventRecorder
	logger       *logrus.Logger
}

// newReconciler creates a new ManagedSeed reconciler with the given clients, actuator, recorder, and logger.
func newReconciler(gardenClient kubernetes.Interface, actuator Actuator, recorder record.EventRecorder, logger *logrus.Logger) reconcile.Reconciler {
	return &reconciler{
		gardenClient: gardenClient,
		actuator:     actuator,
		recorder:     recorder,
		logger:       logger,
	}
}

func (r *reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	managedSeed := &seedmanagementv1alpha1.ManagedSeed{}
	if err := r.gardenClient.Client().Get(ctx, request.NamespacedName, managedSeed); err != nil {
		if apierrors.IsNotFound(err) {
			r.logger.Debugf("Skipping ManagedSeed %s because it has been deleted", request.NamespacedName)
			return reconcile.Result{}, nil
		}
		r.logger.Errorf("Could not get ManagedSeed %s from store: %+v", request.NamespacedName, err)
		return reconcile.Result{}, err
	}

	managedSeedLogger := logger.NewFieldLogger(r.logger, "managedSeed", kutil.ObjectName(managedSeed))

	if managedSeed.DeletionTimestamp != nil {
		return r.delete(ctx, managedSeed, managedSeedLogger)
	}
	return r.reconcile(ctx, managedSeed, managedSeedLogger)
}

func (r *reconciler) reconcile(ctx context.Context, managedSeed *seedmanagementv1alpha1.ManagedSeed, logger *logrus.Entry) (result reconcile.Result, err error) {

	// Ensure gardener finalizer
	if err := controllerutils.PatchAddFinalizers(ctx, r.gardenClient.Client(), managedSeed, gardencorev1beta1.GardenerName); err != nil {
		return reconcile.Result{}, fmt.Errorf("could not ensure gardener finalizer: %w", err)
	}

	conditionShootExists := gardencorev1beta1helper.GetOrInitCondition(managedSeed.Status.Conditions, seedmanagementv1alpha1.ManagedSeedShootExists)
	conditionShootReconciled := gardencorev1beta1helper.GetOrInitCondition(managedSeed.Status.Conditions, seedmanagementv1alpha1.ManagedSeedShootReconciled)
	conditionSeedRegistered := gardencorev1beta1helper.GetOrInitCondition(managedSeed.Status.Conditions, seedmanagementv1alpha1.ManagedSeedSeedRegistered)

	defer func() {
		// Update status, on failure return the update error unless there is another error
		if updateErr := updateStatus(ctx, r.gardenClient.Client(), managedSeed, conditionShootExists, conditionShootReconciled, conditionSeedRegistered); updateErr != nil && err == nil {
			err = fmt.Errorf("could not update status: %w", updateErr)
		}
	}()

	// Ensure the shoot exists and update the ShootExists condition
	shoot, err := r.ensureShootExists(ctx, managedSeed, &conditionShootExists, logger)
	if err != nil {
		return reconcile.Result{}, err
	}

	// Check if the shoot is reconciled and update the ShootReconciled condition
	// If the shoot is not reconciled yet, requeue for another check in 10s
	if shoot.Generation != shoot.Status.ObservedGeneration || shoot.Status.LastOperation == nil || shoot.Status.LastOperation.State != gardencorev1beta1.LastOperationStateSucceeded {
		message := fmt.Sprintf("Shoot %s is still reconciling", kutil.ObjectName(shoot))
		updateCondition(&conditionShootReconciled, gardencorev1beta1.ConditionFalse, "ShootStillReconciling", message)
		r.recordAndLog(managedSeed, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, logger, logrus.InfoLevel, message)
		return reconcile.Result{RequeueAfter: 10 * time.Second}, nil
	}
	message := fmt.Sprintf("Shoot %s is reconciled", kutil.ObjectName(shoot))
	updateCondition(&conditionShootReconciled, gardencorev1beta1.ConditionTrue, "ShootReconciled", message)
	r.recordAndLog(managedSeed, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, logger, logrus.InfoLevel, message)

	// Reconcile creation or update and update the SeedRegistered condition
	if err := r.actuator.Reconcile(ctx, managedSeed, shoot); err != nil {
		message := fmt.Sprintf("Could not register shoot %s as seed: %+v", kutil.ObjectName(shoot), err)
		updateCondition(&conditionSeedRegistered, gardencorev1beta1.ConditionFalse, "SeedRegistrationFailed", message)
		r.recordAndLog(managedSeed, corev1.EventTypeWarning, gardencorev1beta1.EventReconcileError, logger, logrus.ErrorLevel, message)
		return reconcile.Result{}, fmt.Errorf("could not register shoot %s as seed: %w", kutil.ObjectName(shoot), err)
	}
	message = fmt.Sprintf("Shoot %s registered as seed", kutil.ObjectName(shoot))
	updateCondition(&conditionSeedRegistered, gardencorev1beta1.ConditionTrue, "SeedRegistered", message)
	r.recordAndLog(managedSeed, corev1.EventTypeNormal, gardencorev1beta1.EventReconciled, logger, logrus.InfoLevel, message)

	// Return success result
	return reconcile.Result{}, nil
}

func (r *reconciler) delete(ctx context.Context, managedSeed *seedmanagementv1alpha1.ManagedSeed, logger *logrus.Entry) (result reconcile.Result, err error) {
	// Check gardener finalizer
	if !controllerutil.ContainsFinalizer(managedSeed, gardencorev1beta1.GardenerName) {
		logger.Debug("Skipping ManagedSeed as it does not have a finalizer")
		return reconcile.Result{}, nil
	}

	conditionShootExists := gardencorev1beta1helper.GetOrInitCondition(managedSeed.Status.Conditions, seedmanagementv1alpha1.ManagedSeedShootExists)
	conditionSeedRegistered := gardencorev1beta1helper.GetOrInitCondition(managedSeed.Status.Conditions, seedmanagementv1alpha1.ManagedSeedSeedRegistered)

	defer func() {
		// Update status, on failure return the update error unless there is another error
		if updateErr := updateStatus(ctx, r.gardenClient.Client(), managedSeed, conditionShootExists, conditionSeedRegistered); updateErr != nil && err == nil {
			err = fmt.Errorf("could not update status: %w", updateErr)
		}
	}()

	// Ensure the shoot exists and update the ShootExists condition
	shoot, err := r.ensureShootExists(ctx, managedSeed, &conditionShootExists, logger)
	if err != nil {
		return reconcile.Result{}, err
	}

	// Reconcile deletion and update the SeedRegistered condition
	if err := r.actuator.Delete(ctx, managedSeed, shoot); err != nil {
		message := fmt.Sprintf("Could not unregister shoot %s seed: %+v", kutil.ObjectName(shoot), err)
		updateCondition(&conditionSeedRegistered, gardencorev1beta1.ConditionUnknown, "SeedUnregistrationFailed", message)
		r.recordAndLog(managedSeed, corev1.EventTypeWarning, gardencorev1beta1.EventReconcileError, logger, logrus.ErrorLevel, message)
		return reconcile.Result{}, fmt.Errorf("could not unregister shoot %s as seed: %w", kutil.ObjectName(shoot), err)
	}
	message := fmt.Sprintf("Shoot %s unregistered as seed", kutil.ObjectName(shoot))
	updateCondition(&conditionSeedRegistered, gardencorev1beta1.ConditionFalse, "SeedUnregistered", message)
	r.recordAndLog(managedSeed, corev1.EventTypeNormal, gardencorev1beta1.EventReconciled, logger, logrus.InfoLevel, message)

	// Remove gardener finalizer
	if err := controllerutils.PatchRemoveFinalizers(ctx, r.gardenClient.Client(), managedSeed, gardencorev1beta1.GardenerName); err != nil {
		return reconcile.Result{}, fmt.Errorf("could not remove gardener finalizer: %w", err)
	}

	// Return success result
	return reconcile.Result{}, nil
}

func (r *reconciler) ensureShootExists(ctx context.Context, managedSeed *seedmanagementv1alpha1.ManagedSeed, condition *gardencorev1beta1.Condition, logger *logrus.Entry) (*gardencorev1beta1.Shoot, error) {
	shoot := &gardencorev1beta1.Shoot{}
	if err := r.gardenClient.APIReader().Get(ctx, kutil.Key(managedSeed.Namespace, managedSeed.Spec.Shoot.Name), shoot); err != nil {
		if apierrors.IsNotFound(err) {
			message := fmt.Sprintf("Shoot %s/%s not found", managedSeed.Namespace, managedSeed.Spec.Shoot.Name)
			updateCondition(condition, gardencorev1beta1.ConditionFalse, "ShootNotFound", message)
			r.recordAndLog(managedSeed, corev1.EventTypeWarning, gardencorev1beta1.EventReconcileError, logger, logrus.ErrorLevel, message)
			return nil, fmt.Errorf("shoot %s/%s not found", managedSeed.Namespace, managedSeed.Spec.Shoot.Name)
		}
		message := fmt.Sprintf("Could not get shoot %s/%s: %+v", managedSeed.Namespace, managedSeed.Spec.Shoot.Name, err)
		updateCondition(condition, gardencorev1beta1.ConditionUnknown, "CouldNotGetShoot", message)
		r.recordAndLog(managedSeed, corev1.EventTypeWarning, gardencorev1beta1.EventReconcileError, logger, logrus.ErrorLevel, message)
		return nil, fmt.Errorf("could not get shoot %s/%s: %w", managedSeed.Namespace, managedSeed.Spec.Shoot.Name, err)
	}
	message := fmt.Sprintf("Shoot %s found", kutil.ObjectName(shoot))
	updateCondition(condition, gardencorev1beta1.ConditionTrue, "ShootFound", message)

	return shoot, nil
}

func (r *reconciler) recordAndLog(managedSeed *seedmanagementv1alpha1.ManagedSeed, eventType, eventReason string, logger *logrus.Entry, logLevel logrus.Level, message string) {
	r.recorder.Eventf(managedSeed, eventType, eventReason, "%s", message)
	logger.Log(logLevel, message)
}

func updateCondition(condition *gardencorev1beta1.Condition, status gardencorev1beta1.ConditionStatus, reason, message string) {
	*condition = gardencorev1beta1helper.UpdatedCondition(*condition, status, reason, message)
}

func updateStatus(ctx context.Context, c client.Client, managedSeed *seedmanagementv1alpha1.ManagedSeed, conditions ...gardencorev1beta1.Condition) error {
	return kutil.TryPatchStatus(ctx, retry.DefaultBackoff, c, managedSeed, func() error {
		managedSeed.Status.Conditions = gardencorev1beta1helper.MergeConditions(managedSeed.Status.Conditions, getUpdatedConditions(conditions)...)
		managedSeed.Status.ObservedGeneration = managedSeed.Generation
		return nil
	})
}

func getUpdatedConditions(conditions []gardencorev1beta1.Condition) []gardencorev1beta1.Condition {
	var updatedConditions []gardencorev1beta1.Condition
	for _, condition := range conditions {
		if condition.Reason != "ConditionInitialized" {
			updatedConditions = append(updatedConditions, condition)
		}
	}
	return updatedConditions
}

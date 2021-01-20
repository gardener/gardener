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
	if err := r.gardenClient.DirectClient().Get(ctx, request.NamespacedName, managedSeed); err != nil {
		if apierrors.IsNotFound(err) {
			r.logger.Debugf("Skipping ManagedSeed %s because it has been deleted", request.NamespacedName)
			return reconcile.Result{}, nil
		}
		r.logger.Errorf("Could not get ManagedSeed %s from store: %+v", request.NamespacedName, err)
		return reconcile.Result{}, err
	}

	if managedSeed.DeletionTimestamp != nil {
		return r.delete(ctx, managedSeed)
	}
	return r.reconcile(ctx, managedSeed)
}

func (r *reconciler) reconcile(ctx context.Context, managedSeed *seedmanagementv1alpha1.ManagedSeed) (reconcile.Result, error) {
	managedSeedLogger := logger.NewFieldLogger(r.logger, "managedSeed", kutil.ObjectName(managedSeed))

	// Ensure gardener finalizer
	if err := controllerutils.PatchFinalizers(ctx, r.gardenClient.Client(), managedSeed, gardencorev1beta1.GardenerName); err != nil {
		managedSeedLogger.Errorf("Could not ensure gardener finalizer: %+v", err)
		return reconcile.Result{}, err
	}

	conditionShootExists := gardencorev1beta1helper.GetOrInitCondition(managedSeed.Status.Conditions, seedmanagementv1alpha1.ManagedSeedShootExists)
	conditionShootReconciled := gardencorev1beta1helper.GetOrInitCondition(managedSeed.Status.Conditions, seedmanagementv1alpha1.ManagedSeedShootReconciled)
	conditionSeedRegistered := gardencorev1beta1helper.GetOrInitCondition(managedSeed.Status.Conditions, seedmanagementv1alpha1.ManagedSeedSeedRegistered)

	defer func() {
		if err := updateStatus(ctx, r.gardenClient.Client(), managedSeed, conditionShootExists, conditionShootReconciled, conditionSeedRegistered); err != nil {
			managedSeedLogger.Errorf("Could not update status: %+v", err)
		}
	}()

	// Get shoot
	shoot := &gardencorev1beta1.Shoot{}
	if err := r.gardenClient.DirectClient().Get(ctx, kutil.Key(managedSeed.Namespace, managedSeed.Spec.Shoot.Name), shoot); err != nil {
		if apierrors.IsNotFound(err) {
			message := fmt.Sprintf("Shoot %s/%s not found", managedSeed.Namespace, managedSeed.Spec.Shoot.Name)
			conditionShootExists = gardencorev1beta1helper.UpdatedCondition(conditionShootExists, gardencorev1beta1.ConditionFalse, "ShootNotFound", message)
			managedSeedLogger.Error(message)
			r.recorder.Eventf(managedSeed, corev1.EventTypeWarning, gardencorev1beta1.EventReconcileError, "%s", message)
			return reconcile.Result{}, fmt.Errorf("shoot %s/%s not found: %w", managedSeed.Namespace, managedSeed.Spec.Shoot.Name, err)
		}
		return reconcile.Result{}, fmt.Errorf("could not get shoot %s/%s: %w", managedSeed.Namespace, managedSeed.Spec.Shoot.Name, err)
	}
	conditionShootExists = gardencorev1beta1helper.UpdatedCondition(conditionShootExists, gardencorev1beta1.ConditionTrue, "ShootFound",
		fmt.Sprintf("Shoot %s found", kutil.ObjectName(shoot)))

	// Check if the shoot is reconciled
	if shoot.Generation != shoot.Status.ObservedGeneration || shoot.Status.LastOperation == nil || shoot.Status.LastOperation.State != gardencorev1beta1.LastOperationStateSucceeded {
		message := fmt.Sprintf("Shoot %s is still reconciling", kutil.ObjectName(shoot))
		conditionShootReconciled = gardencorev1beta1helper.UpdatedCondition(conditionShootReconciled, gardencorev1beta1.ConditionFalse, "ShootStillReconciling", message)
		managedSeedLogger.Info(message)
		return reconcile.Result{RequeueAfter: 10 * time.Second}, nil
	}
	conditionShootReconciled = gardencorev1beta1helper.UpdatedCondition(conditionShootReconciled, gardencorev1beta1.ConditionTrue, "ShootReconciled",
		fmt.Sprintf("Shoot %s is reconciled", kutil.ObjectName(shoot)))

	// Reconcile creation or update
	if err := r.actuator.Reconcile(ctx, managedSeed, shoot); err != nil {
		message := fmt.Sprintf("Could not register seed: %+v", err)
		conditionSeedRegistered = gardencorev1beta1helper.UpdatedCondition(conditionSeedRegistered, gardencorev1beta1.ConditionFalse, "SeedRegistrationFailed", message)
		managedSeedLogger.Error(message)
		r.recorder.Eventf(managedSeed, corev1.EventTypeWarning, gardencorev1beta1.EventReconcileError, "%s", message)
		return reconcile.Result{}, fmt.Errorf("could not register seed: %w", err)
	}
	conditionSeedRegistered = gardencorev1beta1helper.UpdatedCondition(conditionSeedRegistered, gardencorev1beta1.ConditionTrue, "SeedRegistered",
		fmt.Sprintf("Shoot %s registered as seed", kutil.ObjectName(shoot)))

	// Return success result
	return reconcile.Result{}, nil
}

func (r *reconciler) delete(ctx context.Context, managedSeed *seedmanagementv1alpha1.ManagedSeed) (reconcile.Result, error) {
	managedSeedLogger := logger.NewFieldLogger(r.logger, "managedSeed", kutil.ObjectName(managedSeed))

	// Check gardener finalizer
	if !controllerutil.ContainsFinalizer(managedSeed, gardencorev1beta1.GardenerName) {
		managedSeedLogger.Debug("Skipping ManagedSeed as it does not have a finalizer")
		return reconcile.Result{}, nil
	}

	conditionShootExists := gardencorev1beta1helper.GetOrInitCondition(managedSeed.Status.Conditions, seedmanagementv1alpha1.ManagedSeedShootExists)
	conditionSeedRegistered := gardencorev1beta1helper.GetOrInitCondition(managedSeed.Status.Conditions, seedmanagementv1alpha1.ManagedSeedSeedRegistered)

	defer func() {
		if err := updateStatus(ctx, r.gardenClient.Client(), managedSeed, conditionShootExists, conditionSeedRegistered); err != nil {
			managedSeedLogger.Errorf("Could not update status: %+v", err)
		}
	}()

	// Get shoot
	shoot := &gardencorev1beta1.Shoot{}
	if err := r.gardenClient.DirectClient().Get(ctx, kutil.Key(managedSeed.Namespace, managedSeed.Spec.Shoot.Name), shoot); err != nil {
		if apierrors.IsNotFound(err) {
			message := fmt.Sprintf("Shoot %s/%s not found", managedSeed.Namespace, managedSeed.Spec.Shoot.Name)
			conditionShootExists = gardencorev1beta1helper.UpdatedCondition(conditionShootExists, gardencorev1beta1.ConditionFalse, "ShootNotFound", message)
			managedSeedLogger.Error(message)
			r.recorder.Eventf(managedSeed, corev1.EventTypeWarning, gardencorev1beta1.EventReconcileError, "%s", message)
			return reconcile.Result{}, fmt.Errorf("shoot %s/%s not found: %w", managedSeed.Namespace, managedSeed.Spec.Shoot.Name, err)
		}
		return reconcile.Result{}, fmt.Errorf("could not get shoot %s/%s: %w", managedSeed.Namespace, managedSeed.Spec.Shoot.Name, err)
	}
	conditionShootExists = gardencorev1beta1helper.UpdatedCondition(conditionShootExists, gardencorev1beta1.ConditionTrue, "ShootFound",
		fmt.Sprintf("Shoot %s found", kutil.ObjectName(shoot)))

	// Reconcile deletion
	if err := r.actuator.Delete(ctx, managedSeed, shoot); err != nil {
		message := fmt.Sprintf("Could not unregister seed: %+v", err)
		conditionSeedRegistered = gardencorev1beta1helper.UpdatedCondition(conditionSeedRegistered, gardencorev1beta1.ConditionFalse, "SeedUnregistrationFailed", message)
		managedSeedLogger.Error(message)
		r.recorder.Eventf(managedSeed, corev1.EventTypeWarning, gardencorev1beta1.EventReconcileError, "%s", message)
		return reconcile.Result{}, fmt.Errorf("could not unregister seed: %w", err)
	}
	conditionSeedRegistered = gardencorev1beta1helper.UpdatedCondition(conditionSeedRegistered, gardencorev1beta1.ConditionFalse, "SeedUnregistered",
		fmt.Sprintf("Shoot %s unregistered as seed", kutil.ObjectName(shoot)))

	// Return success result and remove finalizer
	return reconcile.Result{}, controllerutils.PatchRemoveFinalizers(ctx, r.gardenClient.Client(), managedSeed, gardencorev1beta1.GardenerName)
}

func updateStatus(ctx context.Context, c client.Client, managedSeed *seedmanagementv1alpha1.ManagedSeed, conditions ...gardencorev1beta1.Condition) error {
	return kutil.TryPatchStatus(ctx, retry.DefaultBackoff, c, managedSeed, func() error {
		managedSeed.Status.Conditions = gardencorev1beta1helper.MergeConditions(managedSeed.Status.Conditions, conditions...)
		managedSeed.Status.ObservedGeneration = managedSeed.Generation
		return nil
	})
}

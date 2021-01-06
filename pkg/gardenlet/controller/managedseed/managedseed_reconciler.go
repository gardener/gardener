// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// reconciler implements the reconcile.Reconcile interface for ManagedSeed reconciliation.
type reconciler struct {
	ctx         context.Context
	clientMap   clientmap.ClientMap
	config      *config.GardenletConfiguration
	imageVector imagevector.ImageVector
	recorder    record.EventRecorder
	logger      *logrus.Logger
}

// newReconciler returns the new ManagedSeed reconciler.
func newReconciler(ctx context.Context, clientMap clientmap.ClientMap, config *config.GardenletConfiguration, imageVector imagevector.ImageVector, recorder record.EventRecorder, logger *logrus.Logger) reconcile.Reconciler {
	return &reconciler{
		ctx:         ctx,
		clientMap:   clientMap,
		config:      config,
		imageVector: imageVector,
		recorder:    recorder,
		logger:      logger,
	}
}

func (r *reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	gardenClient, err := r.clientMap.GetClient(r.ctx, keys.ForGarden())
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("could not get garden client: %w", err)
	}

	managedSeed := &seedmanagementv1alpha1.ManagedSeed{}
	if err := gardenClient.DirectClient().Get(r.ctx, request.NamespacedName, managedSeed); err != nil {
		if apierrors.IsNotFound(err) {
			r.logger.Debugf("Skipping ManagedSeed %s because it has been deleted", request.NamespacedName)
			return reconcile.Result{}, nil
		}
		r.logger.Errorf("Could not get ManagedSeed %s from store: %+v", request.NamespacedName, err)
		return reconcile.Result{}, err
	}

	if managedSeed.DeletionTimestamp != nil {
		return r.delete(gardenClient, managedSeed)
	}
	return r.reconcile(gardenClient, managedSeed)
}

func (r *reconciler) reconcile(gardenClient kubernetes.Interface, managedSeed *seedmanagementv1alpha1.ManagedSeed) (reconcile.Result, error) {
	managedSeedLogger := logger.NewFieldLogger(r.logger, "managedSeed", kutil.ObjectName(managedSeed))

	// Ensure gardener finalizer
	if err := controllerutils.PatchFinalizers(r.ctx, gardenClient.Client(), managedSeed, gardencorev1beta1.GardenerName); err != nil {
		managedSeedLogger.Errorf("Could not ensure gardener finalizer: %+v", err)
		return reconcile.Result{}, err
	}

	conditionValid := gardencorev1beta1helper.GetOrInitCondition(managedSeed.Status.Conditions, seedmanagementv1alpha1.ManagedSeedValid)
	conditionShootReconciled := gardencorev1beta1helper.GetOrInitCondition(managedSeed.Status.Conditions, seedmanagementv1alpha1.ManagedSeedShootReconciled)
	conditionSeedRegistered := gardencorev1beta1helper.GetOrInitCondition(managedSeed.Status.Conditions, seedmanagementv1alpha1.ManagedSeedSeedRegistered)

	defer func() {
		if err := updateStatus(r.ctx, gardenClient.Client(), managedSeed, conditionValid, conditionShootReconciled, conditionSeedRegistered); err != nil {
			managedSeedLogger.Errorf("Could not update status: %+v", err)
		}
	}()

	// Get shoot
	shoot := &gardencorev1beta1.Shoot{}
	if err := gardenClient.DirectClient().Get(r.ctx, kutil.Key(managedSeed.Namespace, managedSeed.Spec.Shoot.Name), shoot); err != nil {
		if apierrors.IsNotFound(err) {
			message := fmt.Sprintf("Shoot %s not found: %+v", kutil.ObjectName(shoot), err)
			conditionValid = gardencorev1beta1helper.UpdatedCondition(conditionValid, gardencorev1beta1.ConditionFalse, "ShootNotFound", message)
			managedSeedLogger.Error(message)
			r.recorder.Eventf(managedSeed, corev1.EventTypeWarning, gardencorev1beta1.EventReconcileError, "%s", message)
			return reconcile.Result{}, fmt.Errorf("shoot %s not found: %w", kutil.ObjectName(shoot), err)
		}
		return reconcile.Result{}, fmt.Errorf("could not get shoot %s: %w", kutil.ObjectName(shoot), err)
	}

	// Check if shoot can be registered as seed
	if shoot.Spec.DNS == nil || shoot.Spec.DNS.Domain == nil {
		message := fmt.Sprintf("Shoot %s cannot be registered as seed as it does not specify a domain", kutil.ObjectName(shoot))
		conditionValid = gardencorev1beta1helper.UpdatedCondition(conditionValid, gardencorev1beta1.ConditionFalse, "ShootCantBeSeed", message)
		managedSeedLogger.Error(message)
		r.recorder.Eventf(managedSeed, corev1.EventTypeWarning, gardencorev1beta1.EventReconcileError, "%s", message)
		return reconcile.Result{}, fmt.Errorf("shoot %s cannot be registered as seed as it does not specify a domain", kutil.ObjectName(shoot))
	}

	conditionValid = gardencorev1beta1helper.UpdatedCondition(conditionValid, gardencorev1beta1.ConditionTrue, "ShootFoundAndCanBeSeed",
		fmt.Sprintf("Shoot %s found and can be registered as seed", kutil.ObjectName(shoot)))

	// Check if the shoot is reconciled
	if shoot.Generation != shoot.Status.ObservedGeneration || shoot.Status.LastOperation == nil || shoot.Status.LastOperation.State != gardencorev1beta1.LastOperationStateSucceeded {
		message := fmt.Sprintf("Shoot %s is still reconciling", kutil.ObjectName(shoot))
		conditionShootReconciled = gardencorev1beta1helper.UpdatedCondition(conditionShootReconciled, gardencorev1beta1.ConditionFalse, "ShootStillReconciling", message)
		managedSeedLogger.Info(message)
		return reconcile.Result{RequeueAfter: 10 * time.Second}, nil
	}
	conditionShootReconciled = gardencorev1beta1helper.UpdatedCondition(conditionShootReconciled, gardencorev1beta1.ConditionTrue, "ShootReconciled",
		fmt.Sprintf("Shoot %s is reconciled", kutil.ObjectName(shoot)))

	// Get seed client
	seedClient, err := r.clientMap.GetClient(r.ctx, keys.ForSeedWithName(*shoot.Spec.SeedName))
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("could not get seed client: %w", err)
	}

	// Get shoot client
	shootClient, err := r.clientMap.GetClient(r.ctx, keys.ForShoot(shoot))
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("could not get shoot client: %w", err)
	}

	// Create actuator
	a := newActuator(gardenClient, seedClient, shootClient, r.config, r.imageVector, r.logger)

	// Reconcile creation or update
	if err := a.Reconcile(r.ctx, managedSeed, shoot); err != nil {
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

func (r *reconciler) delete(gardenClient kubernetes.Interface, managedSeed *seedmanagementv1alpha1.ManagedSeed) (reconcile.Result, error) {
	managedSeedLogger := logger.NewFieldLogger(r.logger, "managedSeed", kutil.ObjectName(managedSeed))

	// Check gardener finalizer
	if !controllerutils.HasFinalizer(managedSeed, gardencorev1beta1.GardenerName) {
		managedSeedLogger.Debug("Skipping ManagedSeed as it does not have a finalizer")
		return reconcile.Result{}, nil
	}

	conditionValid := gardencorev1beta1helper.GetOrInitCondition(managedSeed.Status.Conditions, seedmanagementv1alpha1.ManagedSeedValid)
	conditionSeedRegistered := gardencorev1beta1helper.GetOrInitCondition(managedSeed.Status.Conditions, seedmanagementv1alpha1.ManagedSeedSeedRegistered)

	defer func() {
		if err := updateStatus(r.ctx, gardenClient.Client(), managedSeed, conditionValid, conditionSeedRegistered); err != nil {
			managedSeedLogger.Errorf("Could not update status: %+v", err)
		}
	}()

	// Get shoot
	shoot := &gardencorev1beta1.Shoot{}
	if err := gardenClient.DirectClient().Get(r.ctx, kutil.Key(managedSeed.Namespace, managedSeed.Spec.Shoot.Name), shoot); err != nil {
		if apierrors.IsNotFound(err) {
			message := fmt.Sprintf("Shoot %s not found: %+v", kutil.ObjectName(shoot), err)
			conditionValid = gardencorev1beta1helper.UpdatedCondition(conditionValid, gardencorev1beta1.ConditionFalse, "ShootNotFound", message)
			managedSeedLogger.Error(message)
			r.recorder.Eventf(managedSeed, corev1.EventTypeWarning, gardencorev1beta1.EventReconcileError, "%s", message)
			return reconcile.Result{}, fmt.Errorf("shoot %s not found: %w", kutil.ObjectName(shoot), err)
		}
		return reconcile.Result{}, fmt.Errorf("could not get shoot %s: %w", kutil.ObjectName(shoot), err)
	}
	conditionValid = gardencorev1beta1helper.UpdatedCondition(conditionValid, gardencorev1beta1.ConditionTrue, "ShootFound",
		fmt.Sprintf("Shoot %s found", kutil.ObjectName(shoot)))

	// Get seed client
	seedClient, err := r.clientMap.GetClient(r.ctx, keys.ForSeedWithName(*shoot.Spec.SeedName))
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("could not get seed client: %w", err)
	}

	// Get shoot client
	shootClient, err := r.clientMap.GetClient(r.ctx, keys.ForShoot(shoot))
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("could not get shoot client: %w", err)
	}

	// Create actuator
	a := newActuator(gardenClient, seedClient, shootClient, r.config, r.imageVector, r.logger)

	// Reconcile deletion
	if err := a.Delete(r.ctx, managedSeed, shoot); err != nil {
		message := fmt.Sprintf("Could not unregister seed: %+v", err)
		conditionSeedRegistered = gardencorev1beta1helper.UpdatedCondition(conditionSeedRegistered, gardencorev1beta1.ConditionFalse, "SeedUnregistrationFailed", message)
		managedSeedLogger.Error(message)
		r.recorder.Eventf(managedSeed, corev1.EventTypeWarning, gardencorev1beta1.EventReconcileError, "%s", message)
		return reconcile.Result{}, fmt.Errorf("could not unregister seed: %w", err)
	}
	conditionSeedRegistered = gardencorev1beta1helper.UpdatedCondition(conditionSeedRegistered, gardencorev1beta1.ConditionFalse, "SeedUnregistered",
		fmt.Sprintf("Shoot %s unregistered as seed", kutil.ObjectName(shoot)))

	// Return success result and remove finalizer
	return reconcile.Result{}, controllerutils.PatchRemoveFinalizers(r.ctx, gardenClient.Client(), managedSeed, gardencorev1beta1.GardenerName)
}

func updateStatus(ctx context.Context, c client.Client, managedSeed *seedmanagementv1alpha1.ManagedSeed, conditions ...gardencorev1beta1.Condition) error {
	return kutil.TryPatchStatus(ctx, retry.DefaultBackoff, c, managedSeed, func() error {
		managedSeed.Status.Conditions = gardencorev1beta1helper.MergeConditions(managedSeed.Status.Conditions, conditions...)
		managedSeed.Status.ObservedGeneration = managedSeed.Generation
		return nil
	})
}

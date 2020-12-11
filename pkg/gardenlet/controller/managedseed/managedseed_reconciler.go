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
	"errors"
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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
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
	if err := gardenClient.Client().Get(r.ctx, request.NamespacedName, managedSeed); err != nil {
		if apierrors.IsNotFound(err) {
			r.logger.Debugf("Skipping ManagedSeed %s because it has been deleted", request.NamespacedName)
			return reconcile.Result{}, nil
		}
		r.logger.Errorf("Could not get ManagedSeed %s from store: %v", request.NamespacedName, err)
		return reconcile.Result{}, err
	}

	if managedSeed.Spec.Shoot == nil || managedSeed.Spec.Shoot.Name == "" {
		r.logger.Errorf("Skipping ManagedSeed %s because it does not specify a shoot", request.NamespacedName)
		return reconcile.Result{}, nil
	}

	if managedSeed.DeletionTimestamp != nil {
		return r.delete(gardenClient, managedSeed)
	}
	return r.createOrUpdate(gardenClient, managedSeed)
}

func (r *reconciler) createOrUpdate(gardenClient kubernetes.Interface, managedSeed *seedmanagementv1alpha1.ManagedSeed) (reconcile.Result, error) {
	managedSeedLogger := logger.NewFieldLogger(r.logger, "managedSeed", kutil.ObjectName(managedSeed))

	// Ensure gardener finalizer
	if err := controllerutils.EnsureFinalizer(r.ctx, gardenClient.DirectClient(), managedSeed, gardencorev1beta1.GardenerName); err != nil {
		managedSeedLogger.Errorf("Could not ensure gardener finalizer on ManagedSeed: %+v", err)
		return reconcile.Result{}, err
	}

	// Get shoot
	shoot := &gardencorev1beta1.Shoot{}
	if err := gardenClient.Client().Get(r.ctx, kutil.Key(managedSeed.Namespace, managedSeed.Spec.Shoot.Name), shoot); err != nil {
		return reconcile.Result{}, err
	}

	// Check if the shoot is reconciled
	if shoot.Generation != shoot.Status.ObservedGeneration || shoot.Status.LastOperation == nil || shoot.Status.LastOperation.State != gardencorev1beta1.LastOperationStateSucceeded {
		managedSeedLogger.Infof("Waiting for shoot %s to be reconciled before registering it as seed", kutil.ObjectName(shoot))

		// Update status to Pending
		if updateErr := updateStatusPending(r.ctx, gardenClient.DirectClient(), managedSeed, "Reconciliation pending"); updateErr != nil {
			managedSeedLogger.Errorf("Could not update ManagedSeed status for pending creation or update: %+v", updateErr)
			return reconcile.Result{}, updateErr
		}

		// Return success result with requeue after 10 seconds
		return reconcile.Result{RequeueAfter: 10 * time.Second}, nil
	}

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

	// Update status to Processing
	if updateErr := updateStatusProcessing(r.ctx, gardenClient.DirectClient(), managedSeed, "Reconciling"); updateErr != nil {
		managedSeedLogger.Errorf("Could not update ManagedSeed status before creation or update: %+v", updateErr)
		return reconcile.Result{}, updateErr
	}

	// Reconcile creation or update
	managedSeedLogger.Debug("Reconciling ManagedSeed creation or update")
	if err := a.CreateOrUpdate(r.ctx, managedSeed, shoot); err != nil {
		managedSeedLogger.Errorf("Could not reconcile ManagedSeed creation or update: %+v", err)

		// Record an event
		lastError := &gardencorev1beta1.LastError{
			Codes:       gardencorev1beta1helper.ExtractErrorCodes(err),
			Description: "Could not reconcile creation or update: " + err.Error(),
		}
		r.recorder.Eventf(managedSeed, corev1.EventTypeWarning, gardencorev1beta1.EventReconcileError, "%s", lastError.Description)

		// Update status to Error
		if updateErr := updateStatusError(r.ctx, gardenClient.DirectClient(), managedSeed, lastError.Description, lastError); updateErr != nil {
			managedSeedLogger.Errorf("Could not update ManagedSeed status after creation or update error: %+v", updateErr)
			return reconcile.Result{}, updateErr
		}

		// Return error result
		return reconcile.Result{}, errors.New(lastError.Description)
	}
	managedSeedLogger.Debug("Successfully reconciled ManagedSeed creation or update")

	// Update status to Succeeded
	if updateErr := updateStatusSucceeded(r.ctx, gardenClient.DirectClient(), managedSeed, "Reconciled successfully"); updateErr != nil {
		managedSeedLogger.Errorf("Could not update ManagedSeed status after creation or update success: %+v", updateErr)
		return reconcile.Result{}, updateErr
	}

	// Return success result
	return reconcile.Result{}, nil
}

func (r *reconciler) delete(gardenClient kubernetes.Interface, managedSeed *seedmanagementv1alpha1.ManagedSeed) (reconcile.Result, error) {
	managedSeedLogger := logger.NewFieldLogger(r.logger, "managedSeed", kutil.ObjectName(managedSeed))

	// Check gardener finalizer
	if !sets.NewString(managedSeed.Finalizers...).Has(gardencorev1beta1.GardenerName) {
		managedSeedLogger.Debug("Skipping ManagedSeed as it does not have a finalizer")
		return reconcile.Result{}, nil
	}

	// Get shoot
	shoot := &gardencorev1beta1.Shoot{}
	if err := gardenClient.Client().Get(r.ctx, kutil.Key(managedSeed.Namespace, managedSeed.Spec.Shoot.Name), shoot); err != nil {
		return reconcile.Result{}, err
	}

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

	// Update status to Processing
	if updateErr := updateStatusProcessing(r.ctx, gardenClient.DirectClient(), managedSeed, "Deleting"); updateErr != nil {
		managedSeedLogger.Errorf("Could not update ManagedSeed status before deletion: %+v", updateErr)
		return reconcile.Result{}, updateErr
	}

	// Reconcile deletion
	managedSeedLogger.Debug("Reconciling ManagedSeed deletion")
	if err := a.Delete(r.ctx, managedSeed, shoot); err != nil {
		managedSeedLogger.Errorf("Could not reconcile ManagedSeed deletion: %+v", err)

		// Record an event
		lastError := &gardencorev1beta1.LastError{
			Codes:       gardencorev1beta1helper.ExtractErrorCodes(err),
			Description: "Could not reconcile deletion: " + err.Error(),
		}
		r.recorder.Eventf(managedSeed, corev1.EventTypeWarning, gardencorev1beta1.EventDeleteError, "%s", lastError.Description)

		// Update status to Error
		if updateErr := updateStatusError(r.ctx, gardenClient.DirectClient(), managedSeed, lastError.Description, lastError); updateErr != nil {
			managedSeedLogger.Errorf("Could not update ManagedSeed status after deletion error: %+v", updateErr)
			return reconcile.Result{}, updateErr
		}

		// Retrun error result
		return reconcile.Result{}, errors.New(lastError.Description)
	}
	managedSeedLogger.Debug("Successfully reconciled ManagedSeed deletion")

	// Update status to Succeeded
	if updateErr := updateStatusSucceeded(r.ctx, gardenClient.DirectClient(), managedSeed, "Deleted successfully"); updateErr != nil {
		managedSeedLogger.Errorf("Could not update ManagedSeed status after deletion success: %+v", updateErr)
		return reconcile.Result{}, updateErr
	}

	// Return success result and remove finalizer
	return reconcile.Result{}, controllerutils.RemoveGardenerFinalizer(r.ctx, gardenClient.DirectClient(), managedSeed)
}

func updateStatusProcessing(ctx context.Context, c client.Client, managedSeed *seedmanagementv1alpha1.ManagedSeed, description string) error {
	return updateStatus(ctx, c, managedSeed, description, gardencorev1beta1.LastOperationStateProcessing, 0, false, false, nil)
}

func updateStatusError(ctx context.Context, c client.Client, managedSeed *seedmanagementv1alpha1.ManagedSeed, description string, lastError *gardencorev1beta1.LastError) error {
	return updateStatus(ctx, c, managedSeed, description, gardencorev1beta1.LastOperationStateError, 0, false, true, lastError)
}

func updateStatusSucceeded(ctx context.Context, c client.Client, managedSeed *seedmanagementv1alpha1.ManagedSeed, description string) error {
	return updateStatus(ctx, c, managedSeed, description, gardencorev1beta1.LastOperationStateSucceeded, 100, true, true, nil)
}

func updateStatusPending(ctx context.Context, c client.Client, managedSeed *seedmanagementv1alpha1.ManagedSeed, description string) error {
	return updateStatus(ctx, c, managedSeed, description, gardencorev1beta1.LastOperationStatePending, 0, true, false, nil)
}

func updateStatus(ctx context.Context, c client.Client, managedSeed *seedmanagementv1alpha1.ManagedSeed, description string, state gardencorev1beta1.LastOperationState, progress int32, updateObservedGeneration, updateLastError bool, lastError *gardencorev1beta1.LastError) error {
	return kutil.TryUpdateStatus(ctx, retry.DefaultBackoff, c, managedSeed, func() error {
		managedSeed.Status.LastOperation = &gardencorev1beta1.LastOperation{
			Description:    description,
			LastUpdateTime: metav1.Now(),
			Progress:       progress,
			State:          state,
			Type:           gardencorev1beta1helper.ComputeOperationType(managedSeed.ObjectMeta, managedSeed.Status.LastOperation),
		}
		if updateLastError {
			managedSeed.Status.LastError = lastError
		}
		if updateObservedGeneration {
			managedSeed.Status.ObservedGeneration = managedSeed.Generation
		}
		return nil
	})
}

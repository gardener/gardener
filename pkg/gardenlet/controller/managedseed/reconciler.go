// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package managedseed

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
)

// Reconciler reconciles the ManagedSeed.
type Reconciler struct {
	GardenClient          client.Client
	Actuator              Actuator
	Config                config.GardenletConfiguration
	Clock                 clock.Clock
	ShootClientMap        clientmap.ClientMap
	GardenNamespaceGarden string
	GardenNamespaceShoot  string
}

// Reconcile performs the main reconciliation logic.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	ctx, cancel := controllerutils.GetMainReconciliationContext(ctx, r.Config.Controllers.ManagedSeed.SyncPeriod.Duration)
	defer cancel()

	ms := &seedmanagementv1alpha1.ManagedSeed{}
	if err := r.GardenClient.Get(ctx, request.NamespacedName, ms); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	if ms.DeletionTimestamp != nil {
		return r.delete(ctx, log, ms)
	}
	return r.reconcile(ctx, log, ms)
}

func (r *Reconciler) reconcile(
	ctx context.Context,
	log logr.Logger,
	ms *seedmanagementv1alpha1.ManagedSeed,
) (
	result reconcile.Result,
	err error,
) {
	// Ensure gardener finalizer
	if !controllerutil.ContainsFinalizer(ms, gardencorev1beta1.GardenerName) {
		log.Info("Adding finalizer")
		if err := controllerutils.AddFinalizers(ctx, r.GardenClient, ms, gardencorev1beta1.GardenerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to add finalizer: %w", err)
		}
	}

	var status *seedmanagementv1alpha1.ManagedSeedStatus
	defer func() {
		// Update status, on failure return the update error unless there is another error
		if updateErr := r.updateStatus(ctx, ms, status); updateErr != nil && err == nil {
			err = fmt.Errorf("could not update status: %w", updateErr)
		}
	}()

	// Reconcile creation or update
	log.V(1).Info("Reconciling")
	var wait bool
	if status, wait, err = r.Actuator.Reconcile(ctx, log, ms); err != nil {
		return reconcile.Result{}, fmt.Errorf("could not reconcile ManagedSeed %s creation or update: %w", client.ObjectKeyFromObject(ms), err)
	}
	log.V(1).Info("Reconciliation finished")

	// If waiting, requeue after WaitSyncPeriod
	if wait {
		return reconcile.Result{RequeueAfter: r.Config.Controllers.ManagedSeed.WaitSyncPeriod.Duration}, nil
	}

	// Return success result
	return reconcile.Result{RequeueAfter: r.Config.Controllers.ManagedSeed.SyncPeriod.Duration}, nil
}

func (r *Reconciler) delete(
	ctx context.Context,
	log logr.Logger,
	ms *seedmanagementv1alpha1.ManagedSeed,
) (
	result reconcile.Result,
	err error,
) {
	// Check gardener finalizer
	if !controllerutil.ContainsFinalizer(ms, gardencorev1beta1.GardenerName) {
		log.V(1).Info("Skipping deletion as object does not have a finalizer")
		return reconcile.Result{}, nil
	}

	var status *seedmanagementv1alpha1.ManagedSeedStatus
	var wait, removeFinalizer bool
	defer func() {
		// Only update status if the finalizer is not removed to prevent errors if the object is already gone
		if !removeFinalizer {
			// Update status, on failure return the update error unless there is another error
			if updateErr := r.updateStatus(ctx, ms, status); updateErr != nil && err == nil {
				err = fmt.Errorf("could not update status: %w", updateErr)
			}
		}
	}()

	// Reconcile deletion
	log.V(1).Info("Deletion")
	if status, wait, removeFinalizer, err = r.Actuator.Delete(ctx, log, ms); err != nil {
		return reconcile.Result{}, fmt.Errorf("could not reconcile ManagedSeed %s deletion: %w", client.ObjectKeyFromObject(ms), err)
	}
	log.V(1).Info("Deletion finished")

	// If waiting, requeue after WaitSyncPeriod
	if wait {
		return reconcile.Result{RequeueAfter: r.Config.Controllers.ManagedSeed.WaitSyncPeriod.Duration}, nil
	}

	// Remove gardener finalizer if requested by the actuator
	if removeFinalizer {
		if controllerutil.ContainsFinalizer(ms, gardencorev1beta1.GardenerName) {
			log.Info("Removing finalizer")
			if err := controllerutils.RemoveFinalizers(ctx, r.GardenClient, ms, gardencorev1beta1.GardenerName); err != nil {
				return reconcile.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
			}
		}
		return reconcile.Result{}, nil
	}

	// Return success result
	return reconcile.Result{RequeueAfter: r.Config.Controllers.ManagedSeed.SyncPeriod.Duration}, nil
}

func (r *Reconciler) updateStatus(ctx context.Context, ms *seedmanagementv1alpha1.ManagedSeed, status *seedmanagementv1alpha1.ManagedSeedStatus) error {
	if status == nil {
		return nil
	}
	patch := client.StrategicMergeFrom(ms.DeepCopy())
	ms.Status = *status
	return r.GardenClient.Status().Patch(ctx, ms, patch)
}

// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package managedseedset

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	controllermanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/controllermanager/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
)

// Reconciler reconciles the ManagedSeedSet.
type Reconciler struct {
	Client   client.Client
	Config   controllermanagerconfigv1alpha1.ManagedSeedSetControllerConfiguration
	Actuator Actuator
}

// Reconcile performs the main reconciliation logic.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	ctx, cancel := controllerutils.GetMainReconciliationContext(ctx, r.Config.SyncPeriod.Duration)
	defer cancel()

	managedSeedSet := &seedmanagementv1alpha1.ManagedSeedSet{}
	if err := r.Client.Get(ctx, request.NamespacedName, managedSeedSet); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	if managedSeedSet.DeletionTimestamp != nil {
		return r.delete(ctx, log, managedSeedSet)
	}
	return r.reconcile(ctx, log, managedSeedSet)
}

func (r *Reconciler) reconcile(ctx context.Context, log logr.Logger, managedSeedSet *seedmanagementv1alpha1.ManagedSeedSet) (result reconcile.Result, err error) {
	// Ensure gardener finalizer
	if !controllerutil.ContainsFinalizer(managedSeedSet, gardencorev1beta1.GardenerName) {
		log.Info("Adding finalizer")
		if err := controllerutils.AddFinalizers(ctx, r.Client, managedSeedSet, gardencorev1beta1.GardenerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("could not add finalizer: %w", err)
		}
	}

	var status *seedmanagementv1alpha1.ManagedSeedSetStatus
	defer func() {
		// Update status, on failure return the update error unless there is another error
		if updateErr := r.updateStatus(ctx, managedSeedSet, status); updateErr != nil && err == nil {
			err = fmt.Errorf("could not update status: %w", updateErr)
		}
	}()

	// Reconcile creation or update
	log.V(1).Info("Reconciling creation or update")
	if status, _, err = r.Actuator.Reconcile(ctx, log, managedSeedSet); err != nil {
		return reconcile.Result{}, fmt.Errorf("could not reconcile ManagedSeedSet %s creation or update: %w", client.ObjectKeyFromObject(managedSeedSet), err)
	}
	log.V(1).Info("Creation or update reconciled")

	// Return success result
	return reconcile.Result{RequeueAfter: r.Config.SyncPeriod.Duration}, nil
}

func (r *Reconciler) delete(ctx context.Context, log logr.Logger, managedSeedSet *seedmanagementv1alpha1.ManagedSeedSet) (result reconcile.Result, err error) {
	// Check gardener finalizer
	if !controllerutil.ContainsFinalizer(managedSeedSet, gardencorev1beta1.GardenerName) {
		log.V(1).Info("Skipping as it does not have a finalizer")
		return reconcile.Result{}, nil
	}

	var (
		status          *seedmanagementv1alpha1.ManagedSeedSetStatus
		removeFinalizer bool
	)

	defer func() {
		// Only update status if the finalizer is not removed to prevent errors if the object is already gone
		if !removeFinalizer {
			// Update status, on failure return the update error unless there is another error
			if updateErr := r.updateStatus(ctx, managedSeedSet, status); updateErr != nil && err == nil {
				err = fmt.Errorf("could not update status: %w", updateErr)
			}
		}
	}()

	// Reconcile deletion
	log.V(1).Info("Reconciling deletion")
	if status, removeFinalizer, err = r.Actuator.Reconcile(ctx, log, managedSeedSet); err != nil {
		return reconcile.Result{}, fmt.Errorf("could not reconcile ManagedSeedSet %s deletion: %w", client.ObjectKeyFromObject(managedSeedSet), err)
	}
	log.V(1).Info("Deletion reconciled")

	// Remove gardener finalizer if requested by the actuator
	if removeFinalizer {
		if controllerutil.ContainsFinalizer(managedSeedSet, gardencorev1beta1.GardenerName) {
			log.Info("Removing finalizer")
			if err := controllerutils.RemoveFinalizers(ctx, r.Client, managedSeedSet, gardencorev1beta1.GardenerName); err != nil {
				return reconcile.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
			}
		}

		return reconcile.Result{}, nil
	}

	// Return success result
	return reconcile.Result{RequeueAfter: r.Config.SyncPeriod.Duration}, nil
}

func (r *Reconciler) updateStatus(ctx context.Context, managedSeedSet *seedmanagementv1alpha1.ManagedSeedSet, status *seedmanagementv1alpha1.ManagedSeedSetStatus) error {
	if status == nil {
		return nil
	}
	patch := client.StrategicMergeFrom(managedSeedSet.DeepCopy())
	managedSeedSet.Status = *status
	return r.Client.Status().Patch(ctx, managedSeedSet, patch)
}

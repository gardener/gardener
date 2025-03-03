// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package finalizer

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	controllermanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/controllermanager/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
)

// FinalizerName is the finalizer used by the controller.
const FinalizerName = "core.gardener.cloud/shootstate"

// Reconciler reconciles ShootState objects and ensures the finalizer
// exists during "Migrate" and "Restore" phases of Shoot migration to
// another Seed.
type Reconciler struct {
	// Client is the API Server client used by the Reconciler.
	Client client.Client
	Config controllermanagerconfigv1alpha1.ShootStateControllerConfiguration
}

// Reconcile reconciles ShootStates.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	shootState := &gardencorev1beta1.ShootState{}
	if err := r.Client.Get(ctx, request.NamespacedName, shootState); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Did not manage to retrieve ShootState object")
			// Since reconciliation runs on Shoot update, we should not
			// flood the logs with errors when a migration is not initiated
			// and the `ShootState` is not supposed to exist.
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	shoot := &gardencorev1beta1.Shoot{}
	if err := r.Client.Get(ctx, request.NamespacedName, shoot); err != nil {
		log.Info("Did not manage to retrieve Shoot for the given ShootState")
		return reconcile.Result{}, fmt.Errorf("error retrieving Shoot from store: %w", err)
	}

	shootLastOperationType := shoot.Status.LastOperation.Type
	shootLastOperationState := shoot.Status.LastOperation.State

	isShootOpMigrate := shootLastOperationType == gardencorev1beta1.LastOperationTypeMigrate
	isShootOpRestore := shootLastOperationType == gardencorev1beta1.LastOperationTypeRestore
	isShootOpReconcile := shootLastOperationType == gardencorev1beta1.LastOperationTypeReconcile

	isShootOpSucceeded := shootLastOperationState == gardencorev1beta1.LastOperationStateSucceeded
	shootStateHasFinalizer := controllerutil.ContainsFinalizer(shootState, FinalizerName)

	log = log.
		WithValues("shootLastOperationType", shootLastOperationType).
		WithValues("shootLastOperationState", shootLastOperationState)

	isShootRestoreNotSucceeded := isShootOpRestore && !isShootOpSucceeded
	if (isShootOpMigrate || isShootRestoreNotSucceeded) && !shootStateHasFinalizer {
		log.Info("Adding finalizer")
		if err := controllerutils.AddFinalizers(ctx, r.Client, shootState, FinalizerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("error adding finalizer to ShootState: %w", err)
		}
		return reconcile.Result{}, nil
	}

	isShootRestoreSucceeded := isShootOpRestore && isShootOpSucceeded
	if (isShootRestoreSucceeded || isShootOpReconcile) && shootStateHasFinalizer {
		log.Info("Removing finalizer")
		if err := controllerutils.RemoveFinalizers(ctx, r.Client, shootState, FinalizerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("error removing finalizer from ShootState: %w", err)
		}
		return reconcile.Result{}, nil
	}

	log.Info("No changes applied to the ShootState")
	return reconcile.Result{}, nil
}

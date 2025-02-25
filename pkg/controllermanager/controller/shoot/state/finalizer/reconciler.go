// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package finalizer

import (
	"context"
	"fmt"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/controllerutils"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const FinalizerName = "core.gardener.cloud/shootstate"

type Reconciler struct {
	Client client.Client
}

func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	shootState := &gardencorev1beta1.ShootState{}
	if err := r.Client.Get(ctx, request.NamespacedName, shootState); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Did not manage to retrieve ShootState object")
			return reconcile.Result{}, fmt.Errorf("error retrieving ShootState from store: %w", err)
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

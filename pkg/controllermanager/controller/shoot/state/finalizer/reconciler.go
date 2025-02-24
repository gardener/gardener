// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package finalizer

import (
	"context"

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
	log := logf.FromContext(ctx).WithValues("reconcileRequest", request)

	shoot := &gardencorev1beta1.Shoot{}
	if err := r.Client.Get(ctx, request.NamespacedName, shoot); err != nil {
		log.Info("Did not manage to retrieve Shoot for the given ShootState")
		return reconcile.Result{}, err
	}

	shootUid := shoot.ObjectMeta.UID
	log = log.WithValues("shootUid", shootUid)

	shootState := &gardencorev1beta1.ShootState{}
	if err := r.Client.Get(ctx, client.ObjectKeyFromObject(shoot), shootState); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Did not manage to retrieve ShootState object")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	shootStateUid := shoot.ObjectMeta.UID
	shootLastOperationType := shoot.Status.LastOperation.Type
	shootLastOperationState := shoot.Status.LastOperation.State
	shootLastOpProgress := shoot.Status.LastOperation.Progress
	shootStateHasFinalizer := controllerutil.ContainsFinalizer(shootState, FinalizerName)

	log = log.
		WithValues("shootStateUid", shootStateUid).
		WithValues("shootLastOperationType", shootLastOperationType).
		WithValues("shootLastOperationState", shootLastOperationState).
		WithValues("shootLastOpProgress", shootLastOpProgress).
		WithValues("isFinalizerAdded", shootStateHasFinalizer).
		WithValues("finalizer", FinalizerName)

	isShootOpMigrate := shootLastOperationType == gardencorev1beta1.LastOperationTypeMigrate
	if isShootOpMigrate && !shootStateHasFinalizer {
		log.Info("Adding finalizer")
		if err := controllerutils.AddFinalizers(ctx, r.Client, shootState, FinalizerName); err != nil {
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, nil
	}

	isShootOpRestore := shootLastOperationType == gardencorev1beta1.LastOperationTypeRestore
	isShootOpSucceeded := shootLastOperationState == gardencorev1beta1.LastOperationStateSucceeded
	if isShootOpRestore && isShootOpSucceeded && shootStateHasFinalizer {
		log.Info("Removing finalizer")
		if err := controllerutils.RemoveFinalizers(ctx, r.Client, shootState, FinalizerName); err != nil {
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, nil
	}

	log.Info("No changes applied to the ShootState")
	return reconcile.Result{}, nil
}

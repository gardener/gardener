// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package seedfinalizer

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/controllerutils"
)

// FinalizerName is the finalizer used by this controller.
const FinalizerName = "core.gardener.cloud/controllerregistration"

// Reconciler reconciles Seeds and manages the finalizer on these objects depending on whether ControllerInstallation
// objects exist in the system.
// It basically protects Seeds from being deleted, if there are still ControllerInstallations referencing it, to make
// sure we are able to cleanup ControllerInstallation objects of terminating Seeds.
type Reconciler struct {
	Client client.Client
}

// Reconcile reconciles Seeds.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	ctx, cancel := controllerutils.GetMainReconciliationContext(ctx, controllerutils.DefaultReconciliationTimeout)
	defer cancel()

	seed := &gardencorev1beta1.Seed{}
	if err := r.Client.Get(ctx, request.NamespacedName, seed); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}

		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	if seed.DeletionTimestamp != nil {
		if !controllerutil.ContainsFinalizer(seed, FinalizerName) {
			return reconcile.Result{}, nil
		}

		controllerInstallationList := &gardencorev1beta1.ControllerInstallationList{}
		if err := r.Client.List(ctx, controllerInstallationList, client.MatchingFields{gardencore.SeedRefName: seed.Name}); err != nil {
			return reconcile.Result{}, err
		}

		if len(controllerInstallationList.Items) > 0 {
			// cannot remove finalizer yet, requeue will happen via watch on controllerinstallations
			return reconcile.Result{}, nil
		}

		log.Info("Removing finalizer")
		if err := controllerutils.RemoveFinalizers(ctx, r.Client, seed, FinalizerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
		}

		return reconcile.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(seed, FinalizerName) {
		log.Info("Adding finalizer")
		if err := controllerutils.AddFinalizers(ctx, r.Client, seed, FinalizerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to add finalizer: %w", err)
		}
	}

	return reconcile.Result{}, nil
}

// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package clusterfinalizer

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/controllerutils"
)

// FinalizerName is the finalizer used by this controller.
const FinalizerName = "core.gardener.cloud/controllerregistration"

// Reconciler reconciles Seeds or Shoots and manages the finalizer on these objects depending on whether
// ControllerInstallation objects exist in the system.
// It basically protects Seeds from being deleted, if there are still ControllerInstallations referencing it, to make
// sure we are able to clean up ControllerInstallation objects of terminating Seeds/Shoots.
type Reconciler struct {
	Client                            client.Client
	NewTargetObjectFunc               func() client.Object
	NewControllerInstallationSelector func(obj client.Object) client.MatchingFields
}

// Reconcile reconciles Seeds.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	obj := r.NewTargetObjectFunc()
	if err := r.Client.Get(ctx, request.NamespacedName, obj); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}

		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	if obj.GetDeletionTimestamp() != nil {
		if !controllerutil.ContainsFinalizer(obj, FinalizerName) {
			return reconcile.Result{}, nil
		}

		controllerInstallationList := &gardencorev1beta1.ControllerInstallationList{}
		if err := r.Client.List(ctx, controllerInstallationList, r.NewControllerInstallationSelector(obj)); err != nil {
			return reconcile.Result{}, err
		}

		if len(controllerInstallationList.Items) > 0 {
			// cannot remove finalizer yet, requeue will happen via watch on ControllerInstallations
			return reconcile.Result{}, nil
		}

		log.Info("Removing finalizer")
		if err := controllerutils.RemoveFinalizers(ctx, r.Client, obj, FinalizerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
		}

		return reconcile.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(obj, FinalizerName) {
		log.Info("Adding finalizer")
		if err := controllerutils.AddFinalizers(ctx, r.Client, obj, FinalizerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to add finalizer: %w", err)
		}
	}

	return reconcile.Result{}, nil
}

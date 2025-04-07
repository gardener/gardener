// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package exposureclass

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	controllermanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/controllermanager/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
)

// Reconciler reconciles ExposureClass.
type Reconciler struct {
	Client   client.Client
	Config   controllermanagerconfigv1alpha1.ExposureClassControllerConfiguration
	Recorder record.EventRecorder

	// RateLimiter allows limiting exponential backoff for testing purposes
	RateLimiter workqueue.TypedRateLimiter[reconcile.Request]
}

// Reconcile performs the main reconciliation logic.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	ctx, cancel := controllerutils.GetMainReconciliationContext(ctx, controllerutils.DefaultReconciliationTimeout)
	defer cancel()

	exposureClass := &gardencorev1beta1.ExposureClass{}
	if err := r.Client.Get(ctx, request.NamespacedName, exposureClass); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	if exposureClass.DeletionTimestamp != nil {
		// Ignore the exposure class if it has no gardener finalizer.
		if !sets.New(exposureClass.Finalizers...).Has(gardencorev1beta1.GardenerName) {
			return reconcile.Result{}, nil
		}

		// Lookup shoots which reference the exposure class. The finalizer will be only
		// removed if there is no Shoot referencing the exposure class anymore.
		associatedShoots, err := controllerutils.DetermineShootsAssociatedTo(ctx, r.Client, exposureClass)
		if err != nil {
			return reconcile.Result{}, err
		}

		if len(associatedShoots) == 0 {
			log.Info("No Shoots are referencing ExposureClass, deletion accepted")

			if controllerutil.ContainsFinalizer(exposureClass, gardencorev1beta1.GardenerName) {
				log.Info("Removing finalizer")
				if err := controllerutils.RemoveFinalizers(ctx, r.Client, exposureClass, gardencorev1beta1.GardenerName); err != nil {
					return reconcile.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
				}
			}

			return reconcile.Result{}, nil
		}

		r.Recorder.Event(exposureClass, corev1.EventTypeNormal, v1beta1constants.EventResourceReferenced, fmt.Sprintf("Cannot delete ExposureClass, because it is still associated by the following Shoots: %+v", associatedShoots))
		return reconcile.Result{}, fmt.Errorf("cannot delete ExposureClass, because it is still associated by the following Shoots: %+v", associatedShoots)
	}

	if !controllerutil.ContainsFinalizer(exposureClass, gardencorev1beta1.GardenerName) {
		log.Info("Adding finalizer")
		if err := controllerutils.AddFinalizers(ctx, r.Client, exposureClass, gardencorev1beta1.GardenerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("could not add finalizer: %w", err)
		}
	}

	return reconcile.Result{}, nil
}

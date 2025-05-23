// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controllerdeployment

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	controllermanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/controllermanager/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
)

// Reconciler reconciles ControllerDeployment.
type Reconciler struct {
	Client client.Client
	Config controllermanagerconfigv1alpha1.ControllerDeploymentControllerConfiguration

	// RateLimiter allows limiting exponential backoff for testing purposes
	RateLimiter workqueue.TypedRateLimiter[reconcile.Request]
}

// Reconcile performs the main reconciliation logic.
func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	ctx, cancel := controllerutils.GetMainReconciliationContext(ctx, controllerutils.DefaultReconciliationTimeout)
	defer cancel()

	controllerDeployment := &gardencorev1.ControllerDeployment{}
	if err := r.Client.Get(ctx, client.ObjectKey{Name: req.Name}, controllerDeployment); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	if controllerDeployment.DeletionTimestamp != nil {
		if !controllerutil.ContainsFinalizer(controllerDeployment, FinalizerName) {
			return reconcile.Result{}, nil
		}

		controllerRegistrationList := &gardencorev1beta1.ControllerRegistrationList{}
		if err := r.Client.List(ctx, controllerRegistrationList); err != nil {
			return reconcile.Result{}, err
		}

		controllerRegistrations := sets.New[string]()
		for _, controllerRegistration := range controllerRegistrationList.Items {
			deployment := controllerRegistration.Spec.Deployment
			if deployment == nil {
				continue
			}
			for _, deploymentRef := range deployment.DeploymentRefs {
				if deploymentRef.Name == controllerDeployment.Name {
					controllerRegistrations.Insert(controllerRegistration.Name)
				}
			}
		}

		if controllerRegistrations.Len() > 0 {
			return reconcile.Result{}, fmt.Errorf("cannot remove finalizer of ControllerDeployment %q because still found ControllerRegistrations: %+v", controllerDeployment.Name, sets.List(controllerRegistrations))
		}

		if controllerutil.ContainsFinalizer(controllerDeployment, FinalizerName) {
			log.Info("Removing finalizer")
			if err := controllerutils.RemoveFinalizers(ctx, r.Client, controllerDeployment, FinalizerName); err != nil {
				return reconcile.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
			}
		}

		return reconcile.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(controllerDeployment, FinalizerName) {
		log.Info("Adding finalizer")
		if err := controllerutils.AddFinalizers(ctx, r.Client, controllerDeployment, FinalizerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to add finalizer: %w", err)
		}
	}

	return reconcile.Result{}, nil
}

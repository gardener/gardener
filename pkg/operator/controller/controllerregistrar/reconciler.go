// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controllerregistrar

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
)

// Reconciler adds the controllers to the manager.
type Reconciler struct {
	Manager     manager.Manager
	Controllers []Controller
}

// Controller contains a function for registering a controller.
type Controller struct {
	AddToManagerFunc func(context.Context, manager.Manager, *operatorv1alpha1.Garden) (bool, error)
	added            bool
}

// RequeueAfter is the time the request is requeued in case a controller is not added yet. Exposed for testing.
var RequeueAfter = 2 * time.Second

// Reconcile performs the controller registration.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	if r.allControllersAdded() {
		return reconcile.Result{}, nil
	}

	garden := &operatorv1alpha1.Garden{}
	if err := r.Manager.GetClient().Get(ctx, request.NamespacedName, garden); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	for i, controller := range r.Controllers {
		if !controller.added {
			done, err := controller.AddToManagerFunc(ctx, r.Manager, garden)
			if err != nil {
				return reconcile.Result{}, err
			}
			if !done {
				return reconcile.Result{RequeueAfter: RequeueAfter}, nil
			}
			r.Controllers[i].added = true
		}
	}
	return reconcile.Result{}, nil
}

func (r *Reconciler) allControllersAdded() bool {
	for _, controller := range r.Controllers {
		if !controller.added {
			return false
		}
	}
	return true
}

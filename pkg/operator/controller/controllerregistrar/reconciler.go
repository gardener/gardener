// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
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

	OperatorCancel context.CancelFunc
}

// Controller contains a function for registering a controller.
type Controller struct {
	Name             string
	AddToManagerFunc func(context.Context, manager.Manager, *operatorv1alpha1.Garden) (bool, error)
	added            bool
}

// Reconcile performs the controller registration.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	garden := &operatorv1alpha1.Garden{}
	if err := r.Manager.GetClient().Get(ctx, request.NamespacedName, garden); err != nil {
		if apierrors.IsNotFound(err) {
			// Shut down Gardener-Operator in case the garden was deleted.
			// This is a pragmatic way to deregister all controllers that depend on the existence of a garden cluster.
			log.Info("Terminating gardener-operator after garden deletion")
			r.OperatorCancel()
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	if r.allControllersAdded() {
		return reconcile.Result{}, nil
	}

	var requeueAfter time.Duration

	for i, controller := range r.Controllers {
		if !controller.added {
			if done, err := controller.AddToManagerFunc(ctx, r.Manager, garden); err != nil {
				return reconcile.Result{}, fmt.Errorf("failed adding %s controller to manager: %w", controller.Name, err)
			} else if done {
				log.Info("Successfully added controller to manager", "controllerName", controller.Name)
				r.Controllers[i].added = true
			} else {
				log.Info("Controller is not yet ready to be added to the manager", "controllerName", controller.Name)
				requeueAfter = 2 * time.Second
			}
		}
	}

	return reconcile.Result{RequeueAfter: requeueAfter}, nil
}

func (r *Reconciler) allControllersAdded() bool {
	for _, controller := range r.Controllers {
		if !controller.added {
			return false
		}
	}
	return true
}

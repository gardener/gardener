// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package ingress

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
)

// ControllerName is the name of the controller.
const ControllerName = "ingress"

// DefaultAddOptions are the default AddOptions for AddToManager.
var DefaultAddOptions = AddOptions{}

// AddOptions are options to apply when adding the local ingress controller to the manager.
type AddOptions struct {
	// Controller are the controller.Options.
	Controller controller.Options
}

// AddToManagerWithOptions adds a controller with the given Options to the given manager.
// The opts.Reconciler is being set with a newly instantiated actuator.
func AddToManagerWithOptions(ctx context.Context, mgr manager.Manager, opts AddOptions) error {
	opts.Controller.Reconciler = &reconciler{client: mgr.GetClient()}

	ctrl, err := controller.New(ControllerName, mgr, opts.Controller)
	if err != nil {
		return err
	}

	return ctrl.Watch(
		source.Kind[client.Object](mgr.GetCache(),
			&networkingv1.Ingress{},
			&handler.EnqueueRequestForObject{},
			predicate.NewPredicateFuncs(func(object client.Object) bool {
				namespace := &corev1.Namespace{}
				if err := mgr.GetClient().Get(ctx, client.ObjectKey{Name: object.GetNamespace()}, namespace); err != nil {
					ctrl.GetLogger().Error(err, "Unable to fetch namespace")
				}
				return namespace.Labels[v1beta1constants.GardenRole] == v1beta1constants.GardenRoleShoot
			}),
		),
	)
}

// AddToManager adds a controller with the default Options.
func AddToManager(ctx context.Context, mgr manager.Manager) error {
	return AddToManagerWithOptions(ctx, mgr, DefaultAddOptions)
}

// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package worker

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	extensionspredicate "github.com/gardener/gardener/extensions/pkg/predicate"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils/mapper"
)

const (
	// FinalizerName is the worker controller finalizer.
	FinalizerName = "extensions.gardener.cloud/worker"
	// ControllerName is the name of the controller.
	ControllerName = "worker"
)

// AddArgs are arguments for adding an worker controller to a manager.
type AddArgs struct {
	// Actuator is an worker actuator.
	Actuator Actuator
	// ControllerOptions are the controller options used for creating a controller.
	// The options.Reconciler is always overridden with a reconciler created from the
	// given actuator.
	ControllerOptions controller.Options
	// Predicates are the predicates to use.
	// If unset, GenerationChangedPredicate will be used.
	Predicates []predicate.Predicate
	// Type is the type of the resource considered for reconciliation.
	Type string
	// IgnoreOperationAnnotation specifies whether to ignore the operation annotation or not.
	// If the annotation is not ignored, the extension controller will only reconcile
	// with a present operation annotation typically set during a reconcile (e.g in the maintenance time) by the Gardenlet
	IgnoreOperationAnnotation bool
}

// DefaultPredicates returns the default predicates for a Worker reconciler.
func DefaultPredicates(ctx context.Context, mgr manager.Manager, ignoreOperationAnnotation bool) []predicate.Predicate {
	return extensionspredicate.DefaultControllerPredicates(ignoreOperationAnnotation, extensionspredicate.ShootNotFailedPredicate(ctx, mgr))
}

// Add creates a new Worker Controller and adds it to the Manager.
// and Start it when the Manager is Started.
func Add(ctx context.Context, mgr manager.Manager, args AddArgs) error {
	args.ControllerOptions.Reconciler = NewReconciler(mgr, args.Actuator)

	predicates := extensionspredicate.AddTypePredicate(args.Predicates, args.Type)

	ctrl, err := controller.New(ControllerName, mgr, args.ControllerOptions)
	if err != nil {
		return err
	}

	if args.IgnoreOperationAnnotation {
		if err := ctrl.Watch(
			source.Kind(mgr.GetCache(), &extensionsv1alpha1.Cluster{}),
			mapper.EnqueueRequestsFrom(ctx, mgr.GetCache(), ClusterToWorkerMapper(mgr, predicates), mapper.UpdateWithNew, ctrl.GetLogger()),
		); err != nil {
			return err
		}
	}

	return ctrl.Watch(source.Kind(mgr.GetCache(), &extensionsv1alpha1.Worker{}), &handler.EnqueueRequestForObject{}, predicates...)
}

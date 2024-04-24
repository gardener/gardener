// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package operatingsystemconfig

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	extensionspredicate "github.com/gardener/gardener/extensions/pkg/predicate"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

const (
	// FinalizerName is the name of the finalizer written by this controller.
	FinalizerName = "extensions.gardener.cloud/operatingsystemconfigs"

	// ControllerName is the name of the operating system configuration controller.
	ControllerName = "operatingsystemconfig"
)

// AddArgs are arguments for adding an operatingsystemconfig controller to a manager.
type AddArgs struct {
	// Actuator is an operatingsystemconfig actuator.
	Actuator Actuator
	// ControllerOptions are the controller options used for creating a controller.
	// The options.Reconciler is always overridden with a reconciler created from the
	// given actuator.
	ControllerOptions controller.Options
	// Predicates are the predicates to use.
	// If unset, GenerationChangedPredicate will be used.
	Predicates []predicate.Predicate
	// Types are the similar types which can be combined with a logic or,
	// of the resource considered for reconciliation.
	Types []string
}

// Add adds an operatingsystemconfig controller to the given manager using the given AddArgs.
func Add(mgr manager.Manager, args AddArgs) error {
	args.ControllerOptions.Reconciler = NewReconciler(mgr, args.Actuator)
	predicates := extensionspredicate.AddTypePredicate(args.Predicates, args.Types...)
	return add(mgr, args.ControllerOptions, predicates)
}

// DefaultPredicates returns the default predicates for an operatingsystemconfig reconciler.
func DefaultPredicates(ctx context.Context, mgr manager.Manager, ignoreOperationAnnotation bool) []predicate.Predicate {
	return extensionspredicate.DefaultControllerPredicates(ignoreOperationAnnotation, extensionspredicate.ShootNotFailedPredicate(ctx, mgr))
}

func add(mgr manager.Manager, options controller.Options, predicates []predicate.Predicate) error {
	ctrl, err := controller.New(ControllerName, mgr, options)
	if err != nil {
		return err
	}

	return ctrl.Watch(source.Kind(mgr.GetCache(), &extensionsv1alpha1.OperatingSystemConfig{}), &handler.EnqueueRequestForObject{}, predicates...)
}

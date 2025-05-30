// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package operatingsystemconfig

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	extensionspredicate "github.com/gardener/gardener/extensions/pkg/predicate"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	predicateutils "github.com/gardener/gardener/pkg/controllerutils/predicate"
)

const (
	// FinalizerName is the name of the finalizer written by this controller.
	FinalizerName = "extensions.gardener.cloud/operatingsystemconfigs"

	// ControllerName is the name of the operating system configuration controller.
	ControllerName = "operatingsystemconfig"
)

// AddArgs are arguments for adding an OperatingSystemConfig controller to a manager.
type AddArgs struct {
	// Actuator is an OperatingSystemConfig actuator.
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
	// ExtensionClass defines the extension class this extension is responsible for.
	ExtensionClass extensionsv1alpha1.ExtensionClass
}

// Add adds an operatingsystemconfig controller to the given manager using the given AddArgs.
func Add(mgr manager.Manager, args AddArgs) error {
	predicates := predicateutils.AddTypeAndClassPredicates(args.Predicates, args.ExtensionClass, args.Types...)

	return builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		WithOptions(args.ControllerOptions).
		Watches(
			&extensionsv1alpha1.OperatingSystemConfig{},
			&handler.EnqueueRequestForObject{},
			builder.WithPredicates(predicates...),
		).
		Complete(NewReconciler(mgr, args.Actuator))
}

// DefaultPredicates returns the default predicates for an operatingsystemconfig reconciler.
func DefaultPredicates(ctx context.Context, mgr manager.Manager, ignoreOperationAnnotation bool) []predicate.Predicate {
	return extensionspredicate.DefaultControllerPredicates(ignoreOperationAnnotation, extensionspredicate.ShootNotFailedPredicate(ctx, mgr))
}

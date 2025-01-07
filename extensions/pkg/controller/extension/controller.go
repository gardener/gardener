// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package extension

import (
	"context"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	extensionspredicate "github.com/gardener/gardener/extensions/pkg/predicate"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

const (
	// FinalizerPrefix is the prefix name of the finalizer written by this controller.
	FinalizerPrefix = "extensions.gardener.cloud"
)

// AddArgs are arguments for adding an Extension resources controller to a manager.
type AddArgs struct {
	// Actuator is an Extension resource actuator.
	Actuator Actuator
	// Name is the name of the controller.
	Name string
	// FinalizerSuffix is the suffix for the finalizer name.
	FinalizerSuffix string
	// ControllerOptions are the controller options used for creating a controller.
	// The options.Reconciler is always overridden with a reconciler created from the
	// given actuator.
	ControllerOptions controller.Options
	// Predicates are the predicates to use.
	Predicates []predicate.Predicate
	// Resync determines the requeue interval.
	Resync time.Duration
	// Type is the type of the resource considered for reconciliation.
	Type string
	// WatchBuilder defines additional watches on controllers that should be set up.
	WatchBuilder extensionscontroller.WatchBuilder
	// IgnoreOperationAnnotation specifies whether to ignore the operation annotation or not.
	// If the annotation is not ignored, the extension controller will only reconcile
	// with a present operation annotation typically set during a reconcile (e.g. in the maintenance time) by the Gardenlet
	IgnoreOperationAnnotation bool
	// ExtensionClass defines the extension class this extension is responsible for.
	ExtensionClass extensionsv1alpha1.ExtensionClass
}

// Add adds an Extension controller to the given manager using the given AddArgs.
func Add(mgr manager.Manager, args AddArgs) error {
	return add(mgr, args)
}

// DefaultPredicates returns the default predicates for an extension reconciler.
func DefaultPredicates(ctx context.Context, mgr manager.Manager, ignoreOperationAnnotation bool) []predicate.Predicate {
	return extensionspredicate.DefaultControllerPredicates(ignoreOperationAnnotation, extensionspredicate.ShootNotFailedPredicate(ctx, mgr))
}

func add(mgr manager.Manager, args AddArgs) error {
	predicates := extensionspredicate.AddTypeAndClassPredicates(args.Predicates, args.ExtensionClass, args.Type)

	c, err := builder.
		ControllerManagedBy(mgr).
		Named(args.Name).
		WithOptions(args.ControllerOptions).
		Watches(
			&extensionsv1alpha1.Extension{},
			&handler.EnqueueRequestForObject{},
			builder.WithPredicates(predicates...),
		).
		Build(NewReconciler(mgr, args))
	if err != nil {
		return err
	}

	if args.IgnoreOperationAnnotation {
		if err := c.Watch(source.Kind[client.Object](
			mgr.GetCache(),
			&extensionsv1alpha1.Cluster{},
			handler.EnqueueRequestsFromMapFunc(ClusterToExtensionMapper(mgr.GetClient(), predicates...)),
		)); err != nil {
			return err
		}
	}

	// Add additional watches to the controller besides the standard one.
	return args.WatchBuilder.AddToController(c)
}

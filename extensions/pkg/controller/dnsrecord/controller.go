// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package dnsrecord

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	extensionspredicate "github.com/gardener/gardener/extensions/pkg/predicate"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	predicateutils "github.com/gardener/gardener/pkg/controllerutils/predicate"
)

const (
	// FinalizerName is the dnsrecord controller finalizer.
	FinalizerName = "extensions.gardener.cloud/dnsrecord"
	// ControllerName is the name of the controller
	ControllerName = "dnsrecord"
)

// AddArgs are arguments for adding a DNSRecord controller to a manager.
type AddArgs struct {
	// Actuator is a DNSRecord actuator.
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
	// ExtensionClass defines the extension class this extension is responsible for.
	ExtensionClass extensionsv1alpha1.ExtensionClass
}

// DefaultPredicates returns the default predicates for a dnsrecord reconciler.
func DefaultPredicates(ctx context.Context, mgr manager.Manager, ignoreOperationAnnotation bool) []predicate.Predicate {
	return extensionspredicate.DefaultControllerPredicates(ignoreOperationAnnotation,
		// Special case for preconditions for the DNSRecord controller: Some DNSRecord resources are created in the
		// 'garden' namespace and don't belong to a Shoot. Most other DNSRecord resources are created in regular shoot
		// namespaces (in such cases we want to check whether the respective Shoot is failed). Consequently, we add both
		// preconditions and ensure at least one of them applies.
		predicate.Or(
			extensionspredicate.IsInGardenNamespacePredicate,
			extensionspredicate.ShootNotFailedPredicate(ctx, mgr),
		),
	)
}

// Add creates a new dnsrecord controller and adds it to the given Manager.
func Add(mgr manager.Manager, args AddArgs) error {
	predicates := predicateutils.AddTypeAndClassPredicates(args.Predicates, args.ExtensionClass, args.Type)

	c, err := builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		WithOptions(args.ControllerOptions).
		Watches(
			&extensionsv1alpha1.DNSRecord{},
			&handler.EnqueueRequestForObject{},
			builder.WithPredicates(predicates...),
		).
		Build(NewReconciler(mgr, args.Actuator))
	if err != nil {
		return err
	}

	if args.IgnoreOperationAnnotation {
		if err := c.Watch(source.Kind[client.Object](
			mgr.GetCache(),
			&extensionsv1alpha1.Cluster{},
			handler.EnqueueRequestsFromMapFunc(ClusterToDNSRecordMapper(mgr.GetClient(), predicates)),
		)); err != nil {
			return err
		}
	}

	return nil
}

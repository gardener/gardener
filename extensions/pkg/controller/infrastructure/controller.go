// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package infrastructure

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	extensionspredicate "github.com/gardener/gardener/extensions/pkg/predicate"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils/mapper"
)

const (
	// FinalizerName is the infrastructure controller finalizer.
	FinalizerName = "extensions.gardener.cloud/infrastructure"
	// ControllerName is the name of the controller.
	ControllerName = "infrastructure"
)

// AddArgs are arguments for adding an infrastructure controller to a manager.
type AddArgs struct {
	// Actuator is an infrastructure actuator.
	Actuator Actuator
	// ConfigValidator is an infrastructure config validator.
	ConfigValidator ConfigValidator
	// ControllerOptions are the controller options used for creating a controller.
	// The options.Reconciler is always overridden with a reconciler created from the
	// given actuator.
	ControllerOptions controller.Options
	// Predicates are the predicates to use.
	// If unset, GenerationChangedPredicate will be used.
	Predicates []predicate.Predicate
	// Type is the type of the resource considered for reconciliation.
	Type string
	// WatchBuilder defines additional watches on controllers that should be set up.
	WatchBuilder extensionscontroller.WatchBuilder
	// IgnoreOperationAnnotation specifies whether to ignore the operation annotation or not.
	// If the annotation is not ignored, the extension controller will only reconcile
	// with a present operation annotation typically set during a reconcile (e.g in the maintenance time) by the Gardenlet
	IgnoreOperationAnnotation bool
	// KnownCodes is a map of known error codes and their respective error check functions.
	KnownCodes map[gardencorev1beta1.ErrorCode]func(string) bool
}

// DefaultPredicates returns the default predicates for an infrastructure reconciler.
func DefaultPredicates(ctx context.Context, mgr manager.Manager, ignoreOperationAnnotation bool) []predicate.Predicate {
	return extensionspredicate.DefaultControllerPredicates(ignoreOperationAnnotation, extensionspredicate.ShootNotFailedPredicate(ctx, mgr))
}

// Add creates a new Infrastructure Controller and adds it to the Manager.
// and Start it when the Manager is Started.
func Add(ctx context.Context, mgr manager.Manager, args AddArgs) error {
	args.ControllerOptions.Reconciler = NewReconciler(mgr, args.Actuator, args.ConfigValidator, args.KnownCodes)
	return add(ctx, mgr, args)
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(ctx context.Context, mgr manager.Manager, args AddArgs) error {
	ctrl, err := controller.New(ControllerName, mgr, args.ControllerOptions)
	if err != nil {
		return err
	}

	predicates := extensionspredicate.AddTypePredicate(args.Predicates, args.Type)

	if err := ctrl.Watch(source.Kind(mgr.GetCache(), &extensionsv1alpha1.Infrastructure{}), &handler.EnqueueRequestForObject{}, predicates...); err != nil {
		return err
	}

	// do not watch cluster if respect operation annotation to prevent unwanted reconciliations in case the operation annotation
	// is already present & the extension CRD is already deleting
	if args.IgnoreOperationAnnotation {
		if err := ctrl.Watch(
			source.Kind(mgr.GetCache(), &extensionsv1alpha1.Cluster{}),
			mapper.EnqueueRequestsFrom(ctx, mgr.GetCache(), ClusterToInfrastructureMapper(mgr, predicates), mapper.UpdateWithNew, ctrl.GetLogger()),
		); err != nil {
			return err
		}
	}

	// Add additional watches to the controller besides the standard one.
	return args.WatchBuilder.AddToController(ctrl)
}

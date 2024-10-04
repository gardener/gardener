// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bastion

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	extensionspredicate "github.com/gardener/gardener/extensions/pkg/predicate"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

const (
	// FinalizerName is the bastion controller finalizer.
	FinalizerName = "extensions.gardener.cloud/bastion"
	// ControllerName is the name of the controller
	ControllerName = "bastion"
)

// AddArgs are arguments for adding a Bastion controller to a manager.
type AddArgs struct {
	// Actuator is a Bastion actuator.
	Actuator Actuator
	// ConfigValidator is a bastion config validator.
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
	// ExtensionClass defines the extension class this extension is responsible for.
	ExtensionClass extensionsv1alpha1.ExtensionClass
}

// DefaultPredicates returns the default predicates for a bastion reconciler.
func DefaultPredicates(ignoreOperationAnnotation bool) []predicate.Predicate {
	return extensionspredicate.DefaultControllerPredicates(ignoreOperationAnnotation)
}

// Add creates a new Bastion Controller and adds it to the Manager.
// and Start it when the Manager is Started.
func Add(mgr manager.Manager, args AddArgs) error {
	args.ControllerOptions.Reconciler = NewReconciler(mgr, args.Actuator, args.ConfigValidator)
	predicates := extensionspredicate.AddTypePredicate(args.Predicates, args.Type)
	predicates = append(predicates, extensionspredicate.HasClass(args.ExtensionClass))
	return add(mgr, args, predicates)
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, args AddArgs, predicates []predicate.Predicate) error {
	ctrl, err := controller.New(ControllerName, mgr, args.ControllerOptions)
	if err != nil {
		return err
	}

	return ctrl.Watch(source.Kind[client.Object](mgr.GetCache(), &extensionsv1alpha1.Bastion{}, &handler.EnqueueRequestForObject{}, predicates...))
}

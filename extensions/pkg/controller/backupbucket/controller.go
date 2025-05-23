// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package backupbucket

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/builder"
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
	// FinalizerName is the backupbucket controller finalizer.
	FinalizerName = "extensions.gardener.cloud/backupbucket"
	// ControllerName is the name of the controller
	ControllerName = "backupbucket"
)

// AddArgs are arguments for adding a BackupBucket controller to a manager.
type AddArgs struct {
	// Actuator is a BackupBucket actuator.
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
	// with a present operation annotation typically set during a reconcile (e.g. in the maintenance time) by the
	// gardenlet.
	IgnoreOperationAnnotation bool
	// ExtensionClass defines the extension class this extension is responsible for.
	ExtensionClass extensionsv1alpha1.ExtensionClass
}

// DefaultPredicates returns the default predicates for a BackupBucket reconciler.
func DefaultPredicates(ignoreOperationAnnotation bool) []predicate.Predicate {
	return extensionspredicate.DefaultControllerPredicates(ignoreOperationAnnotation)
}

// Add creates a new BackupBucket Controller and adds it to the Manager.
// and Start it when the Manager is Started.
func Add(mgr manager.Manager, args AddArgs) error {
	predicates := extensionspredicate.AddTypeAndClassPredicates(args.Predicates, args.ExtensionClass, args.Type)
	return add(mgr, args, predicates)
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, args AddArgs, predicates []predicate.Predicate) error {
	c, err := builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		WithOptions(args.ControllerOptions).
		Watches(
			&extensionsv1alpha1.BackupBucket{},
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
			&metav1.PartialObjectMetadata{TypeMeta: metav1.TypeMeta{Kind: "Secret", APIVersion: "v1"}},
			handler.EnqueueRequestsFromMapFunc(SecretToBackupBucketMapper(mgr.GetClient(), predicates)),
		)); err != nil {
			return err
		}
	}

	return nil
}

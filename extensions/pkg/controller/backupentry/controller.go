// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package backupentry

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
	// FinalizerName is the backupentry controller finalizer.
	FinalizerName = "extensions.gardener.cloud/backupentry"
	// ControllerName is the name of the controller
	ControllerName = "backupentry"
)

// AddArgs are arguments for adding a BackupEntry controller to a manager.
type AddArgs struct {
	// Actuator is a BackupEntry actuator.
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
	// with a present operation annotation typically set during a reconcile (e.g. in the maintenance time) by the Gardenlet
	IgnoreOperationAnnotation bool
	// ExtensionClass defines the extension class this extension is responsible for.
	ExtensionClass extensionsv1alpha1.ExtensionClass
}

// DefaultPredicates returns the default predicates for a controlplane reconciler.
func DefaultPredicates(ignoreOperationAnnotation bool) []predicate.Predicate {
	return extensionspredicate.DefaultControllerPredicates(ignoreOperationAnnotation)
}

// Add creates a new BackupEntry Controller and adds it to the Manager.
// and Start it when the Manager is Started.
func Add(ctx context.Context, mgr manager.Manager, args AddArgs) error {
	args.ControllerOptions.Reconciler = NewReconciler(mgr, args.Actuator)
	predicates := extensionspredicate.AddTypePredicate(args.Predicates, args.Type)
	predicates = append(predicates, extensionspredicate.HasClass(args.ExtensionClass))
	return add(ctx, mgr, args, predicates)
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(ctx context.Context, mgr manager.Manager, args AddArgs, predicates []predicate.Predicate) error {
	ctrl, err := controller.New(ControllerName, mgr, args.ControllerOptions)
	if err != nil {
		return err
	}

	if args.IgnoreOperationAnnotation {
		if err := ctrl.Watch(
			source.Kind[client.Object](mgr.GetCache(),
				&corev1.Namespace{},
				mapper.EnqueueRequestsFrom(ctx, mgr.GetCache(), NamespaceToBackupEntryMapper(predicates), mapper.UpdateWithNew, ctrl.GetLogger())),
		); err != nil {
			return err
		}

		if err := ctrl.Watch(
			source.Kind[client.Object](mgr.GetCache(), &metav1.PartialObjectMetadata{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Secret",
					APIVersion: "v1",
				},
			},
				mapper.EnqueueRequestsFrom(ctx, mgr.GetCache(), SecretToBackupEntryMapper(predicates), mapper.UpdateWithNew, ctrl.GetLogger())),
		); err != nil {
			return err
		}
	}

	return ctrl.Watch(source.Kind[client.Object](mgr.GetCache(), &extensionsv1alpha1.BackupEntry{}, &handler.EnqueueRequestForObject{}, predicates...))
}

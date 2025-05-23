// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package networkpolicy

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/provider-local/local"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

// ControllerName is the name of the controller.
const ControllerName = "networkpolicy"

// AddToManager adds a controller to the given manager.
func AddToManager(_ context.Context, mgr manager.Manager) error {
	return (&Reconciler{}).AddToManager(mgr)
}

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(mgr manager.Manager) error {
	if r.Client == nil {
		r.Client = mgr.GetClient()
	}

	return builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		For(&corev1.Namespace{}, builder.WithPredicates(IsShootNamespace(), IsShootProviderLocal())).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 5,
		}).
		Complete(r)
}

// IsShootNamespace returns a predicate that returns true if the namespace is a shoot namespace.
func IsShootNamespace() predicate.Predicate {
	return predicate.NewPredicateFuncs(func(obj client.Object) bool {
		return gardenerutils.IsShootNamespace(obj.GetName())
	})
}

// IsShootProviderLocal returns a predicate that returns true if the provider of the shoot is of type "local".
func IsShootProviderLocal() predicate.Predicate {
	return predicate.NewPredicateFuncs(func(obj client.Object) bool {
		value, hasLabel := obj.GetLabels()[v1beta1constants.LabelShootProvider]

		return hasLabel && value == local.Type
	})
}

// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package networkpolicy

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/extensions"
	"github.com/gardener/gardener/pkg/provider-local/local"
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
		WatchesRawSource(source.Kind(
			mgr.GetCache(),
			&extensionsv1alpha1.Cluster{},
			&handler.TypedEnqueueRequestForObject[*extensionsv1alpha1.Cluster]{},
			exposureClassChanged(),
		)).
		For(&corev1.Namespace{}, builder.WithPredicates(IsShootNamespace(), IsShootProviderLocal())).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 5,
		}).
		Complete(r)
}

func exposureClassChanged() predicate.TypedFuncs[*extensionsv1alpha1.Cluster] {
	return predicate.TypedFuncs[*extensionsv1alpha1.Cluster]{
		CreateFunc: func(event.TypedCreateEvent[*extensionsv1alpha1.Cluster]) bool {
			return true
		},
		UpdateFunc: func(e event.TypedUpdateEvent[*extensionsv1alpha1.Cluster]) bool {
			oldShoot, err := extensions.ShootFromCluster(e.ObjectOld)
			if err != nil {
				return false
			}
			newShoot, err := extensions.ShootFromCluster(e.ObjectNew)
			if err != nil {
				return false
			}
			// enqueue if exposureclasses are not the same
			return !equality.Semantic.DeepEqual(oldShoot.Spec.ExposureClassName, newShoot.Spec.ExposureClassName)
		},
		DeleteFunc: func(event.TypedDeleteEvent[*extensionsv1alpha1.Cluster]) bool {
			return false
		},
		GenericFunc: func(event.TypedGenericEvent[*extensionsv1alpha1.Cluster]) bool {
			return false
		},
	}
}

// IsShootNamespace returns a predicate that returns true if the namespace is a shoot namespace.
func IsShootNamespace() predicate.Predicate {
	return predicate.NewPredicateFuncs(func(obj client.Object) bool {
		return obj.GetLabels()[v1beta1constants.GardenRole] == v1beta1constants.GardenRoleShoot
	})
}

// IsShootProviderLocal returns a predicate that returns true if the provider of the shoot is of type "local".
func IsShootProviderLocal() predicate.Predicate {
	return predicate.NewPredicateFuncs(func(obj client.Object) bool {
		value, hasLabel := obj.GetLabels()[v1beta1constants.LabelShootProvider]

		return hasLabel && value == local.Type
	})
}

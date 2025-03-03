// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package runtime

import (
	"context"
	"fmt"
	"slices"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/clock"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

// ControllerName is the name of this controller.
const ControllerName = "extension-required-runtime"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(mgr manager.Manager) error {
	if r.Client == nil {
		r.Client = mgr.GetClient()
	}

	r.clock = clock.RealClock{}

	return builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		For(
			&operatorv1alpha1.Extension{},
			builder.WithPredicates(predicate.GenerationChangedPredicate{}),
		).
		Watches(
			&operatorv1alpha1.Garden{},
			handler.EnqueueRequestsFromMapFunc(r.MapGardenToExtensions(mgr.GetLogger().WithValues("controller", ControllerName))),
			builder.WithPredicates(predicate.GenerationChangedPredicate{}),
		).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: ptr.Deref(r.Config.ConcurrentSyncs, 0),
		}).
		Complete(r)
}

// MapGardenToExtensions returns a mapping function that maps a given garden resource to all related extensions.
func (r *Reconciler) MapGardenToExtensions(log logr.Logger) handler.MapFunc {
	return func(ctx context.Context, obj client.Object) []reconcile.Request {
		garden, ok := obj.(*operatorv1alpha1.Garden)
		if !ok {
			log.Error(fmt.Errorf("expected Garden but got %#v", obj), "Unable to convert to Garden")
			return nil
		}

		extensionList := &operatorv1alpha1.ExtensionList{}
		if err := r.Client.List(ctx, extensionList); err != nil {
			log.Error(err, "Failed to list extensions")
			return nil
		}

		var (
			requests           []reconcile.Request
			requiredExtensions = gardenerutils.ComputeRequiredExtensionsForGarden(garden)
		)

		for _, extension := range extensionList.Items {
			if slices.ContainsFunc(extension.Spec.Resources, func(resource gardencorev1beta1.ControllerResource) bool {
				return requiredExtensions.Has(gardenerutils.ExtensionsID(resource.Kind, resource.Type))
			}) {
				requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: extension.Name, Namespace: extension.Namespace}})
			}
		}

		return requests
	}
}

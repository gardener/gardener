// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package care

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/clock"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	predicateutils "github.com/gardener/gardener/pkg/controllerutils/predicate"
	"github.com/gardener/gardener/pkg/gardenlet/controller/controllerinstallation/utils"
)

// ControllerName is the name of this controller.
const ControllerName = "controllerinstallation-care"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(mgr manager.Manager, gardenCluster, seedCluster cluster.Cluster) error {
	if r.GardenClient == nil {
		r.GardenClient = gardenCluster.GetClient()
	}
	if r.SeedClient == nil {
		r.SeedClient = seedCluster.GetClient()
	}
	if r.Clock == nil {
		r.Clock = clock.RealClock{}
	}
	if r.GardenNamespace == "" {
		r.GardenNamespace = v1beta1constants.GardenNamespace
	}

	return builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: ptr.Deref(r.Config.ConcurrentSyncs, 0),
			// if going into exponential backoff, wait at most the configured sync period
			RateLimiter: workqueue.NewTypedWithMaxWaitRateLimiter(workqueue.DefaultTypedControllerRateLimiter[reconcile.Request](), r.Config.SyncPeriod.Duration),
		}).
		WatchesRawSource(source.Kind[client.Object](
			gardenCluster.GetCache(),
			&gardencorev1beta1.ControllerInstallation{},
			&handler.EnqueueRequestForObject{},
			predicateutils.ForEventTypes(predicateutils.Create),
		)).
		WatchesRawSource(source.Kind[client.Object](
			seedCluster.GetCache(),
			&resourcesv1alpha1.ManagedResource{},
			handler.EnqueueRequestsFromMapFunc(r.MapManagedResourceToControllerInstallation),
			r.IsExtensionDeployment(),
			predicateutils.ManagedResourceConditionsChanged(),
		)).
		Complete(r)
}

// IsExtensionDeployment returns a predicate which evaluates to true in case the object is in the garden namespace and
// the 'controllerinstallation-name' label is present.
func (r *Reconciler) IsExtensionDeployment() predicate.Predicate {
	return predicate.NewPredicateFuncs(func(obj client.Object) bool {
		return obj.GetNamespace() == v1beta1constants.GardenNamespace &&
			obj.GetLabels()[utils.LabelKeyControllerInstallationName] != ""
	})
}

// MapManagedResourceToControllerInstallation is a handler.MapFunc for mapping a ManagedResource to the owning
// ControllerInstallation.
func (r *Reconciler) MapManagedResourceToControllerInstallation(_ context.Context, obj client.Object) []reconcile.Request {
	managedResource, ok := obj.(*resourcesv1alpha1.ManagedResource)
	if !ok {
		return nil
	}

	controllerInstallationName, ok := managedResource.Labels[utils.LabelKeyControllerInstallationName]
	if !ok || len(controllerInstallationName) == 0 {
		return nil
	}

	return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: controllerInstallationName}}}
}

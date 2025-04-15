// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package care

import (
	"context"
	"fmt"

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
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	predicateutils "github.com/gardener/gardener/pkg/controllerutils/predicate"
	"github.com/gardener/gardener/pkg/operator/mapper"
	"github.com/gardener/gardener/pkg/operator/predicate"
	"github.com/gardener/gardener/pkg/utils/gardener/operator"
)

// ControllerName is the name of this controller.
const ControllerName = "extension-care"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(mgr manager.Manager, virtualCluster cluster.Cluster) error {
	if r.RuntimeClient == nil {
		r.RuntimeClient = mgr.GetClient()
	}
	if r.Clock == nil {
		r.Clock = clock.RealClock{}
	}
	if r.GardenNamespace == "" {
		r.GardenNamespace = v1beta1constants.GardenNamespace
	}
	if virtualCluster == nil {
		return fmt.Errorf("virtualCluster must not be nil")
	}
	r.VirtualClient = virtualCluster.GetClient()

	return builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: ptr.Deref(r.Config.Controllers.ExtensionCare.ConcurrentSyncs, 0),
			// if going into exponential backoff, wait at most the configured sync period
			RateLimiter: workqueue.NewTypedWithMaxWaitRateLimiter[reconcile.Request](
				workqueue.DefaultTypedControllerRateLimiter[reconcile.Request](),
				r.Config.Controllers.ExtensionCare.SyncPeriod.Duration,
			),
		}).
		Watches(
			&operatorv1alpha1.Extension{},
			&handler.EnqueueRequestForObject{},
			builder.WithPredicates(predicate.ExtensionRequirementsChanged()),
		).
		Watches(
			&resourcesv1alpha1.ManagedResource{},
			handler.EnqueueRequestsFromMapFunc(r.MapManagedResourceToExtension),
			builder.WithPredicates(predicateutils.ManagedResourceConditionsChanged()),
		).
		WatchesRawSource(
			source.Kind[client.Object](
				virtualCluster.GetCache(),
				&v1beta1.ControllerInstallation{},
				handler.EnqueueRequestsFromMapFunc(mapper.MapControllerInstallationToExtension(r.RuntimeClient, mgr.GetLogger().WithValues("controller", ControllerName))),
			)).
		Complete(r)
}

// MapManagedResourceToExtension is a handler.MapFunc for mapping a ManagedResource to the owning Extension.
func (r *Reconciler) MapManagedResourceToExtension(_ context.Context, obj client.Object) []reconcile.Request {
	managedResource, ok := obj.(*resourcesv1alpha1.ManagedResource)
	if !ok {
		return nil
	}

	if managedResource.Namespace != r.GardenNamespace {
		return nil
	}

	if extensionName, ok := operator.ExtensionForManagedResourceName(managedResource.Name); ok {
		return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: extensionName}}}
	}

	return nil
}

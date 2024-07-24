// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package bastion

import (
	"context"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/types"
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

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	operationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils/mapper"
	predicateutils "github.com/gardener/gardener/pkg/controllerutils/predicate"
	"github.com/gardener/gardener/pkg/extensions"
)

// ControllerName is the name of this controller.
const ControllerName = "bastion"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(ctx context.Context, mgr manager.Manager, gardenCluster, seedCluster cluster.Cluster) error {
	if r.GardenClient == nil {
		r.GardenClient = gardenCluster.GetClient()
	}
	if r.SeedClient == nil {
		r.SeedClient = seedCluster.GetClient()
	}
	if r.Clock == nil {
		r.Clock = clock.RealClock{}
	}

	c, err := builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: ptr.Deref(r.Config.ConcurrentSyncs, 0),
			RateLimiter:             r.RateLimiter,
		}).
		WatchesRawSource(
			source.Kind(gardenCluster.GetCache(),
				&operationsv1alpha1.Bastion{},
				&handler.EnqueueRequestForObject{},
				builder.WithPredicates(predicate.GenerationChangedPredicate{})),
		).
		Build(r)
	if err != nil {
		return err
	}

	return c.Watch(
		source.Kind(seedCluster.GetCache(),
			&extensionsv1alpha1.Bastion{},
			mapper.EnqueueRequestsFrom(ctx, mgr.GetCache(), mapper.MapFunc(r.MapExtensionsBastionToOperationsBastion), mapper.UpdateWithNew, c.GetLogger()),
			predicateutils.LastOperationChanged(predicateutils.GetExtensionLastOperation)),
	)
}

// MapExtensionsBastionToOperationsBastion  is a mapper.MapFunc for mapping extensions Bastion in the seed cluster to operations Bastion in the project namespace.
func (r *Reconciler) MapExtensionsBastionToOperationsBastion(ctx context.Context, log logr.Logger, _ client.Reader, obj client.Object) []reconcile.Request {
	shoot, err := extensions.GetShoot(ctx, r.SeedClient, obj.GetNamespace())
	if err != nil {
		log.Error(err, "Failed to get shoot from cluster", "shootTechnicalID", obj.GetNamespace())
		return nil
	}

	if shoot == nil {
		log.Info("Shoot is missing in cluster resource", "clusterName", obj.GetNamespace())
		return nil
	}

	return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: obj.GetName(), Namespace: shoot.Namespace}}}
}

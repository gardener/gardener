// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package backupbucket

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

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils/mapper"
	predicateutils "github.com/gardener/gardener/pkg/controllerutils/predicate"
)

// ControllerName is the name of this controller.
const ControllerName = "backupbucket"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(ctx context.Context, mgr manager.Manager, gardenCluster cluster.Cluster, seedCluster cluster.Cluster) error {
	if r.GardenClient == nil {
		r.GardenClient = gardenCluster.GetClient()
	}
	if r.SeedClient == nil {
		r.SeedClient = seedCluster.GetClient()
	}
	if r.Clock == nil {
		r.Clock = clock.RealClock{}
	}
	if r.Recorder == nil {
		r.Recorder = gardenCluster.GetEventRecorderFor(ControllerName + "-controller")
	}
	if r.GardenNamespace == "" {
		r.GardenNamespace = v1beta1constants.GardenNamespace
	}

	c, err := builder.
		ControllerManagedBy(mgr).
		Named(ControllerName).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: ptr.Deref(r.Config.ConcurrentSyncs, 0),
			RateLimiter:             r.RateLimiter,
		}).
		WatchesRawSource(
			source.Kind[client.Object](gardenCluster.GetCache(),
				&gardencorev1beta1.BackupBucket{},
				&handler.EnqueueRequestForObject{},
				builder.WithPredicates(
					&predicate.GenerationChangedPredicate{},
					r.SeedNamePredicate(),
				)),
		).
		Build(r)
	if err != nil {
		return err
	}

	return c.Watch(
		source.Kind[client.Object](seedCluster.GetCache(),
			&extensionsv1alpha1.BackupBucket{},
			mapper.EnqueueRequestsFrom(ctx, mgr.GetCache(), mapper.MapFunc(r.MapExtensionBackupBucketToCoreBackupBucket), mapper.UpdateWithNew, c.GetLogger()),
			predicateutils.LastOperationChanged(predicateutils.GetExtensionLastOperation),
		))
}

// SeedNamePredicate returns a predicate which returns true when the object belongs to this seed.
func (r *Reconciler) SeedNamePredicate() predicate.Predicate {
	return predicate.NewPredicateFuncs(func(obj client.Object) bool {
		backupBucket, ok := obj.(*gardencorev1beta1.BackupBucket)
		if !ok {
			return false
		}
		return ptr.Deref(backupBucket.Spec.SeedName, "") == r.SeedName
	})
}

// MapExtensionBackupBucketToCoreBackupBucket is a mapper.MapFunc for mapping a extensions.gardener.cloud/v1alpha1.BackupBucket to the owning
// core.gardener.cloud/v1beta1.BackupBucket.
func (r *Reconciler) MapExtensionBackupBucketToCoreBackupBucket(_ context.Context, _ logr.Logger, _ client.Reader, obj client.Object) []reconcile.Request {
	return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: obj.GetName()}}}
}

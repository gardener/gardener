// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package backupbucket

import (
	"context"
	"reflect"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/clock"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
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
			MaxConcurrentReconciles: pointer.IntDeref(r.Config.ConcurrentSyncs, 0),
			RateLimiter:             r.RateLimiter,
		}).
		Watches(
			source.NewKindWithCache(&gardencorev1beta1.BackupBucket{}, gardenCluster.GetCache()),
			controllerutils.EnqueueCreateEventsOncePer24hDuration(r.Clock),
			builder.WithPredicates(
				&predicate.GenerationChangedPredicate{},
				r.SeedNamePredicate(),
			),
		).
		Build(r)
	if err != nil {
		return err
	}

	if err := c.Watch(
		source.NewKindWithCache(&extensionsv1alpha1.BackupBucket{}, seedCluster.GetCache()),
		mapper.EnqueueRequestsFrom(ctx, mgr.GetCache(), mapper.MapFunc(r.MapExtensionBackupBucketToCoreBackupBucket), mapper.UpdateWithNew, c.GetLogger()),
		predicateutils.LastOperationChanged(predicateutils.GetExtensionLastOperation),
	); err != nil {
		return err
	}

	return c.Watch(
		source.NewKindWithCache(&corev1.Secret{}, gardenCluster.GetCache()),
		mapper.EnqueueRequestsFrom(ctx, mgr.GetCache(), mapper.MapFunc(r.MapSecretsToBackupBucket), mapper.UpdateWithOldAndNew, c.GetLogger()),
		SecretPredicate(),
	)
}

// SeedNamePredicate returns a predicate which returns true when the object belongs to this seed.
func (r *Reconciler) SeedNamePredicate() predicate.Predicate {
	return predicate.NewPredicateFuncs(func(obj client.Object) bool {
		backupBucket, ok := obj.(*gardencorev1beta1.BackupBucket)
		if !ok {
			return false
		}
		return pointer.StringDeref(backupBucket.Spec.SeedName, "") == r.SeedName
	})
}

// SecretPredicate returns a predicate which returns true when the secret has gardener.cloud/role label backup-secret and the secret data has changed.
func SecretPredicate() predicate.Predicate {
	return predicate.Funcs{
		// We don't want to requeue the secret on controller restart
		CreateFunc: func(e event.CreateEvent) bool {
			return false
		},
		UpdateFunc: func(e event.UpdateEvent) bool {
			secret, ok := e.ObjectNew.(*corev1.Secret)
			if !ok {
				return false
			}

			if secret.Labels[v1beta1constants.GardenRole] != v1beta1constants.GardenRoleBackupSecret {
				return false
			}

			oldSecret, ok := e.ObjectOld.(*corev1.Secret)
			if !ok {
				return false
			}

			return !reflect.DeepEqual(oldSecret.Data, secret.Data)
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return false
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return false
		},
	}
}

// MapExtensionBackupBucketToCoreBackupBucket is a mapper.MapFunc for mapping a extensions.gardener.cloud/v1alpha1.BackupBucket to the owning
// core.gardener.cloud/v1beta1.BackupBucket.
func (r *Reconciler) MapExtensionBackupBucketToCoreBackupBucket(_ context.Context, _ logr.Logger, _ client.Reader, obj client.Object) []reconcile.Request {
	return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: obj.GetName()}}}
}

// MapSecretsToBackupBucket is a mapper.MapFunc for mapping a Secret to the BackupBuckets referencing it.
func (r *Reconciler) MapSecretsToBackupBucket(ctx context.Context, log logr.Logger, _ client.Reader, obj client.Object) []reconcile.Request {
	secret, ok := obj.(*corev1.Secret)
	if !ok {
		return nil
	}

	backupBucketList := &gardencorev1beta1.BackupBucketList{}
	if err := r.GardenClient.List(ctx, backupBucketList, client.MatchingFields{gardencore.BackupBucketSeedName: r.SeedName}); err != nil {
		log.Error(err, "Failed to list backupbuckets")
		return nil
	}

	list := mapper.ObjectListToRequests(backupBucketList, func(object client.Object) bool {
		backupBucket, ok := object.(*gardencorev1beta1.BackupBucket)
		if !ok {
			return false
		}

		// The GCM secret controller only copies the secrets in the garden namespace to the seed namespace, so only
		// requeue the BackupBucket if the secret referenced is in the garden namespace
		return backupBucket.Spec.SecretRef.Name == secret.Name &&
			backupBucket.Spec.SecretRef.Namespace == v1beta1constants.GardenNamespace
	})

	return list
}

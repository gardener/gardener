// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package backupentry

import (
	"context"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/clock"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/controllerutils/mapper"
	predicateutils "github.com/gardener/gardener/pkg/controllerutils/predicate"
	"github.com/gardener/gardener/pkg/extensions"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// ControllerName is the name of this controller.
const ControllerName = "backupentry"

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
			source.NewKindWithCache(&gardencorev1beta1.BackupEntry{}, gardenCluster.GetCache()),
			controllerutils.EnqueueCreateEventsOncePer24hDuration(r.Clock),
			builder.WithPredicates(
				&predicate.GenerationChangedPredicate{},
				predicateutils.SeedNamePredicate(r.SeedName, gardenerutils.GetBackupEntrySeedNames),
			),
		).
		Build(r)
	if err != nil {
		return err
	}

	return c.Watch(
		source.NewKindWithCache(&extensionsv1alpha1.BackupEntry{}, seedCluster.GetCache()),
		mapper.EnqueueRequestsFrom(mapper.MapFunc(r.MapExtensionBackupEntryToCoreBackupEntry), mapper.UpdateWithNew, c.GetLogger()),
		predicateutils.ExtensionStatusChanged(),
	)
}

// MapExtensionBackupEntryToCoreBackupEntry is a mapper.MapFunc for mapping a extensions.gardener.cloud/v1alpha1.BackupEntry to the owning
// core.gardener.cloud/v1beta1.BackupEntry.
func (r *Reconciler) MapExtensionBackupEntryToCoreBackupEntry(ctx context.Context, log logr.Logger, _ client.Reader, obj client.Object) []reconcile.Request {
	if obj.GetDeletionTimestamp() != nil {
		return nil
	}

	shootTechnicalID, _ := gardenerutils.ExtractShootDetailsFromBackupEntryName(obj.GetName())
	if shootTechnicalID == "" {
		return nil
	}

	shoot, err := extensions.GetShoot(ctx, r.SeedClient, shootTechnicalID)
	if err != nil {
		log.Error(err, "Failed to get shoot from cluster", "shootTechnicalID", shootTechnicalID)
		return nil
	}
	if shoot == nil {
		log.Info("Shoot is missing in cluster resource", "cluster", kubernetesutils.Key(shootTechnicalID))
		return nil
	}

	return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: obj.GetName(), Namespace: shoot.Namespace}}}
}

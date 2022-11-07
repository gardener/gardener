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
	"strings"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
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
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
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
	if r.Recorder == nil {
		r.Recorder = gardenCluster.GetEventRecorderFor(ControllerName + "-controller")
	}
	if r.GardenNamespace == "" {
		r.GardenNamespace = v1beta1constants.GardenNamespace
	}

	// It's not possible to overwrite the event handler when using the controller builder. Hence, we have to build up
	// the controller manually.
	c, err := controller.New(
		ControllerName,
		mgr,
		controller.Options{
			Reconciler:              r,
			MaxConcurrentReconciles: pointer.IntDeref(r.Config.ConcurrentSyncs, 0),
			RecoverPanic:            true,
			// if going into exponential backoff, wait at most the configured sync period
			RateLimiter: r.RateLimiter,
		},
	)
	if err != nil {
		return err
	}

	if err := c.Watch(
		source.NewKindWithCache(&gardencorev1beta1.BackupEntry{}, gardenCluster.GetCache()),
		controllerutils.EnqueueCreateEventsOncePer24hDuration(r.Clock),
		&predicate.GenerationChangedPredicate{},
		r.SeedNamePredicate(),
	); err != nil {
		return err
	}

	return c.Watch(
		source.NewKindWithCache(&extensionsv1alpha1.BackupEntry{}, seedCluster.GetCache()),
		mapper.EnqueueRequestsFrom(mapper.MapFunc(r.MapExtensionBackupEntryToCoreBackupEntry), mapper.UpdateWithNew, c.GetLogger()),
		predicateutils.ExtensionStatusChanged(),
	)
}

// SeedNamePredicate returns a predicate which returns true when the object belongs to this seed.
func (r *Reconciler) SeedNamePredicate() predicate.Predicate {
	return predicate.NewPredicateFuncs(func(obj client.Object) bool {
		backupEntry, ok := obj.(*gardencorev1beta1.BackupEntry)
		if !ok {
			return false
		}
		if backupEntry.Spec.SeedName == nil {
			return false
		}
		if backupEntry.Status.SeedName == nil || *backupEntry.Spec.SeedName == *backupEntry.Status.SeedName {
			return *backupEntry.Spec.SeedName == r.SeedName
		}

		return *backupEntry.Status.SeedName == r.SeedName
	})
}

// MapExtensionBackupEntryToCoreBackupEntry is a mapper.MapFunc for mapping a extensions.gardener.cloud/v1alpha1.BackupEntry to the owning
// core.gardener.cloud/v1beta1.BackupEntry.
func (r *Reconciler) MapExtensionBackupEntryToCoreBackupEntry(ctx context.Context, log logr.Logger, _ client.Reader, obj client.Object) []reconcile.Request {
	extensionBackupEntry, ok := obj.(*extensionsv1alpha1.BackupEntry)
	if !ok {
		return nil
	}

	shootTechnicalID, _ := gutil.ExtractShootDetailsFromBackupEntryName(extensionBackupEntry.Name)
	if shootTechnicalID == "" {
		return nil
	}

	namespaceName := getProjectNamespaceFromTechincalId(ctx, r.GardenClient, shootTechnicalID)
	if namespaceName == "" {
		return nil
	}

	return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: obj.GetName(), Namespace: namespaceName}}}
}

// TODO: Reuse function from https://github.com/gardener/gardener/pull/6839
func getProjectNamespaceFromTechincalId(ctx context.Context, reader client.Reader, shootTechnicalID string) string {
	var (
		tokens      = strings.Split(shootTechnicalID, "--")
		projectName = tokens[len(tokens)-2]

		project = &gardencorev1beta1.Project{}
	)

	if err := reader.Get(ctx, kutil.Key(projectName), project); err != nil {
		return ""
	}
	return *project.Spec.Namespace
}

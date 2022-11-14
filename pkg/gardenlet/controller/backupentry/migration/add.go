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

package migration

import (
	"context"

	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	contextutil "github.com/gardener/gardener/pkg/utils/context"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// ControllerName is the name of this controller.
const ControllerName = "backupentry-migration"

// AddToManager adds Reconciler to the given manager.
func (r *Reconciler) AddToManager(mgr manager.Manager, gardenCluster cluster.Cluster) error {
	if r.GardenClient == nil {
		r.GardenClient = gardenCluster.GetClient()
	}

	// It's not possible to overwrite the event handler when using the controller builder. Hence, we have to build up
	// the controller manually.
	c, err := controller.New(
		ControllerName,
		mgr,
		controller.Options{
			Reconciler:              r,
			MaxConcurrentReconciles: pointer.IntDeref(r.Config.Controllers.BackupEntryMigration.ConcurrentSyncs, 0),
			RecoverPanic:            true,
		},
	)
	if err != nil {
		return err
	}

	return c.Watch(
		source.NewKindWithCache(&gardencorev1beta1.BackupEntry{}, gardenCluster.GetCache()),
		&handler.EnqueueRequestForObject{},
		&predicate.GenerationChangedPredicate{},
		r.IsBeingMigratedPredicate(),
	)
}

// IsBeingMigratedPredicate returns a predicate which returns true for backup entries that are being migrated to a different seed.
func (r *Reconciler) IsBeingMigratedPredicate() predicate.Predicate {
	return &isBeingMigratedPredicate{
		reader:   r.GardenClient,
		seedName: r.Config.SeedConfig.Name,
	}
}

type isBeingMigratedPredicate struct {
	ctx      context.Context
	reader   client.Reader
	seedName string
}

func (p *isBeingMigratedPredicate) InjectStopChannel(stopChan <-chan struct{}) error {
	p.ctx = contextutil.FromStopChannel(stopChan)
	return nil
}

func (p *isBeingMigratedPredicate) Create(e event.CreateEvent) bool {
	return p.isBeingMigratedToSeed(e.Object)
}
func (p *isBeingMigratedPredicate) Update(e event.UpdateEvent) bool {
	return p.isBeingMigratedToSeed(e.ObjectNew)
}
func (p *isBeingMigratedPredicate) Delete(e event.DeleteEvent) bool {
	return p.isBeingMigratedToSeed(e.Object)
}
func (p *isBeingMigratedPredicate) Generic(e event.GenericEvent) bool {
	return p.isBeingMigratedToSeed(e.Object)
}

func (p *isBeingMigratedPredicate) isBeingMigratedToSeed(obj client.Object) bool {
	backupEntry, ok := obj.(*gardencorev1beta1.BackupEntry)
	if !ok {
		return false
	}
	if backupEntry.Spec.SeedName != nil && backupEntry.Status.SeedName != nil && *backupEntry.Spec.SeedName != *backupEntry.Status.SeedName && *backupEntry.Spec.SeedName == p.seedName {
		seed := &gardencorev1beta1.Seed{}
		if err := p.reader.Get(p.ctx, kutil.Key(*backupEntry.Status.SeedName), seed); err != nil {
			return false
		}
		return v1beta1helper.SeedSettingOwnerChecksEnabled(seed.Spec.Settings)
	}
	return false
}

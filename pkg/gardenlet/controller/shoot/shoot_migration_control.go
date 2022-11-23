// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package shoot

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	confighelper "github.com/gardener/gardener/pkg/gardenlet/apis/config/helper"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
)

const migrationReconcilerName = "migration"

func (c *Controller) shootMigrationAdd(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		return
	}
	c.shootMigrationQueue.Add(key)
}

func (c *Controller) shootMigrationUpdate(oldObj, newObj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(newObj)
	if err != nil {
		return
	}

	shoot, ok := newObj.(*gardencorev1beta1.Shoot)
	if !ok {
		return
	}
	if shoot.Generation == shoot.Status.ObservedGeneration {
		return
	}

	c.shootMigrationQueue.Add(key)
}

func (c *Controller) shootMigrationDelete(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		return
	}
	c.shootMigrationQueue.Add(key)
}

// NewMigrationReconciler returns an implementation of reconcile.Reconciler that forces the shoot's restoration
// to this seed during control plane migration if the preparation for migration in the source seed is not finished
// after a certain grace period and is considered unlikely to succeed ("bad case" scenario).
func NewMigrationReconciler(gardenClient client.Client, config *config.GardenletConfiguration, clock clock.Clock) reconcile.Reconciler {
	return &migrationReconciler{
		gardenClient: gardenClient,
		config:       config,
		clock:        clock,
	}
}

type migrationReconciler struct {
	gardenClient client.Client
	config       *config.GardenletConfiguration
	clock        clock.Clock
}

func (r *migrationReconciler) Reconcile(ctx context.Context, req reconcile.Request) (result reconcile.Result, err error) {
	log := logf.FromContext(ctx)

	shoot := &gardencorev1beta1.Shoot{}
	if err := r.gardenClient.Get(ctx, req.NamespacedName, shoot); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	// If the shoot is being deleted or no longer being migrated to this seed, clear the migration start time
	if shoot.DeletionTimestamp != nil || !controllerutils.ShootIsBeingMigratedToSeed(ctx, r.gardenClient, shoot, confighelper.SeedNameFromSeedConfig(r.config.SeedConfig)) {
		log.V(1).Info("Clearing migration start time")
		if err := setMigrationStartTime(ctx, r.gardenClient, shoot, nil); err != nil {
			return reconcile.Result{}, fmt.Errorf("could not clear migration start time: %w", err)
		}

		// Return without requeue as the shoot is no longer being migrated (we should not force restore)
		return reconcile.Result{}, nil
	}

	// Set the migration start time if needed
	if shoot.Status.MigrationStartTime == nil {
		log.V(1).Info("Setting migration start time")
		if err := setMigrationStartTime(ctx, r.gardenClient, shoot, &metav1.Time{Time: r.clock.Now().UTC()}); err != nil {
			return reconcile.Result{}, fmt.Errorf("could not set migration start time: %w", err)
		}
	}

	// If the restore annotation is set or the grace period is elapsed and migration is not currently in progress,
	// update the shoot status to force the restoration (fallback to the "bad case" scenario)
	log.V(1).Info("Checking if the shoot should be forcefully restored")
	if hasForceRestoreAnnotation(shoot) || r.isGracePeriodElapsed(shoot) && !r.isMigrationInProgress(shoot) {

		log.Info("Updating status to force restoration")
		if err := updateStatusForRestore(ctx, r.gardenClient, shoot, r.clock); err != nil {
			return reconcile.Result{}, fmt.Errorf("could not update shoot status to force restoration: %w", err)
		}

		if hasForceRestoreAnnotation(shoot) {
			log.V(1).Info("Removing force-restore annotation")
			if err := removeForceRestoreAnnotation(ctx, r.gardenClient, shoot); err != nil {
				return reconcile.Result{}, fmt.Errorf("could not remove force-restore annotation: %w", err)
			}
		}

		// Return without requeue as the shoot is no longer being migrated (we just forced the restoration)
		return reconcile.Result{}, nil
	}

	// Requeue after the configured sync period as the shoot is still being migrated,
	// so we might need to force the restoration
	return reconcile.Result{RequeueAfter: r.config.Controllers.ShootMigration.SyncPeriod.Duration}, nil
}

func (r *migrationReconciler) isGracePeriodElapsed(shoot *gardencorev1beta1.Shoot) bool {
	return r.clock.Now().UTC().After(shoot.Status.MigrationStartTime.Add(r.config.Controllers.ShootMigration.GracePeriod.Duration))
}

func (r *migrationReconciler) isMigrationInProgress(shoot *gardencorev1beta1.Shoot) bool {
	staleCutoffTime := metav1.NewTime(r.clock.Now().UTC().Add(-r.config.Controllers.ShootMigration.LastOperationStaleDuration.Duration))
	lastOperation := shoot.Status.LastOperation
	return lastOperation != nil &&
		lastOperation.Type == gardencorev1beta1.LastOperationTypeMigrate &&
		(lastOperation.State == gardencorev1beta1.LastOperationStateProcessing || lastOperation.State == gardencorev1beta1.LastOperationStateError) &&
		!lastOperation.LastUpdateTime.Before(&staleCutoffTime)
}

func setMigrationStartTime(ctx context.Context, c client.Client, shoot *gardencorev1beta1.Shoot, migrationStartTime *metav1.Time) error {
	patch := client.MergeFrom(shoot.DeepCopy())
	shoot.Status.MigrationStartTime = migrationStartTime
	return c.Status().Patch(ctx, shoot, patch)
}

func updateStatusForRestore(ctx context.Context, c client.Client, shoot *gardencorev1beta1.Shoot, clock clock.Clock) error {
	patch := client.StrategicMergeFrom(shoot.DeepCopy())

	shoot.Status.LastOperation = &gardencorev1beta1.LastOperation{
		Type:           gardencorev1beta1.LastOperationTypeMigrate,
		State:          gardencorev1beta1.LastOperationStateAborted,
		Description:    "Shoot cluster preparation for migration has been aborted.",
		LastUpdateTime: metav1.NewTime(clock.Now().UTC()),
	}
	shoot.Status.LastErrors = nil
	shoot.Status.ObservedGeneration = shoot.Generation
	shoot.Status.RetryCycleStartTime = nil
	shoot.Status.SeedName = nil
	shoot.Status.MigrationStartTime = nil

	return c.Status().Patch(ctx, shoot, patch)
}

func hasForceRestoreAnnotation(shoot *gardencorev1beta1.Shoot) bool {
	return kutil.HasMetaDataAnnotation(shoot, v1beta1constants.AnnotationShootForceRestore, "true")
}

func removeForceRestoreAnnotation(ctx context.Context, c client.Client, shoot *gardencorev1beta1.Shoot) error {
	patch := client.MergeFrom(shoot.DeepCopy())
	delete(shoot.GetAnnotations(), v1beta1constants.AnnotationShootForceRestore)
	return c.Patch(ctx, shoot, patch)
}

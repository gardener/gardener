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

package migration

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// Reconciler reconciles the BackupEntry by forcing the backup entry's restoration to this seed during control plane
// migration if the preparation for migration in the source seed is not finished after a certain grace period and
// is considered unlikely to succeed ("bad case" scenario).
type Reconciler struct {
	GardenClient client.Client
	Config       config.BackupEntryMigrationControllerConfiguration
	Clock        clock.Clock
	SeedName     string
}

// Reconcile reconciles the BackupEntry by forcing the backup entry's restoration to this seed during control plane
// migration if the preparation for migration in the source seed is not finished after a certain grace period and
// is considered unlikely to succeed ("bad case" scenario).
func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (result reconcile.Result, err error) {
	log := logf.FromContext(ctx)

	backupEntry := &gardencorev1beta1.BackupEntry{}
	if err := r.GardenClient.Get(ctx, req.NamespacedName, backupEntry); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	// If the backup entry is being deleted or no longer being migrated to this seed, clear the migration start time
	if backupEntry.DeletionTimestamp != nil || !gutil.IsObjectBeingMigrated(ctx, r.GardenClient, backupEntry, r.SeedName, gutil.GetBackupEntrySeedNames) {
		log.V(1).Info("Clearing migration start time")
		if err := r.setMigrationStartTime(ctx, backupEntry, nil); err != nil {
			return reconcile.Result{}, fmt.Errorf("could not clear migration start time: %w", err)
		}

		// Return without requeue as the backup entry is no longer being migrated (we should not force restore)
		return reconcile.Result{}, nil
	}

	// Set the migration start time if needed
	if backupEntry.Status.MigrationStartTime == nil {
		log.V(1).Info("Setting migration start time to current time")
		if err := r.setMigrationStartTime(ctx, backupEntry, &metav1.Time{Time: r.Clock.Now().UTC()}); err != nil {
			return reconcile.Result{}, fmt.Errorf("could not set migration start time: %w", err)
		}
	}

	// If the force-restore annotation is set or the grace period is elapsed and migration is not currently in progress,
	// update the backup entry status to force the restoration (fallback to the "bad case" scenario)
	log.V(1).Info("Checking whether restoration should be forceful")
	if hasForceRestoreAnnotation(backupEntry) || r.isGracePeriodElapsed(backupEntry) && !r.isMigrationInProgress(backupEntry) {
		log.Info("Updating status to force restoration")
		if err := r.updateStatusForRestore(ctx, backupEntry); err != nil {
			return reconcile.Result{}, fmt.Errorf("could not update backup entry status to force restoration: %w", err)
		}

		if hasForceRestoreAnnotation(backupEntry) {
			log.V(1).Info("Removing force-restore annotation")
			if err := removeForceRestoreAnnotation(ctx, r.GardenClient, backupEntry); err != nil {
				return reconcile.Result{}, fmt.Errorf("could not remove force-restore annotation: %w", err)
			}
		}

		// Return without requeue as the backup entry is no longer being migrated (we just forced the restoration)
		return reconcile.Result{}, nil
	}

	// Requeue after the configured sync period as the backup entry is still being migrated,
	// so we might need to force the restoration
	return reconcile.Result{RequeueAfter: r.Config.SyncPeriod.Duration}, nil
}

func (r *Reconciler) isGracePeriodElapsed(backupEntry *gardencorev1beta1.BackupEntry) bool {
	return r.Clock.Now().UTC().After(backupEntry.Status.MigrationStartTime.Add(r.Config.GracePeriod.Duration))
}

func (r *Reconciler) isMigrationInProgress(backupEntry *gardencorev1beta1.BackupEntry) bool {
	staleCutoffTime := metav1.NewTime(r.Clock.Now().UTC().Add(-r.Config.LastOperationStaleDuration.Duration))
	lastOperation := backupEntry.Status.LastOperation
	return lastOperation != nil &&
		lastOperation.Type == gardencorev1beta1.LastOperationTypeMigrate &&
		(lastOperation.State == gardencorev1beta1.LastOperationStateProcessing || lastOperation.State == gardencorev1beta1.LastOperationStateError) &&
		!lastOperation.LastUpdateTime.Before(&staleCutoffTime)
}

func (r *Reconciler) setMigrationStartTime(ctx context.Context, backupEntry *gardencorev1beta1.BackupEntry, migrationStartTime *metav1.Time) error {
	patch := client.MergeFrom(backupEntry.DeepCopy())
	backupEntry.Status.MigrationStartTime = migrationStartTime
	return r.GardenClient.Status().Patch(ctx, backupEntry, patch)
}

func (r *Reconciler) updateStatusForRestore(ctx context.Context, backupEntry *gardencorev1beta1.BackupEntry) error {
	patch := client.StrategicMergeFrom(backupEntry.DeepCopy())

	backupEntry.Status.LastOperation = &gardencorev1beta1.LastOperation{
		Type:           gardencorev1beta1.LastOperationTypeMigrate,
		State:          gardencorev1beta1.LastOperationStateAborted,
		Description:    "BackupEntry preparation for migration has been aborted.",
		LastUpdateTime: metav1.NewTime(r.Clock.Now().UTC()),
	}
	backupEntry.Status.LastError = nil
	backupEntry.Status.ObservedGeneration = backupEntry.Generation
	backupEntry.Status.SeedName = nil
	backupEntry.Status.MigrationStartTime = nil

	return r.GardenClient.Status().Patch(ctx, backupEntry, patch)
}

func hasForceRestoreAnnotation(backupEntry *gardencorev1beta1.BackupEntry) bool {
	return kutil.HasMetaDataAnnotation(backupEntry, v1beta1constants.AnnotationShootForceRestore, "true")
}

func removeForceRestoreAnnotation(ctx context.Context, c client.Client, backupEntry *gardencorev1beta1.BackupEntry) error {
	patch := client.MergeFrom(backupEntry.DeepCopy())
	delete(backupEntry.GetAnnotations(), v1beta1constants.AnnotationShootForceRestore)
	return c.Patch(ctx, backupEntry, patch)
}

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

// Reconciler reconciles Shoot resources and updates the status for a forceful restoration in case the grace period for
// the smooth migration has been elapsed.
type Reconciler struct {
	GardenClient client.Client
	Config       config.ShootMigrationControllerConfiguration
	Clock        clock.Clock
	SeedName     string
}

// Reconcile reconciles Shoot resources.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	shoot := &gardencorev1beta1.Shoot{}
	if err := r.GardenClient.Get(ctx, request.NamespacedName, shoot); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	// If the shoot is being deleted or no longer being migrated to this seed, clear the migration start time
	if shoot.DeletionTimestamp != nil || !gutil.IsObjectBeingMigrated(ctx, r.GardenClient, shoot, r.SeedName, getShootSeedNames) {
		log.V(1).Info("Clearing migration start time")
		if err := r.setMigrationStartTime(ctx, shoot, nil); err != nil {
			return reconcile.Result{}, fmt.Errorf("could not clear migration start time: %w", err)
		}

		// Return without requeue as the shoot is no longer being migrated (we should not force restore)
		return reconcile.Result{}, nil
	}

	// Set the migration start time if needed
	if shoot.Status.MigrationStartTime == nil {
		log.V(1).Info("Setting migration start time")
		if err := r.setMigrationStartTime(ctx, shoot, &metav1.Time{Time: r.Clock.Now().UTC()}); err != nil {
			return reconcile.Result{}, fmt.Errorf("could not set migration start time: %w", err)
		}
	}

	// If the restore annotation is set or the grace period is elapsed and migration is not currently in progress,
	// update the shoot status to force the restoration (fallback to the "bad case" scenario)
	log.V(1).Info("Checking if the shoot should be forcefully restored")
	if hasForceRestoreAnnotation(shoot) || r.isGracePeriodElapsed(shoot) && !r.isMigrationInProgress(shoot) {
		log.Info("Updating status to force restoration")
		if err := r.updateStatusForRestore(ctx, shoot); err != nil {
			return reconcile.Result{}, fmt.Errorf("could not update shoot status to force restoration: %w", err)
		}

		if hasForceRestoreAnnotation(shoot) {
			log.V(1).Info("Removing force-restore annotation")
			if err := removeForceRestoreAnnotation(ctx, r.GardenClient, shoot); err != nil {
				return reconcile.Result{}, fmt.Errorf("could not remove force-restore annotation: %w", err)
			}
		}

		// Return without requeue as the shoot is no longer being migrated (we just forced the restoration)
		return reconcile.Result{}, nil
	}

	// Requeue after the configured sync period as the shoot is still being migrated,
	// so we might need to force the restoration
	return reconcile.Result{RequeueAfter: r.Config.SyncPeriod.Duration}, nil
}

func (r *Reconciler) isGracePeriodElapsed(shoot *gardencorev1beta1.Shoot) bool {
	return r.Clock.Now().UTC().After(shoot.Status.MigrationStartTime.Add(r.Config.GracePeriod.Duration))
}

func (r *Reconciler) isMigrationInProgress(shoot *gardencorev1beta1.Shoot) bool {
	staleCutoffTime := metav1.NewTime(r.Clock.Now().UTC().Add(-r.Config.LastOperationStaleDuration.Duration))
	lastOperation := shoot.Status.LastOperation
	return lastOperation != nil &&
		lastOperation.Type == gardencorev1beta1.LastOperationTypeMigrate &&
		(lastOperation.State == gardencorev1beta1.LastOperationStateProcessing || lastOperation.State == gardencorev1beta1.LastOperationStateError) &&
		!lastOperation.LastUpdateTime.Before(&staleCutoffTime)
}

func (r *Reconciler) setMigrationStartTime(ctx context.Context, shoot *gardencorev1beta1.Shoot, migrationStartTime *metav1.Time) error {
	patch := client.MergeFrom(shoot.DeepCopy())
	shoot.Status.MigrationStartTime = migrationStartTime
	return r.GardenClient.Status().Patch(ctx, shoot, patch)
}

func (r *Reconciler) updateStatusForRestore(ctx context.Context, shoot *gardencorev1beta1.Shoot) error {
	patch := client.StrategicMergeFrom(shoot.DeepCopy())

	shoot.Status.LastOperation = &gardencorev1beta1.LastOperation{
		Type:           gardencorev1beta1.LastOperationTypeMigrate,
		State:          gardencorev1beta1.LastOperationStateAborted,
		Description:    "Shoot cluster preparation for migration has been aborted.",
		LastUpdateTime: metav1.NewTime(r.Clock.Now().UTC()),
	}
	shoot.Status.LastErrors = nil
	shoot.Status.ObservedGeneration = shoot.Generation
	shoot.Status.RetryCycleStartTime = nil
	shoot.Status.SeedName = nil
	shoot.Status.MigrationStartTime = nil

	return r.GardenClient.Status().Patch(ctx, shoot, patch)
}

func hasForceRestoreAnnotation(shoot *gardencorev1beta1.Shoot) bool {
	return kutil.HasMetaDataAnnotation(shoot, v1beta1constants.AnnotationShootForceRestore, "true")
}

func removeForceRestoreAnnotation(ctx context.Context, c client.Client, shoot *gardencorev1beta1.Shoot) error {
	patch := client.MergeFrom(shoot.DeepCopy())
	delete(shoot.GetAnnotations(), v1beta1constants.AnnotationShootForceRestore)
	return c.Patch(ctx, shoot, patch)
}

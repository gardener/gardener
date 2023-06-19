// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package state

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/utils/clock"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	"github.com/gardener/gardener/pkg/utils/gardener/shootstate"
)

// Reconciler performs periodic backups of Shoot states.
type Reconciler struct {
	GardenClient client.Client
	SeedClient   client.Client
	Config       config.ShootStateControllerConfiguration
	Clock        clock.Clock
	SeedName     string
}

var (
	// RequeueWhenShootIsNotYetCreated is the duration for the requeueing when a shoot is not yet successfully created.
	RequeueWhenShootIsNotYetCreated = 10 * time.Minute
	// JitterDuration is the duration for jittering when scheduling the next periodic backup.
	JitterDuration = 30 * time.Minute
)

// Reconcile performs periodic backups of Shoot states and persists them into ShootState resources in the garden
// cluster.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	ctx, cancel := controllerutils.GetMainReconciliationContext(ctx, controllerutils.DefaultReconciliationTimeout)
	defer cancel()

	shoot := &gardencorev1beta1.Shoot{}
	if err := r.GardenClient.Get(ctx, request.NamespacedName, shoot); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	// if shoot got deleted or is no longer managed by this gardenlet (e.g., due to migration to another seed) then don't requeue
	if shoot.DeletionTimestamp != nil || pointer.StringDeref(shoot.Spec.SeedName, "") != r.SeedName {
		return reconcile.Result{}, nil
	}

	if !shootCreatedSuccessfully(shoot.Status) {
		log.Info("Requeuing because shoot was not yet successfully created", "requeueAfter", RequeueWhenShootIsNotYetCreated)
		return reconcile.Result{RequeueAfter: RequeueWhenShootIsNotYetCreated}, nil
	}

	shootState := &gardencorev1beta1.ShootState{}
	if err := r.GardenClient.Get(ctx, client.ObjectKeyFromObject(shoot), shootState); client.IgnoreNotFound(err) != nil {
		return reconcile.Result{}, fmt.Errorf("failed fetching ShootState %s: %w", client.ObjectKeyFromObject(shoot), err)
	}

	var lastBackup time.Time
	if v, ok := shootState.Annotations[v1beta1constants.GardenerTimestamp]; ok {
		var err error
		lastBackup, err = time.Parse(time.RFC3339, v)
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("failed parsing timestamp %q on ShootState object: %w", v, err)
		}
	}

	if nextBackupDue := lastBackup.UTC().Add(r.Config.SyncPeriod.Duration); nextBackupDue.Before(r.Clock.Now().UTC()) {
		log.Info("Performing periodic ShootState backup", "lastBackup", lastBackup.Round(time.Minute), "nextBackupDue", nextBackupDue.Round(time.Minute), "now", r.Clock.Now().UTC().Round(time.Minute))
		if err := shootstate.Deploy(ctx, r.Clock, r.GardenClient, r.SeedClient, shoot); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed performing periodic ShootState backup: %w", err)
		}
		lastBackup = r.Clock.Now()
	} else {
		log.Info("No need to perform periodic ShootState backup yet", "lastBackup", lastBackup.Round(time.Minute), "syncPeriod", r.Config.SyncPeriod.Duration)
	}

	requeueAfter, nextBackup := r.requeueAfter(lastBackup)
	log.Info("Scheduled next periodic ShootState backup for Shoot", "duration", requeueAfter.Round(time.Minute), "nextBackup", nextBackup.Round(time.Minute))
	return reconcile.Result{RequeueAfter: requeueAfter}, nil
}

func (r *Reconciler) requeueAfter(lastBackup time.Time) (time.Duration, time.Time) {
	var (
		nextRegularBackup = lastBackup.UTC().Add(r.Config.SyncPeriod.Duration)
		randomDuration    = controllerutils.RandomDuration(JitterDuration)

		nextBackup              = nextRegularBackup.Add(-JitterDuration / 2).Add(randomDuration)
		durationUntilNextBackup = nextBackup.UTC().Sub(r.Clock.Now().UTC())
	)

	return durationUntilNextBackup, nextBackup
}

func shootCreatedSuccessfully(status gardencorev1beta1.ShootStatus) bool {
	return status.LastOperation != nil &&
		((status.LastOperation.Type == gardencorev1beta1.LastOperationTypeCreate && status.LastOperation.State == gardencorev1beta1.LastOperationStateSucceeded) ||
			status.LastOperation.Type == gardencorev1beta1.LastOperationTypeReconcile)
}

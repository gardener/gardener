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

package seed

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
)

const seedBackupBucketsCheckReconcilerName = "backupbuckets-check"

func (c *Controller) backupBucketEnqueue(bb *gardencorev1beta1.BackupBucket) {
	seedName := bb.Spec.SeedName
	if seedName == nil {
		return
	}

	c.seedBackupBucketsCheckQueue.Add(*seedName)
}

func (c *Controller) backupBucketAdd(obj interface{}) {
	backupBucket, ok := obj.(*gardencorev1beta1.BackupBucket)
	if !ok {
		return
	}

	c.backupBucketEnqueue(backupBucket)
}

func (c *Controller) backupBucketUpdate(oldObj, newObj interface{}) {
	oldBackupBucket, ok1 := oldObj.(*gardencorev1beta1.BackupBucket)
	newBackupBucket, ok2 := newObj.(*gardencorev1beta1.BackupBucket)

	if !ok1 || !ok2 {
		return
	}

	if lastErrorChanged(oldBackupBucket.Status.LastError, newBackupBucket.Status.LastError) {
		c.backupBucketEnqueue(newBackupBucket)
	}
}

// NewBackupBucketsCheckReconciler creates a new reconciler that maintains the BackupBucketsReady condition of Seeds
// according to the observed status of BackupBuckets.
func NewBackupBucketsCheckReconciler(gardenClient client.Client, config config.SeedBackupBucketsCheckControllerConfiguration, clock clock.Clock) *backupBucketsCheckReconciler {
	return &backupBucketsCheckReconciler{
		gardenClient: gardenClient,
		config:       config,
		clock:        clock,
	}
}

type backupBucketsCheckReconciler struct {
	gardenClient client.Client
	config       config.SeedBackupBucketsCheckControllerConfiguration
	clock        clock.Clock
}

type backupBucketInfo struct {
	name     string
	errorMsg string
}

func (b backupBucketInfo) String() string {
	return fmt.Sprintf("Name: %s, Error: %s", b.name, b.errorMsg)
}

func (b *backupBucketsCheckReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	seed := &gardencorev1beta1.Seed{}
	if err := b.gardenClient.Get(ctx, req.NamespacedName, seed); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcileResult(nil)
		}
		return reconcileResult(fmt.Errorf("error retrieving object from store: %w", err))
	}

	backupBucketList := &gardencorev1beta1.BackupBucketList{}
	if err := b.gardenClient.List(ctx, backupBucketList, client.MatchingFields{core.BackupBucketSeedName: seed.Name}); err != nil {
		return reconcileResult(err)
	}

	conditionBackupBucketsReady := gardencorev1beta1helper.GetOrInitCondition(seed.Status.Conditions, gardencorev1beta1.SeedBackupBucketsReady)

	var (
		bbCount                int
		erroneousBackupBuckets []backupBucketInfo
	)
	for _, bb := range backupBucketList.Items {
		// not needed for real client, but fake client doesn't support field selector
		// see https://github.com/kubernetes-sigs/controller-runtime/issues/1376
		// could be solved by switching from fake client to real client against envtest
		if bb.Spec.SeedName == nil || *bb.Spec.SeedName != seed.Name {
			continue
		}

		bbCount++
		if occurred, msg := gardencorev1beta1helper.BackupBucketIsErroneous(&bb); occurred {
			erroneousBackupBuckets = append(erroneousBackupBuckets, backupBucketInfo{
				name:     bb.Name,
				errorMsg: msg,
			})
		}
	}

	conditionThreshold := getThresholdForCondition(b.config.ConditionThresholds, gardencorev1beta1.SeedBackupBucketsReady)
	switch {
	case len(erroneousBackupBuckets) > 0:
		errorMsg := "The following BackupBuckets have issues:"
		for _, bb := range erroneousBackupBuckets {
			errorMsg += fmt.Sprintf("\n* %s", bb)
		}
		conditionBackupBucketsReady = setToProgressingOrFalse(b.clock, conditionThreshold, conditionBackupBucketsReady, "BackupBucketsError", errorMsg)
		if updateErr := patchSeedCondition(ctx, b.gardenClient, seed, conditionBackupBucketsReady); updateErr != nil {
			return reconcileResult(updateErr)
		}
	case bbCount > 0:
		if updateErr := patchSeedCondition(ctx, b.gardenClient, seed, gardencorev1beta1helper.UpdatedCondition(conditionBackupBucketsReady,
			gardencorev1beta1.ConditionTrue, "BackupBucketsAvailable", "Backup Buckets are available.")); updateErr != nil {
			return reconcileResult(updateErr)
		}
	case bbCount == 0:
		conditionBackupBucketsReady = setToProgressingOrUnknown(b.clock, conditionThreshold, conditionBackupBucketsReady, "BackupBucketsGone", "Backup Buckets are gone.")
		if updateErr := patchSeedCondition(ctx, b.gardenClient, seed, conditionBackupBucketsReady); updateErr != nil {
			return reconcileResult(updateErr)
		}
	}

	return reconcileAfter(b.config.SyncPeriod.Duration)
}

func lastErrorChanged(oldLastError, newLastError *gardencorev1beta1.LastError) bool {
	return oldLastError == nil && newLastError != nil ||
		oldLastError != nil && newLastError == nil ||
		oldLastError != nil && newLastError != nil && oldLastError.Description != newLastError.Description
}

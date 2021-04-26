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

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/client/kubernetes"

	"github.com/sirupsen/logrus"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func (c *Controller) backupBucketEnqueue(bb *gardencorev1beta1.BackupBucket) {
	seedName := bb.Spec.SeedName
	if seedName == nil {
		return
	}

	c.seedBackupBucketQueue.Add(*seedName)
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

	if !apiequality.Semantic.DeepEqual(oldBackupBucket.Status, newBackupBucket.Status) || !apiequality.Semantic.DeepEqual(oldBackupBucket.Spec, newBackupBucket.Spec) {
		c.backupBucketEnqueue(newBackupBucket)
	}
}

// NewDefaultBackupBucketControl returns a new default control to checks backup buckets of related seeds.
func NewDefaultBackupBucketControl(logger logrus.FieldLogger, gardenClient kubernetes.Interface) *backupBucketReconciler {
	return &backupBucketReconciler{
		logger:       logger,
		gardenClient: gardenClient,
	}
}

type backupBucketReconciler struct {
	logger       logrus.FieldLogger
	gardenClient kubernetes.Interface
}

type backupBucketInfo struct {
	name     string
	errorMsg string
}

func (b *backupBucketInfo) String() string {
	return fmt.Sprintf("Name: %s, Error: %s", b.name, b.errorMsg)
}

func (b *backupBucketReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	seed := &gardencorev1beta1.Seed{}
	if err := b.gardenClient.Client().Get(ctx, req.NamespacedName, seed); err != nil {
		if apierrors.IsNotFound(err) {
			b.logger.Debugf("[BACKUPBUCKET SEED RECONCILE] %s - skipping because Seed has been deleted", req.NamespacedName)
			return reconcileResult(nil)
		}
		b.logger.Infof("[BACKUPBUCKET SEED RECONCILE] %s - unable to retrieve seed object from store: %v", req.NamespacedName, err)
		return reconcileResult(err)
	}

	backupBucketList := &gardencorev1beta1.BackupBucketList{}
	if err := b.gardenClient.Client().List(ctx, backupBucketList); err != nil {
		return reconcileResult(err)
	}

	conditionBackupBucketsReady := gardencorev1beta1helper.GetOrInitCondition(seed.Status.Conditions, gardencorev1beta1.SeedBackupBucketsReady)

	var (
		bbCount                int
		erroneousBackupBuckets []backupBucketInfo
	)
	for _, bb := range backupBucketList.Items {
		if *bb.Spec.SeedName != seed.Name {
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

	switch {
	case len(erroneousBackupBuckets) > 0:
		errorMsg := "The following BackupBuckets have issues:"
		for _, bb := range erroneousBackupBuckets {
			errorMsg += fmt.Sprintf("\n* %s", bb.String())
		}

		if updateErr := patchSeedCondition(ctx, b.gardenClient.Client(), seed, gardencorev1beta1helper.UpdatedCondition(conditionBackupBucketsReady,
			gardencorev1beta1.ConditionFalse, "BackupBucketsError", errorMsg)); updateErr != nil {
			return reconcileResult(updateErr)
		}
	case bbCount > 0:
		if updateErr := patchSeedCondition(ctx, b.gardenClient.Client(), seed, gardencorev1beta1helper.UpdatedCondition(conditionBackupBucketsReady,
			gardencorev1beta1.ConditionTrue, "BackupBucketsAvailable", "Backup Buckets are available.")); updateErr != nil {
			return reconcileResult(updateErr)
		}
	case bbCount == 0:
		if updateErr := patchSeedCondition(ctx, b.gardenClient.Client(), seed, gardencorev1beta1helper.UpdatedCondition(conditionBackupBucketsReady,
			gardencorev1beta1.ConditionUnknown, "BackupBucketsGone", "Backup Buckets are gone.")); updateErr != nil {
			return reconcileResult(updateErr)
		}
	}

	return reconcileResult(nil)
}

func patchSeedCondition(ctx context.Context, c client.StatusClient, seed *gardencorev1beta1.Seed, condition gardencorev1beta1.Condition) error {
	patch := client.StrategicMergeFrom(seed.DeepCopy())

	conditions := gardencorev1beta1helper.MergeConditions(seed.Status.Conditions, condition)
	if !gardencorev1beta1helper.ConditionsNeedUpdate(seed.Status.Conditions, conditions) {
		return nil
	}

	seed.Status.Conditions = conditions
	return c.Status().Patch(ctx, seed, patch)
}

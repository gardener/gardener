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
	gardencoreclientset "github.com/gardener/gardener/pkg/client/core/clientset/versioned"
	gardencorelisters "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/logger"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
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

type backupBucketReconciler struct {
	clientMap clientmap.ClientMap

	seedLister         gardencorelisters.SeedLister
	backupBucketLister gardencorelisters.BackupBucketLister
}

type backupBucketInfo struct {
	name     string
	errorMsg string
}

func (b *backupBucketInfo) String() string {
	return fmt.Sprintf("Name: %s, Error: %s", b.name, b.errorMsg)
}

func (b *backupBucketReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	seed, err := b.seedLister.Get(req.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Logger.Debugf("[BACKUPBUCKET SEED RECONCILE] %s - skipping because Seed has been deleted", req.NamespacedName)
			return reconcile.Result{}, nil
		}
		logger.Logger.Infof("[BACKUPBUCKET SEED RECONCILE] %s - unable to retrieve seed object from store: %v", req.NamespacedName, err)
		return reconcile.Result{}, err
	}

	bbs, err := b.backupBucketLister.List(labels.Everything())
	if err != nil {
		return reconcile.Result{}, err
	}

	conditionBackupBucketsReady := gardencorev1beta1helper.GetOrInitCondition(seed.Status.Conditions, gardencorev1beta1.SeedBackupBucketsReady)

	var (
		bbCount                int
		erroneousBackupBuckets []backupBucketInfo
	)
	for _, bb := range bbs {
		if *bb.Spec.SeedName != seed.Name {
			continue
		}

		bbCount++
		if occurred, msg := gardencorev1beta1helper.BackupBucketIsErroneous(bb); occurred {
			erroneousBackupBuckets = append(erroneousBackupBuckets, backupBucketInfo{
				name:     bb.Name,
				errorMsg: msg,
			})
		}
	}

	gardenClient, err := b.clientMap.GetClient(ctx, keys.ForGarden())
	if err != nil {
		return reconcile.Result{}, err
	}

	switch {
	case len(erroneousBackupBuckets) > 0:
		errorMsg := "The following BackupBuckets have issues:"
		for _, bb := range erroneousBackupBuckets {
			errorMsg += fmt.Sprintf("\n* %s", bb.String())
		}

		if updateErr := patchSeedCondition(ctx, gardenClient.GardenCore(), seed, gardencorev1beta1helper.UpdatedCondition(conditionBackupBucketsReady,
			gardencorev1beta1.ConditionFalse, "BackupBucketsError", errorMsg)); updateErr != nil {
			return reconcile.Result{}, updateErr
		}
	case bbCount > 0:
		if updateErr := patchSeedCondition(ctx, gardenClient.GardenCore(), seed, gardencorev1beta1helper.UpdatedCondition(conditionBackupBucketsReady,
			gardencorev1beta1.ConditionTrue, "BackupBucketsAvailable", "Backup Buckets are available.")); updateErr != nil {
			return reconcile.Result{}, updateErr
		}
	case bbCount == 0:
		if updateErr := patchSeedCondition(ctx, gardenClient.GardenCore(), seed, gardencorev1beta1helper.UpdatedCondition(conditionBackupBucketsReady,
			gardencorev1beta1.ConditionUnknown, "BackupBucketsGone", "Backup Buckets are gone.")); updateErr != nil {
			return reconcile.Result{}, updateErr
		}
	}

	return reconcile.Result{}, nil
}

func patchSeedCondition(ctx context.Context, gardenClientSet gardencoreclientset.Interface, seed *gardencorev1beta1.Seed, condition gardencorev1beta1.Condition) error {
	conditions := gardencorev1beta1helper.MergeConditions(seed.Status.Conditions, condition)
	if !gardencorev1beta1helper.ConditionsNeedUpdate(seed.Status.Conditions, conditions) {
		return nil
	}

	seedCopy := seed.DeepCopy()
	seedCopy.Status.Conditions = conditions
	patchBytes, err := kutil.CreateTwoWayMergePatch(seed, seedCopy)
	if err != nil {
		return fmt.Errorf("could not create data for status patch: %s", err)
	}

	_, patchErr := gardenClientSet.CoreV1beta1().Seeds().Patch(ctx, seed.Name, types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{}, "status")
	return patchErr
}

// NewDefaultBackupBucketControl returns a new default control to checks backup buckets of related seeds.
func NewDefaultBackupBucketControl(clientMap clientmap.ClientMap, bbLister gardencorelisters.BackupBucketLister, seedLister gardencorelisters.SeedLister) *backupBucketReconciler {
	return &backupBucketReconciler{
		clientMap: clientMap,

		backupBucketLister: bbLister,
		seedLister:         seedLister,
	}
}

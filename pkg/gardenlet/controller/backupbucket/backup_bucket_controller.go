// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package backupbucket

import (
	"time"

	"k8s.io/client-go/tools/cache"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/logger"
)

func (c *Controller) backupBucketAdd(obj interface{}) {
	backupBucket, ok := obj.(*gardencorev1beta1.BackupBucket)
	if !ok {
		logger.Logger.Errorf("Couldn't convert object: %T", obj)
		return
	}

	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		logger.Logger.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}

	addAfter := controllerutils.ReconcileOncePer24hDuration(backupBucket.ObjectMeta, backupBucket.Status.ObservedGeneration, backupBucket.Status.LastOperation)
	if addAfter > 0 {
		logger.Logger.Infof("Scheduled next reconciliation for BackupBucket %q in %s (%s)", key, addAfter, time.Now().Add(addAfter))
	}

	c.backupBucketQueue.AddAfter(key, addAfter)
}

func (c *Controller) backupBucketUpdate(_, newObj interface{}) {
	var (
		newBackupBucket    = newObj.(*gardencorev1beta1.BackupBucket)
		backupBucketLogger = logger.NewFieldLogger(logger.Logger, "backupbucket", newBackupBucket.Name)
	)

	// If the generation did not change for an update event (i.e., no changes to the .spec section have
	// been made), we do not want to add the BackupBucket to the queue. The periodic reconciliation is handled
	// elsewhere by adding the BackupBucket to the queue to dedicated times.
	if newBackupBucket.Generation == newBackupBucket.Status.ObservedGeneration {
		backupBucketLogger.Debug("Do not need to do anything as the Update event occurred due to .status field changes")
		return
	}

	// if oldBackupBucket.Spec.Seed !=nil && newBackupBucket.Spec.Seed != oldBackupBucket.Spec.Seed {
	// 	TODO:apply migration operation on old backupBucket extension.
	//  Idea here is migration operation on extension resources will actually force extension controller
	//  to prepare for migration.
	// 	And delete the old backupBucket resource
	// }

	c.backupBucketAdd(newObj)
}

func (c *Controller) backupBucketDelete(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		logger.Logger.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}
	c.backupBucketQueue.Add(key)
}

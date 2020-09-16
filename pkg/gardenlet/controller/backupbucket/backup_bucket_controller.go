// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package backupbucket

import (
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/logger"

	"k8s.io/client-go/tools/cache"
)

func (c *Controller) backupBucketAdd(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		logger.Logger.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}
	c.backupBucketQueue.Add(key)
}

func (c *Controller) backupBucketUpdate(oldObj, newObj interface{}) {
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

// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package backupentry

import (
	"fmt"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/utils"

	"k8s.io/client-go/tools/cache"
)

func (c *Controller) backupEntryAdd(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		logger.Logger.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}

	backupEntry, ok := obj.(*gardencorev1beta1.BackupEntry)
	if !ok {
		return
	}

	if backupEntry.Generation == backupEntry.Status.ObservedGeneration {
		// spread BackupEntry reconciliation across one minute to avoid reconciling all BackupEntries roughly at the
		// same time after startup of the gardenlet
		c.backupEntryQueue.AddAfter(key, utils.RandomDuration(time.Minute))
		return
	}

	// don't add random duration for enqueueing new BackupBuckets, that have never been reconciled
	c.backupEntryQueue.Add(key)
}

func (c *Controller) backupEntryUpdate(oldObj, newObj interface{}) {
	var (
		newBackupEntry    = newObj.(*gardencorev1beta1.BackupEntry)
		backupEntryLogger = logger.NewFieldLogger(logger.Logger, "backupentry", fmt.Sprintf("%s/%s", newBackupEntry.Namespace, newBackupEntry.Name))
	)

	// If the generation did not change for an update event (i.e., no changes to the .spec section have
	// been made), we do not want to add the BackupEntry to the queue. The periodic reconciliation is handled
	// elsewhere by adding the BackupEntry to the queue to dedicated times.
	if newBackupEntry.Generation == newBackupEntry.Status.ObservedGeneration {
		backupEntryLogger.Debug("Do not need to do anything as the Update event occurred due to .status field changes")
		return
	}

	c.backupEntryAdd(newObj)
}

func (c *Controller) backupEntryDelete(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		logger.Logger.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}
	c.backupEntryQueue.Add(key)
}

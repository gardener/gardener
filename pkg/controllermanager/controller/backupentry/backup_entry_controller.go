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

package backupentry

import (
	"fmt"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	"github.com/gardener/gardener/pkg/logger"

	"k8s.io/client-go/tools/cache"
)

func (c *Controller) backupEntryAdd(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		logger.Logger.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}
	c.backupEntryQueue.Add(key)
}

func (c *Controller) backupEntryUpdate(oldObj, newObj interface{}) {
	var (
		newBackupEntry    = newObj.(*gardencorev1alpha1.BackupEntry)
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

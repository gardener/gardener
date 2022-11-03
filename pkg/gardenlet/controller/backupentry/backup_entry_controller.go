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

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/controllerutils"

	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (c *Controller) backupEntryAdd(obj interface{}) {
	backupEntry, ok := obj.(*gardencorev1beta1.BackupEntry)
	if !ok {
		c.log.Error(fmt.Errorf("could not convert object of type %T to *gardencorev1beta1.BackupEntry", obj), "Unexpected object type", "obj", obj)
		return
	}

	log := c.log.WithValues("backupEntry", client.ObjectKeyFromObject(backupEntry))

	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		log.Error(err, "Could not get key")
		return
	}

	addAfter := controllerutils.ReconcileOncePer24hDuration(c.clock, backupEntry.ObjectMeta, backupEntry.Status.ObservedGeneration, backupEntry.Status.LastOperation)
	if addAfter > 0 {
		log.V(1).Info("Scheduled next reconciliation for BackupEntry", "duration", addAfter, "nextReconciliation", c.clock.Now().Add(addAfter))
	}

	c.backupEntryQueue.AddAfter(key, addAfter)
}

func (c *Controller) backupEntryUpdate(_, newObj interface{}) {
	var (
		newBackupEntry = newObj.(*gardencorev1beta1.BackupEntry)
		log            = c.log.WithValues("backupEntry", client.ObjectKeyFromObject(newBackupEntry))
	)

	// If the generation did not change for an update event (i.e., no changes to the .spec section have
	// been made), we do not want to add the BackupEntry to the queue. The periodic reconciliation is handled
	// elsewhere by adding the BackupEntry to the queue to dedicated times.
	if newBackupEntry.Generation == newBackupEntry.Status.ObservedGeneration && !v1beta1helper.HasOperationAnnotation(newBackupEntry.ObjectMeta.Annotations) {
		log.V(1).Info("Do not need to do anything as the Update event occurred due to .status field changes")
		return
	}

	c.backupEntryAdd(newObj)
}

func (c *Controller) backupEntryDelete(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		c.log.Error(err, "Could not get key", "obj", obj)
		return
	}

	c.backupEntryQueue.Add(key)
}

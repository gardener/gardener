// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controllerregistration

import (
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
)

func (c *Controller) backupEntryAdd(obj interface{}) {
	backupEntry, ok := obj.(*gardencorev1beta1.BackupEntry)
	if !ok {
		return
	}

	if backupEntry.Spec.SeedName == nil {
		return
	}

	c.controllerRegistrationSeedQueue.Add(*backupEntry.Spec.SeedName)
}

func (c *Controller) backupEntryUpdate(oldObj, newObj interface{}) {
	oldObject, ok := oldObj.(*gardencorev1beta1.BackupEntry)
	if !ok {
		return
	}

	newObject, ok := newObj.(*gardencorev1beta1.BackupEntry)
	if !ok {
		return
	}

	if apiequality.Semantic.DeepEqual(oldObject.Spec.SeedName, newObject.Spec.SeedName) &&
		oldObject.Spec.BucketName == newObject.Spec.BucketName {
		return
	}

	c.backupEntryAdd(newObj)
}

func (c *Controller) backupEntryDelete(obj interface{}) {
	c.backupEntryAdd(obj)
}

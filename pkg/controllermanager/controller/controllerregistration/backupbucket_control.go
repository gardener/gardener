// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controllerregistration

import (
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
)

func (c *Controller) backupBucketAdd(obj interface{}) {
	backupBucket, ok := obj.(*gardencorev1beta1.BackupBucket)
	if !ok {
		return
	}

	if backupBucket.Spec.SeedName == nil {
		return
	}

	c.controllerRegistrationSeedQueue.Add(*backupBucket.Spec.SeedName)
}

func (c *Controller) backupBucketUpdate(oldObj, newObj interface{}) {
	oldObject, ok := oldObj.(*gardencorev1beta1.BackupBucket)
	if !ok {
		return
	}

	newObject, ok := newObj.(*gardencorev1beta1.BackupBucket)
	if !ok {
		return
	}

	if apiequality.Semantic.DeepEqual(oldObject.Spec.SeedName, newObject.Spec.SeedName) &&
		oldObject.Spec.Provider.Type == newObject.Spec.Provider.Type {
		return
	}

	c.backupBucketAdd(newObj)
}

func (c *Controller) backupBucketDelete(obj interface{}) {
	c.backupBucketAdd(obj)
}

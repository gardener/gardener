// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controllerregistration

import (
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
)

func (c *Controller) controllerInstallationAdd(obj interface{}) {
	controllerInstallation, ok := obj.(*gardencorev1beta1.ControllerInstallation)
	if !ok {
		return
	}

	c.controllerRegistrationSeedQueue.Add(controllerInstallation.Spec.SeedRef.Name)
}

func (c *Controller) controllerInstallationUpdate(oldObj, newObj interface{}) {
	oldObject, ok := oldObj.(*gardencorev1beta1.ControllerInstallation)
	if !ok {
		return
	}

	newObject, ok := newObj.(*gardencorev1beta1.ControllerInstallation)
	if !ok {
		return
	}

	if gardencorev1beta1helper.IsControllerInstallationRequired(*oldObject) == gardencorev1beta1helper.IsControllerInstallationRequired(*newObject) {
		return
	}

	c.controllerInstallationAdd(newObj)
}

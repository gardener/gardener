// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"fmt"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	gardencorelisters "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/logger"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
)

func (c *Controller) controllerInstallationOfSeedAdd(obj interface{}) {
	controllerInstallation, ok := obj.(*gardencorev1beta1.ControllerInstallation)
	if !ok {
		return
	}
	c.seedExtensionCheckQueue.Add(controllerInstallation.Spec.SeedRef.Name)
}

func (c *Controller) controllerInstallationOfSeedUpdate(oldObj, newObj interface{}) {
	c.controllerInstallationOfSeedAdd(newObj)
}

func (c *Controller) controllerInstallationOfSeedDelete(obj interface{}) {
	c.controllerInstallationOfSeedAdd(obj)
}

func (c *Controller) reconcileSeedExtensionCheckKey(key string) error {
	_, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	seed, err := c.seedLister.Get(name)
	if apierrors.IsNotFound(err) {
		logger.Logger.Debugf("[SEED EXTENSION CHECK] %s - skipping because Seed has been deleted", key)
		return nil
	}
	if err != nil {
		logger.Logger.Infof("[SEED EXTENSION CHECK] %s - unable to retrieve object from store: %v", key, err)
		return err
	}

	return c.extensionCheckControl.ReconcileExtensionCheckFor(seed)
}

// ExtensionCheckControlInterface implements the control logic for updating Seeds. It is implemented as an interface to allow
// for extensions that provide different semantics. Currently, there is only one implementation.
type ExtensionCheckControlInterface interface {
	// ReconcileExtensionCheckFor implements the control logic for Seed extension checks.
	// If an implementation returns a non-nil error, the invocation will be retried using a rate-limited strategy.
	// Implementors should sink any errors that they do not wish to trigger a retry, and they may feel free to
	// exit exceptionally at any point provided they wish the update to be re-run at a later point in time.
	ReconcileExtensionCheckFor(seed *gardencorev1beta1.Seed) error
}

// NewDefaultExtensionCheckControl returns a new instance of the default implementation ExtensionCheckControlInterface that
// implements the documented semantics for Seeds. You should use an instance returned from NewDefaultExtensionCheckControl() for any
// scenario other than testing.
func NewDefaultExtensionCheckControl(
	k8sGardenClient kubernetes.Interface,
	controllerInstallationLister gardencorelisters.ControllerInstallationLister,
	recorder record.EventRecorder,
) ExtensionCheckControlInterface {
	return &defaultExtensionCheckControl{
		k8sGardenClient,
		controllerInstallationLister,
		recorder,
	}
}

type defaultExtensionCheckControl struct {
	k8sGardenClient              kubernetes.Interface
	controllerInstallationLister gardencorelisters.ControllerInstallationLister
	recorder                     record.EventRecorder
}

func (c *defaultExtensionCheckControl) ReconcileExtensionCheckFor(obj *gardencorev1beta1.Seed) error {
	var (
		seed                         = obj.DeepCopy()
		conditionSeedExtensionsReady = gardencorev1beta1helper.GetOrInitCondition(seed.Status.Conditions, gardencorev1beta1.SeedExtensionsReady)
	)

	conditionSeedExtensionsReady = gardencorev1beta1helper.UpdatedCondition(conditionSeedExtensionsReady, gardencorev1beta1.ConditionTrue, "AllExtensionsReady", "All extensions installed into the seed cluster are ready and healthy.")

	controllerInstallationList, err := c.controllerInstallationLister.List(labels.Everything())
	if err != nil {
		return err
	}

	for _, controllerInstallation := range controllerInstallationList {
		if controllerInstallation.Spec.SeedRef.Name != seed.Name {
			continue
		}

		if len(controllerInstallation.Status.Conditions) == 0 {
			conditionSeedExtensionsReady = gardencorev1beta1helper.UpdatedCondition(conditionSeedExtensionsReady, gardencorev1beta1.ConditionFalse, "NotAllExtensionsInstalled", fmt.Sprintf("Extension %q has not yet been installed", controllerInstallation.Name))
			break
		}

		var (
			allRequiredConditionsHealthy = true
			requiredConditions           = map[gardencorev1beta1.ConditionType]struct{}{
				gardencorev1beta1.ControllerInstallationValid:     {},
				gardencorev1beta1.ControllerInstallationInstalled: {},
				gardencorev1beta1.ControllerInstallationHealthy:   {},
			}
		)

		for _, condition := range controllerInstallation.Status.Conditions {
			if _, ok := requiredConditions[condition.Type]; !ok {
				continue
			}

			if condition.Status != gardencorev1beta1.ConditionTrue {
				conditionSeedExtensionsReady = gardencorev1beta1helper.UpdatedCondition(conditionSeedExtensionsReady, gardencorev1beta1.ConditionFalse, "NotAllExtensionsReady", fmt.Sprintf("Condition %q for extension %q is %s: %s", condition.Type, condition.Status, controllerInstallation.Name, condition.Message))
				allRequiredConditionsHealthy = false
				break
			}
		}

		if !allRequiredConditionsHealthy {
			break
		}
	}

	_, err = kutil.TryUpdateSeedConditions(c.k8sGardenClient.GardenCore(), retry.DefaultBackoff, seed.ObjectMeta,
		func(seed *gardencorev1beta1.Seed) (*gardencorev1beta1.Seed, error) {
			seed.Status.Conditions = gardencorev1beta1helper.MergeConditions(seed.Status.Conditions, conditionSeedExtensionsReady)
			return seed, nil
		},
	)
	return err
}

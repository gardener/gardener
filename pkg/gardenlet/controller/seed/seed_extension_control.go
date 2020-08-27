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
	"context"
	"fmt"

	"github.com/gardener/gardener/pkg/client/kubernetes"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	gardencorelisters "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/logger"
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
	clientMap clientmap.ClientMap,
	controllerInstallationLister gardencorelisters.ControllerInstallationLister,
	nowFunc func() metav1.Time,
) ExtensionCheckControlInterface {
	return &defaultExtensionCheckControl{
		clientMap,
		controllerInstallationLister,
		nowFunc,
	}
}

type defaultExtensionCheckControl struct {
	clientMap                    clientmap.ClientMap
	controllerInstallationLister gardencorelisters.ControllerInstallationLister
	nowFunc                      func() metav1.Time
}

func (c *defaultExtensionCheckControl) ReconcileExtensionCheckFor(obj *gardencorev1beta1.Seed) error {
	ctx := context.TODO()

	gardenClient, err := c.clientMap.GetClient(ctx, keys.ForGarden())
	if err != nil {
		return fmt.Errorf("failed to get garden client: %w", err)
	}

	controllerInstallationList, err := c.controllerInstallationLister.List(labels.Everything())
	if err != nil {
		return err
	}

	var (
		seed         = obj.DeepCopy()
		notValid     = make(map[string]string)
		notInstalled = make(map[string]string)
		notHealthy   = make(map[string]string)
	)

	for _, controllerInstallation := range controllerInstallationList {
		if controllerInstallation.Spec.SeedRef.Name != seed.Name {
			continue
		}

		if len(controllerInstallation.Status.Conditions) == 0 {
			notInstalled[controllerInstallation.Name] = "extension was not yet installed"
			continue
		}

		var (
			conditionsReady    = 0
			requiredConditions = map[gardencorev1beta1.ConditionType]struct{}{
				gardencorev1beta1.ControllerInstallationValid:     {},
				gardencorev1beta1.ControllerInstallationInstalled: {},
				gardencorev1beta1.ControllerInstallationHealthy:   {},
			}
		)

		for _, condition := range controllerInstallation.Status.Conditions {
			if _, ok := requiredConditions[condition.Type]; !ok {
				continue
			}

			if condition.Type == gardencorev1beta1.ControllerInstallationValid && condition.Status != gardencorev1beta1.ConditionTrue {
				notValid[controllerInstallation.Name] = condition.Message
				break
			}

			if condition.Type == gardencorev1beta1.ControllerInstallationInstalled && condition.Status != gardencorev1beta1.ConditionTrue {
				notInstalled[controllerInstallation.Name] = condition.Message
				break
			}

			if condition.Type == gardencorev1beta1.ControllerInstallationHealthy && condition.Status != gardencorev1beta1.ConditionTrue {
				notHealthy[controllerInstallation.Name] = condition.Message
				break
			}

			conditionsReady++
		}

		if _, found := notHealthy[controllerInstallation.Name]; !found && conditionsReady != len(requiredConditions) {
			notHealthy[controllerInstallation.Name] = "not all required conditions found in ControllerInstallation"
		}
	}

	bldr, err := helper.NewConditionBuilder(gardencorev1beta1.SeedExtensionsReady)
	if err != nil {
		return err
	}

	if c := helper.GetCondition(seed.Status.Conditions, gardencorev1beta1.SeedExtensionsReady); c != nil {
		bldr.WithOldCondition(*c)
	}

	switch {
	case len(notValid) != 0:
		bldr.
			WithStatus(gardencorev1beta1.ConditionFalse).
			WithReason("NotAllExtensionsValid").
			WithMessage(fmt.Sprintf("Some extensions are not valid: %+v", notValid))

	case len(notInstalled) != 0:
		bldr.
			WithStatus(gardencorev1beta1.ConditionFalse).
			WithReason("NotAllExtensionsInstalled").
			WithMessage(fmt.Sprintf("Some extensions are not installed: +%v", notInstalled))

	case len(notHealthy) != 0:
		bldr.
			WithStatus(gardencorev1beta1.ConditionFalse).
			WithReason("NotAllExtensionsHealthy").
			WithMessage(fmt.Sprintf("Some extensions are not healthy: +%v", notHealthy))

	default:
		bldr.
			WithStatus(gardencorev1beta1.ConditionTrue).
			WithReason("AllExtensionsReady").
			WithMessage("All extensions installed into the seed cluster are ready and healthy.")
	}

	newCondition, update := bldr.WithNowFunc(c.nowFunc).Build()
	if !update {
		return nil
	}
	seed.Status.Conditions = helper.MergeConditions(seed.Status.Conditions, newCondition)

	_, err = gardenClient.GardenCore().CoreV1beta1().Seeds().UpdateStatus(ctx, seed, kubernetes.DefaultUpdateOptions())
	return err
}

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

package shoot

import (
	"fmt"
	"sync"
	"time"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardencorev1alpha1helper "github.com/gardener/gardener/pkg/apis/core/v1alpha1/helper"
	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	gardeninformers "github.com/gardener/gardener/pkg/client/garden/informers/externalversions/garden/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/operation"
	botanistpkg "github.com/gardener/gardener/pkg/operation/botanist"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"
)

func (c *Controller) shootCareAdd(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		return
	}
	c.shootCareQueue.Add(key)
}

func (c *Controller) reconcileShootCareKey(key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}
	shoot, err := c.shootLister.Shoots(namespace).Get(name)
	if apierrors.IsNotFound(err) {
		logger.Logger.Infof("[SHOOT CARE] Stopping care operations for Shoot %s since it has been deleted", key)
		c.shootCareQueue.Done(key)
		return nil
	}
	if err != nil {
		logger.Logger.Infof("[SHOOT CARE] %s - unable to retrieve object from store: %v", key, err)
		return err
	}

	if err := c.careControl.Care(shoot, key); err != nil {
		return err
	}

	c.shootCareQueue.AddAfter(key, c.config.Controllers.ShootCare.SyncPeriod.Duration)
	return nil
}

// CareControlInterface implements the control logic for caring for Shoots. It is implemented as an interface to allow
// for extensions that provide different semantics. Currently, there is only one implementation.
type CareControlInterface interface {
	Care(shoot *gardenv1beta1.Shoot, key string) error
}

// NewDefaultCareControl returns a new instance of the default implementation CareControlInterface that
// implements the documented semantics for caring for Shoots. updater is the UpdaterInterface used
// to update the status of Shoots. You should use an instance returned from NewDefaultCareControl() for any
// scenario other than testing.
func NewDefaultCareControl(k8sGardenClient kubernetes.Interface, k8sGardenInformers gardeninformers.Interface, secrets map[string]*corev1.Secret, imageVector imagevector.ImageVector, identity *gardenv1beta1.Gardener, config *config.ControllerManagerConfiguration) CareControlInterface {
	return &defaultCareControl{k8sGardenClient, k8sGardenInformers, secrets, imageVector, identity, config}
}

type defaultCareControl struct {
	k8sGardenClient    kubernetes.Interface
	k8sGardenInformers gardeninformers.Interface
	secrets            map[string]*corev1.Secret
	imageVector        imagevector.ImageVector
	identity           *gardenv1beta1.Gardener
	config             *config.ControllerManagerConfiguration
}

func (c *defaultCareControl) conditionThresholdsToProgressingMapping() map[gardencorev1alpha1.ConditionType]time.Duration {
	out := make(map[gardencorev1alpha1.ConditionType]time.Duration)
	for _, threshold := range c.config.Controllers.ShootCare.ConditionThresholds {
		out[gardencorev1alpha1.ConditionType(threshold.Type)] = threshold.Duration.Duration
	}
	return out
}

func shootClientInitializer(b *botanistpkg.Botanist) func() error {
	var (
		once sync.Once
		err  error
	)
	return func() error {
		once.Do(func() {
			err = b.InitializeShootClients()
		})
		return err
	}
}

func (c *defaultCareControl) Care(shootObj *gardenv1beta1.Shoot, key string) error {
	var (
		shoot       = shootObj.DeepCopy()
		shootLogger = logger.NewShootLogger(logger.Logger, shoot.Name, shoot.Namespace, "")
	)
	shootLogger.Debugf("[SHOOT CARE] %s", key)

	operation, err := operation.New(shoot, shootLogger, c.k8sGardenClient, c.k8sGardenInformers, c.identity, c.secrets, c.imageVector, nil)
	if err != nil {
		shootLogger.Errorf("could not initialize a new operation: %s", err.Error())
		return nil // We do not want to run in the exponential backoff for the condition checks.
	}

	// Initialize conditions based on the current status.
	var (
		newConditions                    = gardencorev1alpha1helper.MergeConditions(shoot.Status.Conditions, gardencorev1alpha1helper.InitCondition(gardenv1beta1.ShootAPIServerAvailable), gardencorev1alpha1helper.InitCondition(gardenv1beta1.ShootControlPlaneHealthy), gardencorev1alpha1helper.InitCondition(gardenv1beta1.ShootEveryNodeReady), gardencorev1alpha1helper.InitCondition(gardenv1beta1.ShootSystemComponentsHealthy))
		conditionAPIServerAvailable      = newConditions[0]
		conditionControlPlaneHealthy     = newConditions[1]
		conditionEveryNodeReady          = newConditions[2]
		conditionSystemComponentsHealthy = newConditions[3]
	)

	botanist, err := botanistpkg.New(operation)
	if err != nil {
		message := fmt.Sprintf("Failed to create a botanist object to perform the care operations (%s).", err.Error())
		conditionAPIServerAvailable = gardencorev1alpha1helper.UpdatedConditionUnknownErrorMessage(conditionAPIServerAvailable, message)
		conditionControlPlaneHealthy = gardencorev1alpha1helper.UpdatedConditionUnknownErrorMessage(conditionControlPlaneHealthy, message)
		conditionEveryNodeReady = gardencorev1alpha1helper.UpdatedConditionUnknownErrorMessage(conditionEveryNodeReady, message)
		conditionSystemComponentsHealthy = gardencorev1alpha1helper.UpdatedConditionUnknownErrorMessage(conditionSystemComponentsHealthy, message)
		operation.Logger.Error(message)

		c.updateShootConditions(shoot, conditionAPIServerAvailable, conditionControlPlaneHealthy, conditionEveryNodeReady, conditionSystemComponentsHealthy)
		return nil // We do not want to run in the exponential backoff for the condition checks.
	}

	initializeShootClients := shootClientInitializer(botanist)

	// Trigger garbage collection
	go garbageCollection(initializeShootClients, botanist)

	// Trigger health check
	conditionAPIServerAvailable, conditionControlPlaneHealthy, conditionEveryNodeReady, conditionSystemComponentsHealthy = botanist.HealthChecks(
		initializeShootClients,
		c.conditionThresholdsToProgressingMapping(),
		conditionAPIServerAvailable,
		conditionControlPlaneHealthy,
		conditionEveryNodeReady,
		conditionSystemComponentsHealthy,
	)

	// Update Shoot status
	shoot, err = c.updateShootConditions(shoot, conditionAPIServerAvailable, conditionControlPlaneHealthy, conditionEveryNodeReady, conditionSystemComponentsHealthy)
	if err != nil {
		botanist.Logger.Errorf("Could not update Shoot conditions: %+v", err)
		return nil // We do not want to run in the exponential backoff for the condition checks.
	}

	// Mark Shoot as healthy/unhealthy
	kutil.TryUpdateShootLabels(
		c.k8sGardenClient.Garden(),
		retry.DefaultBackoff, shoot.ObjectMeta,
		StatusLabelTransform(
			ComputeStatus(
				shoot.Status.LastOperation,
				shoot.Status.LastError,
				conditionAPIServerAvailable,
				conditionControlPlaneHealthy,
				conditionEveryNodeReady,
				conditionSystemComponentsHealthy,
			),
		),
	)
	return nil // We do not want to run in the exponential backoff for the condition checks.
}

func (c *defaultCareControl) updateShootConditions(shoot *gardenv1beta1.Shoot, conditions ...gardencorev1alpha1.Condition) (*gardenv1beta1.Shoot, error) {
	newShoot, err := kutil.TryUpdateShootConditions(c.k8sGardenClient.Garden(), retry.DefaultBackoff, shoot.ObjectMeta,
		func(shoot *gardenv1beta1.Shoot) (*gardenv1beta1.Shoot, error) {
			shoot.Status.Conditions = conditions
			return shoot, nil
		})

	return newShoot, err
}

// garbageCollection cleans the Seed and the Shoot cluster from no longer required
// objects. It receives a Garden object <garden> which stores the Shoot object.
func garbageCollection(initShootClients func() error, botanist *botanistpkg.Botanist) {
	var (
		qualifiedShootName = fmt.Sprintf("%s/%s", botanist.Shoot.Info.Namespace, botanist.Shoot.Info.Name)
		wg                 sync.WaitGroup
	)

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := botanist.PerformGarbageCollectionSeed(); err != nil {
			botanist.Logger.Errorf("Error during seed garbage collection: %+v", err)
		}
	}()

	if !botanist.Shoot.IsHibernated {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := initShootClients(); err != nil {
				botanist.Logger.Errorf("Could not initialize Shoot client for garbage collection of shoot %s: %+v", qualifiedShootName, err)
				return
			}
			if err := botanist.PerformGarbageCollectionShoot(); err != nil {
				botanist.Logger.Errorf("Error during shoot garbage collection: %+v", err)
			}
		}()
	}

	wg.Wait()
	botanist.Logger.Debugf("Successfully performed full garbage collection for Shoot cluster %s", qualifiedShootName)
}

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

	"github.com/gardener/gardener/pkg/apis/componentconfig"
	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/apis/garden/v1beta1/helper"
	gardeninformers "github.com/gardener/gardener/pkg/client/garden/informers/externalversions/garden/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/operation"
	botanistpkg "github.com/gardener/gardener/pkg/operation/botanist"
	"github.com/gardener/gardener/pkg/operation/cloudbotanist"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/cache"
)

func (c *Controller) shootCareAdd(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		return
	}
	c.shootCareQueue.AddAfter(key, c.config.Controllers.ShootCare.SyncPeriod.Duration)
}

func (c *Controller) shootCareDelete(obj interface{}) {
	shoot, ok := obj.(*gardenv1beta1.Shoot)
	if shoot == nil || !ok {
		return
	}
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		return
	}
	c.shootCareQueue.Done(key)
}

func (c *Controller) reconcileShootCareKey(key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}
	shoot, err := c.shootLister.Shoots(namespace).Get(name)
	if apierrors.IsNotFound(err) {
		logger.Logger.Debugf("[SHOOT CARE] %s - skipping because Shoot has been deleted", key)
		return nil
	}
	if err != nil {
		logger.Logger.Infof("[SHOOT CARE] %s - unable to retrieve object from store: %v", key, err)
		return err
	}

	defer c.shootCareAdd(shoot)

	if operationOngoing(shoot) {
		logger.Logger.Debugf("[SHOOT CARE] %s - skipping because an operation in ongoing", key)
		return nil
	}

	// Either ignore Shoots which are marked as to-be-ignored or execute care operations.
	if mustIgnoreShoot(shoot.Annotations, c.config.Controllers.Shoot.RespectSyncPeriodOverwrite) {
		logger.Logger.Infof("[SHOOT CARE] %s - skipping because Shoot is marked as 'to-be-ignored'.", key)
		return nil
	}

	return c.careControl.Care(shoot, key)
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
func NewDefaultCareControl(k8sGardenClient kubernetes.Client, k8sGardenInformers gardeninformers.Interface, secrets map[string]*corev1.Secret, imageVector imagevector.ImageVector, identity *gardenv1beta1.Gardener, config *componentconfig.ControllerManagerConfiguration, updater UpdaterInterface) CareControlInterface {
	return &defaultCareControl{k8sGardenClient, k8sGardenInformers, secrets, imageVector, identity, config, updater}
}

type defaultCareControl struct {
	k8sGardenClient    kubernetes.Client
	k8sGardenInformers gardeninformers.Interface
	secrets            map[string]*corev1.Secret
	imageVector        imagevector.ImageVector
	identity           *gardenv1beta1.Gardener
	config             *componentconfig.ControllerManagerConfiguration
	updater            UpdaterInterface
}

func (c *defaultCareControl) Care(shootObj *gardenv1beta1.Shoot, key string) error {
	var (
		shoot       = shootObj.DeepCopy()
		shootLogger = logger.NewShootLogger(logger.Logger, shoot.Name, shoot.Namespace, "")
	)
	shootLogger.Debugf("[SHOOT CARE] %s", key)

	operation, err := operation.New(shoot, shootLogger, c.k8sGardenClient, c.k8sGardenInformers, c.identity, c.secrets, c.imageVector)
	if err != nil {
		shootLogger.Errorf("could not initialize a new operation: %s", err.Error())
		return nil
	}

	// Initialize conditions based on the current status.
	var (
		newConditions                    = helper.NewConditions(shoot.Status.Conditions, gardenv1beta1.ShootControlPlaneHealthy, gardenv1beta1.ShootEveryNodeReady, gardenv1beta1.ShootSystemComponentsHealthy)
		conditionControlPlaneHealthy     = newConditions[0]
		conditionEveryNodeReady          = newConditions[1]
		conditionSystemComponentsHealthy = newConditions[2]
	)

	botanist, err := botanistpkg.New(operation)
	if err != nil {
		message := fmt.Sprintf("Failed to create a botanist object to perform the care operations (%s).", err.Error())
		conditionControlPlaneHealthy = helper.ModifyCondition(conditionControlPlaneHealthy, corev1.ConditionUnknown, gardenv1beta1.ConditionCheckError, message)
		conditionEveryNodeReady = helper.ModifyCondition(conditionEveryNodeReady, corev1.ConditionUnknown, gardenv1beta1.ConditionCheckError, message)
		conditionSystemComponentsHealthy = helper.ModifyCondition(conditionSystemComponentsHealthy, corev1.ConditionUnknown, gardenv1beta1.ConditionCheckError, message)
		operation.Logger.Error(message)
		c.updateShootStatus(shoot, *conditionControlPlaneHealthy, *conditionEveryNodeReady, *conditionSystemComponentsHealthy)
		return nil
	}
	cloudBotanist, err := cloudbotanist.New(operation, common.CloudPurposeShoot)
	if err != nil {
		message := fmt.Sprintf("Failed to create a Cloud Botanist to perform the care operations (%s).", err.Error())
		conditionControlPlaneHealthy = helper.ModifyCondition(conditionControlPlaneHealthy, corev1.ConditionUnknown, gardenv1beta1.ConditionCheckError, message)
		conditionEveryNodeReady = helper.ModifyCondition(conditionEveryNodeReady, corev1.ConditionUnknown, gardenv1beta1.ConditionCheckError, message)
		conditionSystemComponentsHealthy = helper.ModifyCondition(conditionSystemComponentsHealthy, corev1.ConditionUnknown, gardenv1beta1.ConditionCheckError, message)
		operation.Logger.Error(message)
		c.updateShootStatus(shoot, *conditionControlPlaneHealthy, *conditionEveryNodeReady, *conditionSystemComponentsHealthy)
		return nil
	}
	if err := botanist.InitializeShootClients(); err != nil {
		message := fmt.Sprintf("Failed to create a K8SClient for the Shoot cluster to perform the care operations (%s).", err.Error())
		conditionEveryNodeReady = helper.ModifyCondition(conditionEveryNodeReady, corev1.ConditionUnknown, gardenv1beta1.ConditionCheckError, message)
		conditionSystemComponentsHealthy = helper.ModifyCondition(conditionSystemComponentsHealthy, corev1.ConditionUnknown, gardenv1beta1.ConditionCheckError, message)
		operation.Logger.Error(message)
		c.updateShootStatus(shoot, *conditionControlPlaneHealthy, *conditionEveryNodeReady, *conditionSystemComponentsHealthy)
		return nil
	}

	// Trigger garbage collection
	garbageCollection(botanist)

	// Trigger health check
	conditionControlPlaneHealthy, conditionEveryNodeReady, conditionSystemComponentsHealthy = healthCheck(botanist, cloudBotanist, conditionControlPlaneHealthy, conditionEveryNodeReady, conditionSystemComponentsHealthy)

	// Update Shoot status
	if newShoot, _ := c.updateShootStatus(shoot, *conditionControlPlaneHealthy, *conditionEveryNodeReady, *conditionSystemComponentsHealthy); newShoot != nil {
		shoot = newShoot
	}

	// Mark Shoot as healthy/unhealthy
	var (
		lastOperation = shoot.Status.LastOperation
		lastError     = shoot.Status.LastError
		healthy       = lastOperation == nil || (lastOperation.State == gardenv1beta1.ShootLastOperationStateSucceeded && lastError == nil && conditionControlPlaneHealthy.Status == corev1.ConditionTrue && conditionEveryNodeReady.Status == corev1.ConditionTrue && conditionSystemComponentsHealthy.Status == corev1.ConditionTrue)
	)
	c.labelShoot(shoot, healthy)

	return nil
}

func (c *defaultCareControl) updateShootStatus(shoot *gardenv1beta1.Shoot, conditions ...gardenv1beta1.Condition) (*gardenv1beta1.Shoot, error) {
	if !helper.ConditionsNeedUpdate(shoot.Status.Conditions, conditions) {
		return shoot, nil
	}

	shoot.Status.Conditions = conditions

	newShoot, err := c.updater.UpdateShootStatusIfNoOperation(shoot)
	if err != nil {
		logger.Logger.Errorf("Could not update the Shoot status: %+v", err)
	}

	return newShoot, err
}

func (c *defaultCareControl) labelShoot(shoot *gardenv1beta1.Shoot, healthy bool) error {
	_, err := c.updater.UpdateShootLabels(shoot, computeLabelsWithShootHealthiness(healthy))
	if err != nil {
		logger.Logger.Errorf("Could not update the Shoot metadata: %s", err.Error())
	}
	return err
}

// garbageCollection cleans the Seed and the Shoot cluster from unrequired objects.
// It receives a Garden object <garden> which stores the Shoot object.
func garbageCollection(botanist *botanistpkg.Botanist) {
	var wg sync.WaitGroup

	wg.Add(2)
	go func() {
		defer wg.Done()
		botanist.PerformGarbageCollectionSeed()
	}()
	go func() {
		defer wg.Done()
		botanist.PerformGarbageCollectionShoot()
	}()
	wg.Wait()

	botanist.Logger.Debugf("Successfully performed garbage collection for Shoot cluster '%s'", botanist.Shoot.Info.Name)
}

// healthCheck performs several health checks and updates the status conditions.
// It receives a Garden object <garden> which stores the Shoot object.
// The current Health check verifies that the control plane running in the Seed cluster is healthy, every
// node is ready and that all system components (pods running kube-system) are healthy.
func healthCheck(botanist *botanistpkg.Botanist, cloudBotanist cloudbotanist.CloudBotanist, conditionControlPlaneHealthy, conditionEveryNodeReady, conditionSystemComponentsHealthy *gardenv1beta1.Condition) (*gardenv1beta1.Condition, *gardenv1beta1.Condition, *gardenv1beta1.Condition) {
	var wg sync.WaitGroup

	wg.Add(3)
	go func() {
		defer wg.Done()
		conditionControlPlaneHealthy = botanist.CheckConditionControlPlaneHealthy(conditionControlPlaneHealthy)
	}()
	go func() {
		defer wg.Done()
		conditionEveryNodeReady = botanist.CheckConditionEveryNodeReady(conditionEveryNodeReady)
	}()
	go func() {
		defer wg.Done()
		conditionSystemComponentsHealthy = botanist.CheckConditionSystemComponentsHealthy(conditionSystemComponentsHealthy)
	}()
	wg.Wait()

	botanist.Logger.Debugf("Successfully performed health check for Shoot cluster '%s'", botanist.Shoot.Info.Name)
	return conditionControlPlaneHealthy, conditionEveryNodeReady, conditionSystemComponentsHealthy
}

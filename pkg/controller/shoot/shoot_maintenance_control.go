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
	"time"

	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/apis/garden/v1beta1/helper"
	gardeninformers "github.com/gardener/gardener/pkg/client/garden/informers/externalversions/garden/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/operation"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
)

func (c *Controller) shootMaintenanceAdd(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		return
	}
	c.shootMaintenanceQueue.AddAfter(key, c.config.Controllers.ShootMaintenance.SyncPeriod.Duration)
}

func (c *Controller) shootMaintenanceDelete(obj interface{}) {
	shoot, ok := obj.(*gardenv1beta1.Shoot)
	if shoot == nil || !ok {
		return
	}
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		return
	}
	c.shootMaintenanceQueue.Done(key)
}

func (c *Controller) reconcileShootMaintenanceKey(key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}
	shoot, err := c.shootLister.Shoots(namespace).Get(name)
	if apierrors.IsNotFound(err) {
		logger.Logger.Debugf("[SHOOT MAINTENANCE] %s - skipping because Shoot has been deleted", key)
		return nil
	}
	if err != nil {
		logger.Logger.Infof("[SHOOT MAINTENANCE] %s - unable to retrieve object from store: %v", key, err)
		return err
	}
	if shoot.DeletionTimestamp != nil {
		logger.Logger.Debugf("[SHOOT MAINTENANCE] %s - skipping because Shoot is marked as to be deleted", key)
		return nil
	}
	defer c.shootMaintenanceAdd(shoot)
	return c.maintenanceControl.Maintain(shoot, key)
}

// MaintenanceControlInterface implements the control logic for maintaining Shoots. It is implemented as an interface to allow
// for extensions that provide different semantics. Currently, there is only one implementation.
type MaintenanceControlInterface interface {
	Maintain(shoot *gardenv1beta1.Shoot, key string) error
}

// NewDefaultMaintenanceControl returns a new instance of the default implementation MaintenanceControlInterface that
// implements the documented semantics for maintaining Shoots. updater is the UpdaterInterface used
// to update the spec of Shoots. You should use an instance returned from NewDefaultMaintenanceControl() for any
// scenario other than testing.
func NewDefaultMaintenanceControl(k8sGardenClient kubernetes.Client, k8sGardenInformers gardeninformers.Interface, secrets map[string]*corev1.Secret, imageVector imagevector.ImageVector, identity *gardenv1beta1.Gardener, recorder record.EventRecorder, updater UpdaterInterface) MaintenanceControlInterface {
	return &defaultMaintenanceControl{k8sGardenClient, k8sGardenInformers, secrets, imageVector, identity, recorder, updater}
}

type defaultMaintenanceControl struct {
	k8sGardenClient    kubernetes.Client
	k8sGardenInformers gardeninformers.Interface
	secrets            map[string]*corev1.Secret
	imageVector        imagevector.ImageVector
	identity           *gardenv1beta1.Gardener
	recorder           record.EventRecorder
	updater            UpdaterInterface
}

func (c *defaultMaintenanceControl) Maintain(shootObj *gardenv1beta1.Shoot, key string) error {
	var (
		operationID = utils.GenerateRandomString(8)
		shoot       = shootObj.DeepCopy()
		shootLogger = logger.NewShootLogger(logger.Logger, shoot.Name, shoot.Namespace, operationID)
		handleError = func(msg string) {
			c.recorder.Eventf(shoot, corev1.EventTypeWarning, gardenv1beta1.ShootEventMaintenanceError, "[%s] %s", operationID, msg)
			shootLogger.Error(msg)
		}
	)

	maintenanceWindowBegin, err := utils.ParseMaintenanceTime(shoot.Spec.Maintenance.TimeWindow.Begin)
	if err != nil {
		handleError(fmt.Sprintf("Could not parse the maintenance time window begin value: %s", err.Error()))
		return nil
	}
	maintenanceWindowEnd, err := utils.ParseMaintenanceTime(shoot.Spec.Maintenance.TimeWindow.End)
	if err != nil {
		handleError(fmt.Sprintf("Could not parse the maintenance time window end value: %s", err.Error()))
		return nil
	}
	now, err := utils.ParseMaintenanceTime(utils.FormatMaintenanceTime(time.Now()))
	if err != nil {
		handleError(fmt.Sprintf("Could not parse the current time into the maintenance format: %s", err.Error()))
		return nil
	}

	// Check if the current time is between the begin and the end of the maintenance time window.
	// Only in this case we want to perform maintenance operations.
	if now.After(maintenanceWindowBegin) && now.Before(maintenanceWindowEnd) {
		shootLogger.Infof("[SHOOT MAINTENANCE] %s", key)

		operation, err := operation.New(shoot, shootLogger, c.k8sGardenClient, c.k8sGardenInformers, c.identity, c.secrets, c.imageVector)
		if err != nil {
			handleError(fmt.Sprintf("Could not initialize a new operation: %s", err.Error()))
			return nil
		}

		// Check if the CloudProfile contains another version of the machine image.
		machineImageFound, machineImage, err := helper.DetermineMachineImage(*operation.Shoot.CloudProfile, operation.Shoot.GetMachineImageName(), shoot.Spec.Cloud.Region)
		if err != nil {
			handleError(fmt.Sprintf("Failure while determining the machine image in the CloudProfile: %s", err.Error()))
			return nil
		}
		if machineImageFound {
			switch operation.Shoot.CloudProvider {
			case gardenv1beta1.CloudProviderAWS:
				image := machineImage.(*gardenv1beta1.AWSMachineImage)
				shoot.Spec.Cloud.AWS.MachineImage = image
			case gardenv1beta1.CloudProviderAzure:
				image := machineImage.(*gardenv1beta1.AzureMachineImage)
				shoot.Spec.Cloud.Azure.MachineImage = image
			case gardenv1beta1.CloudProviderGCP:
				image := machineImage.(*gardenv1beta1.GCPMachineImage)
				shoot.Spec.Cloud.GCP.MachineImage = image
			case gardenv1beta1.CloudProviderOpenStack:
				image := machineImage.(*gardenv1beta1.OpenStackMachineImage)
				shoot.Spec.Cloud.OpenStack.MachineImage = image
			}
		}

		// Check if the CloudProfile contains a newer Kubernetes patch version.
		if shoot.Spec.Maintenance.AutoUpdate.KubernetesVersion {
			newerPatchVersionFound, latestPatchVersion, err := helper.DetermineLatestKubernetesVersion(*operation.Shoot.CloudProfile, operation.Shoot.Info.Spec.Kubernetes.Version)
			if err != nil {
				handleError(fmt.Sprintf("Failure while determining the latest Kubernetes patch version in the CloudProfile: %s", err.Error()))
				return nil
			}
			if newerPatchVersionFound {
				shoot.Spec.Kubernetes.Version = latestPatchVersion
			}
		}

		// Update the Shoot resource object.
		if _, err := c.updater.UpdateShoot(shoot); err != nil {
			handleError(fmt.Sprintf("Could not update the Shoot specification: %s", err.Error()))
			return nil
		}
		msg := "Completed; updated the Shoot specification successfully."
		shootLogger.Infof("[SHOOT MAINTENANCE] %s", msg)
		c.recorder.Eventf(shoot, corev1.EventTypeNormal, gardenv1beta1.ShootEventMaintenanceDone, "[%s] %s", operationID, msg)
	}

	return nil
}

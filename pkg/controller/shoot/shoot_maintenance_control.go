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
	controllerutils "github.com/gardener/gardener/pkg/controller/utils"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/operation"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
)

const (
	// BestGuessMaintenanceMinutes is the average duration of a maintenance operation. It is subtracted from the end
	// of a maintenance time window to use a best-effort kind of finishing the operation before the end.
	// Generally, we can't make sure that the maintenance operation is done by the end of the time window anyway (considering large
	// clusters with hundreds of nodes, a rolling update will take several hours).
	BestGuessMaintenanceMinutes = 15
)

func (c *Controller) shootMaintenanceAdd(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		return
	}
	c.shootMaintenanceQueue.Add(key)
}

func (c *Controller) shootMaintenanceUpdate(oldObj, newObj interface{}) {
	newShoot, ok1 := newObj.(*gardenv1beta1.Shoot)
	oldShoot, ok2 := oldObj.(*gardenv1beta1.Shoot)
	if !ok1 || !ok2 {
		return
	}

	if hasMaintainNowAnnotation(newShoot) || !apiequality.Semantic.DeepEqual(oldShoot.Spec.Maintenance.TimeWindow, newShoot.Spec.Maintenance.TimeWindow) {
		c.shootMaintenanceAdd(newObj)
	}
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
		logger.Logger.Errorf("[SHOOT MAINTENANCE] %s - unable to retrieve object from store: %v", key, err)
		return err
	}
	if shoot.DeletionTimestamp != nil {
		logger.Logger.Debugf("[SHOOT MAINTENANCE] %s - skipping because Shoot is marked as to be deleted", key)
		return nil
	}

	maintenanceTimeWindow, err := utils.ParseMaintenanceTimeWindow(shoot.Spec.Maintenance.TimeWindow.Begin, shoot.Spec.Maintenance.TimeWindow.End)
	if err != nil {
		logger.Logger.Errorf("[SHOOT MAINTENANCE] %s - invalid time window: %v", key, err)
		return err
	}
	maintenanceTimeWindow = maintenanceTimeWindow.WithEnd(maintenanceTimeWindow.End().Add(0, -BestGuessMaintenanceMinutes, 0))

	var now = time.Now().UTC()

	defer c.shootMaintenanceRequeue(key, maintenanceTimeWindow, now)

	if mustIgnoreShoot(shoot.Annotations, c.config.Controllers.Shoot.RespectSyncPeriodOverwrite) || !mustMaintainNow(shoot, maintenanceTimeWindow, now) {
		logger.Logger.Infof("[SHOOT MAINTENANCE] %s - skipping because Shoot (it is either marked as 'to-be-ignored' or must not be maintained now).", key)
		return nil
	}

	return c.maintenanceControl.Maintain(shoot, key)
}

// newRandomTimeWindow computes a new random time window either for today or the next day (depending on <today>).
func (c *Controller) shootMaintenanceRequeue(key string, maintenanceTimeWindow *utils.MaintenanceTimeWindow, now time.Time) {
	var (
		duration        = maintenanceTimeWindow.RandomDurationUntilNext(now)
		nextMaintenance = now.Add(duration)
	)
	logger.Logger.Infof("[SHOOT MAINTENANCE] %s - Scheduled maintenance in %s at %s", key, duration, nextMaintenance.UTC())
	c.shootMaintenanceQueue.AddAfter(key, duration)
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
func NewDefaultMaintenanceControl(k8sGardenClient kubernetes.Interface, k8sGardenInformers gardeninformers.Interface, secrets map[string]*corev1.Secret, imageVector imagevector.ImageVector, identity *gardenv1beta1.Gardener, recorder record.EventRecorder) MaintenanceControlInterface {
	return &defaultMaintenanceControl{k8sGardenClient, k8sGardenInformers, secrets, imageVector, identity, recorder}
}

type defaultMaintenanceControl struct {
	k8sGardenClient    kubernetes.Interface
	k8sGardenInformers gardeninformers.Interface
	secrets            map[string]*corev1.Secret
	imageVector        imagevector.ImageVector
	identity           *gardenv1beta1.Gardener
	recorder           record.EventRecorder
}

func (c *defaultMaintenanceControl) Maintain(shootObj *gardenv1beta1.Shoot, key string) error {
	operationID, err := utils.GenerateRandomString(8)
	if err != nil {
		return err
	}

	var (
		shoot       = shootObj.DeepCopy()
		shootLogger = logger.NewShootLogger(logger.Logger, shoot.Name, shoot.Namespace, operationID)
		handleError = func(msg string) {
			c.recorder.Eventf(shoot, corev1.EventTypeWarning, gardenv1beta1.ShootEventMaintenanceError, "[%s] %s", operationID, msg)
			shootLogger.Error(msg)
		}
	)

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

	var updateMachineImage func(s *gardenv1beta1.Cloud)
	if machineImageFound {
		switch operation.Shoot.CloudProvider {
		case gardenv1beta1.CloudProviderAWS:
			image := machineImage.(*gardenv1beta1.AWSMachineImage)
			updateMachineImage = func(s *gardenv1beta1.Cloud) { s.AWS.MachineImage = image }
		case gardenv1beta1.CloudProviderAzure:
			image := machineImage.(*gardenv1beta1.AzureMachineImage)
			updateMachineImage = func(s *gardenv1beta1.Cloud) { s.Azure.MachineImage = image }
		case gardenv1beta1.CloudProviderGCP:
			image := machineImage.(*gardenv1beta1.GCPMachineImage)
			updateMachineImage = func(s *gardenv1beta1.Cloud) { s.GCP.MachineImage = image }
		case gardenv1beta1.CloudProviderOpenStack:
			image := machineImage.(*gardenv1beta1.OpenStackMachineImage)
			updateMachineImage = func(s *gardenv1beta1.Cloud) { s.OpenStack.MachineImage = image }
		}
	}

	// Check if the CloudProfile contains a newer Kubernetes patch version.
	var updateKubernetesVersion func(s *gardenv1beta1.Kubernetes)
	if shoot.Spec.Maintenance.AutoUpdate.KubernetesVersion {
		newerPatchVersionFound, latestPatchVersion, err := helper.DetermineLatestKubernetesVersion(*operation.Shoot.CloudProfile, operation.Shoot.Info.Spec.Kubernetes.Version)
		if err != nil {
			handleError(fmt.Sprintf("Failure while determining the latest Kubernetes patch version in the CloudProfile: %s", err.Error()))
			return nil
		}
		if newerPatchVersionFound {
			updateKubernetesVersion = func(s *gardenv1beta1.Kubernetes) { s.Version = latestPatchVersion }
		}
	}

	// Update the Shoot resource object.
	_, err = kutil.TryUpdateShoot(c.k8sGardenClient.Garden(), retry.DefaultBackoff, shoot.ObjectMeta, func(s *gardenv1beta1.Shoot) (*gardenv1beta1.Shoot, error) {
		if !apiequality.Semantic.DeepEqual(shootObj.Spec.Maintenance.AutoUpdate, s.Spec.Maintenance.AutoUpdate) {
			return nil, fmt.Errorf("auto update section of Shoot %s/%s changed mid-air", s.Namespace, s.Name)
		}

		delete(s.Annotations, common.ShootOperation)

		controllerutils.AddTasks(s.Annotations, common.ShootTaskDeployInfrastructure, common.ShootTaskDeployKube2IAMResource)
		s.Annotations[common.ShootOperation] = common.ShootOperationReconcile

		if updateMachineImage != nil {
			updateMachineImage(&s.Spec.Cloud)
		}
		if updateKubernetesVersion != nil {
			updateKubernetesVersion(&s.Spec.Kubernetes)
		}
		return s, nil
	})
	if err != nil {
		handleError(fmt.Sprintf("Could not update the Shoot specification: %s", err.Error()))
		return nil
	}
	msg := "Completed; updated the Shoot specification successfully."
	shootLogger.Infof("[SHOOT MAINTENANCE] %s", msg)
	c.recorder.Eventf(shoot, corev1.EventTypeNormal, gardenv1beta1.ShootEventMaintenanceDone, "[%s] %s", operationID, msg)

	return nil
}

func mustMaintainNow(shoot *gardenv1beta1.Shoot, maintenanceTimeWindow *utils.MaintenanceTimeWindow, now time.Time) bool {
	return hasMaintainNowAnnotation(shoot) || maintenanceTimeWindow.Contains(now)
}

func hasMaintainNowAnnotation(shoot *gardenv1beta1.Shoot) bool {
	operation, ok := shoot.Annotations[common.ShootOperation]
	return ok && operation == common.ShootOperationMaintain
}

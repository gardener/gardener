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

	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"

	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/apis/garden/v1beta1/helper"
	gardeninformers "github.com/gardener/gardener/pkg/client/garden/informers/externalversions/garden/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	controllerutils "github.com/gardener/gardener/pkg/controllermanager/controller/utils"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/operation"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
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
	log := logger.NewShootLogger(logger.Logger, name, namespace)

	shoot, err := c.shootLister.Shoots(namespace).Get(name)
	if apierrors.IsNotFound(err) {
		log.Debugf("[SHOOT MAINTENANCE] - skipping because Shoot has been deleted")
		return nil
	}
	if err != nil {
		log.WithError(err).Error("[SHOOT MAINTENANCE] - unable to retrieve object from store")
		return err
	}
	if shoot.DeletionTimestamp != nil {
		log.Debug("[SHOOT MAINTENANCE] - skipping because Shoot is marked as to be deleted")
		return nil
	}

	defer c.shootMaintenanceRequeue(key, shoot)

	if common.ShouldIgnoreShoot(c.respectSyncPeriodOverwrite(), shoot) || !mustMaintainNow(shoot) {
		logger.Logger.Infof("[SHOOT MAINTENANCE] %s - skipping because Shoot (it is either marked as 'to-be-ignored' or must not be maintained now).", key)
		return nil
	}

	return c.maintenanceControl.Maintain(shoot, key)
}

// newRandomTimeWindow computes a new random time window either for today or the next day (depending on <today>).
func (c *Controller) shootMaintenanceRequeue(key string, shoot *gardenv1beta1.Shoot) {
	var (
		duration        = c.durationUntilNextShootSync(shoot)
		nextMaintenance = time.Now().Add(duration)
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
// implements the documented semantics for maintaining Shoots. You should use an instance returned from
// NewDefaultMaintenanceControl() for any scenario other than testing.
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
		shootLogger = logger.NewShootLogger(logger.Logger, shoot.Name, shoot.Namespace)
		handleError = func(msg string) {
			c.recorder.Eventf(shoot, corev1.EventTypeWarning, gardenv1beta1.ShootEventMaintenanceError, "[%s] %s", operationID, msg)
			shootLogger.Error(msg)
		}
	)

	shootLogger.Infof("[SHOOT MAINTENANCE] %s", key)

	operation, err := operation.New(shoot, &config.ControllerManagerConfiguration{}, shootLogger, c.k8sGardenClient, c.k8sGardenInformers, c.identity, c.secrets, c.imageVector, nil)
	if err != nil {
		handleError(fmt.Sprintf("Could not initialize a new operation: %s", err.Error()))
		return nil
	}

	defaultMachineImage, machineImages, err := MaintainMachineImages(operation.Shoot.Info, operation.Shoot.CloudProfile, operation.Shoot.GetDefaultMachineImage(), operation.Shoot.GetMachineImages())
	if err != nil {
		// continue execution to allow the kubernetes version update
		handleError(fmt.Sprintf("Could not maintain machine image version: %s", err.Error()))
	}

	var updateDefaultMachineImage func(s *gardenv1beta1.Cloud)
	if defaultMachineImage != nil {
		updateDefaultMachineImage = helper.UpdateDefaultMachineImage(operation.Shoot.CloudProvider, defaultMachineImage)
	}

	var updateWorkerMachineImages func(s *gardenv1beta1.Cloud)
	if len(machineImages) > 0 {
		updateWorkerMachineImages = helper.UpdateMachineImages(operation.Shoot.CloudProvider, machineImages)
	}

	updatedVersion, err := MaintainKubernetesVersion(operation.Shoot.Info, operation.Shoot.CloudProfile)
	if err != nil {
		// continue execution to allow the kubernetes version update
		handleError(fmt.Sprintf("Could not maintain kubernetes version: %s", err.Error()))
	}

	// Check if the CloudProfile contains a newer Kubernetes patch version.
	var updateKubernetesVersion func(s *gardenv1beta1.Kubernetes)
	if updatedVersion != nil {
		updateKubernetesVersion = func(s *gardenv1beta1.Kubernetes) { s.Version = *updatedVersion }
	}

	// Update the Shoot resource object.
	_, err = kutil.TryUpdateShoot(c.k8sGardenClient.Garden(), retry.DefaultBackoff, shoot.ObjectMeta, func(s *gardenv1beta1.Shoot) (*gardenv1beta1.Shoot, error) {
		if !apiequality.Semantic.DeepEqual(shootObj.Spec.Maintenance.AutoUpdate, s.Spec.Maintenance.AutoUpdate) {
			return nil, fmt.Errorf("auto update section of Shoot %s/%s changed mid-air", s.Namespace, s.Name)
		}

		delete(s.Annotations, common.ShootOperation)

		controllerutils.AddTasks(s.Annotations, common.ShootTaskDeployInfrastructure, common.ShootTaskDeployKube2IAMResource)
		s.Annotations[common.ShootOperation] = common.ShootOperationReconcile

		if updateDefaultMachineImage != nil {
			updateDefaultMachineImage(&s.Spec.Cloud)
		}
		if updateWorkerMachineImages != nil {
			updateWorkerMachineImages(&s.Spec.Cloud)
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

// MaintainKubernetesVersion determines if a shoots kubernetes version has to be maintained and in case returns the target version
func MaintainKubernetesVersion(shoot *gardenv1beta1.Shoot, profile *gardenv1beta1.CloudProfile) (*string, error) {
	shouldBeUpdated, err := shouldKubernetesVersionBeUpdated(shoot, profile)
	if err != nil {
		return nil, err
	}
	if shouldBeUpdated {
		newerPatchVersionFound, latestPatchVersion, err := helper.DetermineLatestKubernetesPatchVersion(*profile, shoot.Spec.Kubernetes.Version)
		if err != nil {
			return nil, fmt.Errorf("failure while determining the latest Kubernetes patch version in the CloudProfile: %s", err.Error())
		}
		if newerPatchVersionFound {
			return &latestPatchVersion, nil
		}
	}
	return nil, nil
}

func shouldKubernetesVersionBeUpdated(shoot *gardenv1beta1.Shoot, profile *gardenv1beta1.CloudProfile) (bool, error) {
	versionExistsInCloudProfile, offeredVersion, err := helper.KubernetesVersionExistsInCloudProfile(*profile, shoot.Spec.Kubernetes.Version)
	if err != nil {
		return false, err
	}

	return !versionExistsInCloudProfile || shoot.Spec.Maintenance.AutoUpdate.KubernetesVersion || ExpirationDateExpired(offeredVersion.ExpirationDate), nil
}

func mustMaintainNow(shoot *gardenv1beta1.Shoot) bool {
	return hasMaintainNowAnnotation(shoot) || common.IsNowInEffectiveShootMaintenanceTimeWindow(shoot)
}

func hasMaintainNowAnnotation(shoot *gardenv1beta1.Shoot) bool {
	operation, ok := shoot.Annotations[common.ShootOperation]
	return ok && operation == common.ShootOperationMaintain
}

// MaintainMachineImages determines if a shoots machine images have to be maintained and in case returns the target images
func MaintainMachineImages(shoot *gardenv1beta1.Shoot, cloudProfile *gardenv1beta1.CloudProfile, shootDefaultImage *gardenv1beta1.ShootMachineImage, shootCurrentImages []*gardenv1beta1.ShootMachineImage) (*gardenv1beta1.ShootMachineImage, []*gardenv1beta1.ShootMachineImage, error) {
	var defaultMachineImageForUpdate *gardenv1beta1.ShootMachineImage
	defaultMachineImageFromCloudProfile, err := determineMachineImage(cloudProfile, shootDefaultImage)
	if err != nil {
		return nil, nil, err
	}

	shouldBeUpdated, shootDefaultMachineImage, err := shouldMachineImageBeUpdated(shoot, &defaultMachineImageFromCloudProfile, shootDefaultImage)
	if err != nil {
		return nil, nil, err
	}

	if shouldBeUpdated {
		defaultMachineImageForUpdate = shootDefaultMachineImage
	}

	shootMachineImagesForUpdate := []*gardenv1beta1.ShootMachineImage{}
	for _, shootImage := range shootCurrentImages {
		machineImageFromCloudProfile, err := determineMachineImage(cloudProfile, shootImage)
		if err != nil {
			return nil, nil, err
		}

		shouldBeUpdated, shootMachineImage, err := shouldMachineImageBeUpdated(shoot, &machineImageFromCloudProfile, shootImage)
		if err != nil {
			return nil, nil, err
		}

		if shouldBeUpdated {
			shootMachineImagesForUpdate = append(shootMachineImagesForUpdate, shootMachineImage)
		}
	}

	return defaultMachineImageForUpdate, shootMachineImagesForUpdate, nil
}

func determineMachineImage(cloudProfile *gardenv1beta1.CloudProfile, shootMachineImage *gardenv1beta1.ShootMachineImage) (gardenv1beta1.MachineImage, error) {
	machineImagesFound, machineImageFromCloudProfile, err := helper.DetermineMachineImageForName(*cloudProfile, shootMachineImage.Name)
	if err != nil {
		return gardenv1beta1.MachineImage{}, fmt.Errorf("failure while determining the default machine image in the CloudProfile: %s", err.Error())
	}
	if !machineImagesFound {
		return gardenv1beta1.MachineImage{}, fmt.Errorf("failure while determining the default machine image in the CloudProfile: no machineImage with name '%s' (specified in shoot) could be found in the cloudProfile '%s'", shootMachineImage.Name, cloudProfile.Name)
	}

	return machineImageFromCloudProfile, nil
}

func shouldMachineImageBeUpdated(shoot *gardenv1beta1.Shoot, machineImage *gardenv1beta1.MachineImage, shootMachineImage *gardenv1beta1.ShootMachineImage) (bool, *gardenv1beta1.ShootMachineImage, error) {
	versionExistsInCloudProfile, _ := helper.ShootMachineImageVersionExists(*machineImage, *shootMachineImage)
	if !versionExistsInCloudProfile || *shoot.Spec.Maintenance.AutoUpdate.MachineImageVersion || ForceMachineImageUpdateRequired(shootMachineImage, *machineImage) {
		shootMachineImage, err := updateToLatestMachineImageVersion(*machineImage)
		if err != nil {
			return false, nil, fmt.Errorf("failure while updating machineImage to the latest version: %s", err.Error())
		}

		return true, shootMachineImage, nil
	}

	return false, nil, nil
}

// updateToLatestMachineImageVersion returns the latest machine image and requiring an image update
func updateToLatestMachineImageVersion(machineImage gardenv1beta1.MachineImage) (*gardenv1beta1.ShootMachineImage, error) {
	_, latestMachineImage, err := helper.GetShootMachineImageFromLatestMachineImageVersion(machineImage)
	if err != nil {
		return nil, fmt.Errorf("failed to determine latest machine image in cloud profile: %s", err.Error())
	}
	return &latestMachineImage, nil
}

// ForceMachineImageUpdateRequired checks if the shoots current machine image has to be forcefully updated
func ForceMachineImageUpdateRequired(shootCurrentImage *gardenv1beta1.ShootMachineImage, imageCloudProvider gardenv1beta1.MachineImage) bool {
	for _, image := range imageCloudProvider.Versions {
		if shootCurrentImage.Version != image.Version {
			continue
		}
		return ExpirationDateExpired(image.ExpirationDate)
	}
	return false
}

// ExpirationDateExpired returns if now is equal or after the given expirationDate
func ExpirationDateExpired(timestamp *metav1.Time) bool {
	if timestamp == nil {
		return false
	}
	return time.Now().UTC().After(timestamp.Time) || time.Now().UTC().Equal(timestamp.Time)
}

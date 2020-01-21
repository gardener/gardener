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

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/operation/common"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
)

func (c *Controller) shootMaintenanceAdd(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		return
	}
	c.shootMaintenanceQueue.Add(key)
}

func (c *Controller) shootMaintenanceUpdate(oldObj, newObj interface{}) {
	newShoot, ok1 := newObj.(*gardencorev1beta1.Shoot)
	oldShoot, ok2 := oldObj.(*gardencorev1beta1.Shoot)
	if !ok1 || !ok2 {
		return
	}

	if hasMaintainNowAnnotation(newShoot) || !apiequality.Semantic.DeepEqual(oldShoot.Spec.Maintenance.TimeWindow, newShoot.Spec.Maintenance.TimeWindow) {
		c.shootMaintenanceAdd(newObj)
	}
}

func (c *Controller) shootMaintenanceDelete(obj interface{}) {
	shoot, ok := obj.(*gardencorev1beta1.Shoot)
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

	if !mustMaintainNow(shoot) {
		logger.Logger.Infof("[SHOOT MAINTENANCE] %s - skipping because Shoot must not be maintained now.", key)
		return nil
	}

	return c.maintenanceControl.Maintain(shoot, key)
}

// newRandomTimeWindow computes a new random time window either for today or the next day (depending on <today>).
func (c *Controller) shootMaintenanceRequeue(key string, shoot *gardencorev1beta1.Shoot) {
	var (
		now             = time.Now()
		window          = common.EffectiveShootMaintenanceTimeWindow(shoot)
		duration        = window.RandomDurationUntilNext(now)
		nextMaintenance = time.Now().Add(duration)
	)

	logger.Logger.Infof("[SHOOT MAINTENANCE] %s - Scheduled maintenance in %s at %s", key, duration, nextMaintenance.UTC())
	c.shootMaintenanceQueue.AddAfter(key, duration)
}

// MaintenanceControlInterface implements the control logic for maintaining Shoots. It is implemented as an interface to allow
// for extensions that provide different semantics. Currently, there is only one implementation.
type MaintenanceControlInterface interface {
	Maintain(shoot *gardencorev1beta1.Shoot, key string) error
}

// NewDefaultMaintenanceControl returns a new instance of the default implementation MaintenanceControlInterface that
// implements the documented semantics for maintaining Shoots. You should use an instance returned from
// NewDefaultMaintenanceControl() for any scenario other than testing.
func NewDefaultMaintenanceControl(k8sGardenClient kubernetes.Interface, k8sGardenCoreInformers gardencoreinformers.Interface, recorder record.EventRecorder) MaintenanceControlInterface {
	return &defaultMaintenanceControl{k8sGardenClient, k8sGardenCoreInformers, recorder}
}

type defaultMaintenanceControl struct {
	k8sGardenClient        kubernetes.Interface
	k8sGardenCoreInformers gardencoreinformers.Interface
	recorder               record.EventRecorder
}

func (c *defaultMaintenanceControl) Maintain(shootObj *gardencorev1beta1.Shoot, key string) error {
	var (
		shoot       = shootObj.DeepCopy()
		shootLogger = logger.NewShootLogger(logger.Logger, shoot.Name, shoot.Namespace)
		handleError = func(msg string) {
			c.recorder.Eventf(shoot, corev1.EventTypeWarning, gardencorev1beta1.ShootEventMaintenanceError, "%s", msg)
			shootLogger.Error(msg)
		}
	)

	shootLogger.Infof("[SHOOT MAINTENANCE] %s", key)

	cloudProfile, err := c.k8sGardenCoreInformers.CloudProfiles().Lister().Get(shoot.Spec.CloudProfileName)
	if err != nil {
		return err
	}

	updatedMachineImages, err := MaintainMachineImages(shootObj, cloudProfile, gardencorev1beta1helper.GetMachineImagesFor(shoot))
	if err != nil {
		// continue execution to allow the kubernetes version update
		handleError(fmt.Sprintf("Could not maintain machine image version: %s", err.Error()))
	}

	updatedKubernetesVersion, err := MaintainKubernetesVersion(shootObj, cloudProfile)
	if err != nil {
		// continue execution to allow the machine image version update
		handleError(fmt.Sprintf("Could not maintain kubernetes version: %s", err.Error()))
	}

	// Update the Shoot resource object.
	_, err = kutil.TryUpdateShoot(c.k8sGardenClient.GardenCore(), retry.DefaultBackoff, shoot.ObjectMeta, func(s *gardencorev1beta1.Shoot) (*gardencorev1beta1.Shoot, error) {
		if !apiequality.Semantic.DeepEqual(shootObj.Spec.Maintenance.AutoUpdate, s.Spec.Maintenance.AutoUpdate) {
			return nil, fmt.Errorf("auto update section of Shoot %s/%s changed mid-air", s.Namespace, s.Name)
		}

		delete(s.Annotations, v1beta1constants.GardenerOperation)
		controllerutils.AddTasks(s.Annotations, common.ShootTaskDeployInfrastructure)
		s.Annotations[v1beta1constants.GardenerOperation] = common.ShootOperationReconcile

		if updatedMachineImages != nil {
			gardencorev1beta1helper.UpdateMachineImages(s.Spec.Provider.Workers, updatedMachineImages)
		}
		if updatedKubernetesVersion != nil {
			s.Spec.Kubernetes.Version = *updatedKubernetesVersion
		}

		return s, nil
	})
	if err != nil {
		handleError(fmt.Sprintf("Could not update the Shoot specification: %s", err.Error()))
		return err
	}
	msg := "Completed; updated the Shoot specification successfully."
	shootLogger.Infof("[SHOOT MAINTENANCE] %s", msg)
	c.recorder.Eventf(shoot, corev1.EventTypeNormal, gardencorev1beta1.ShootEventMaintenanceDone, "%s", msg)

	return nil
}

// MaintainKubernetesVersion determines if a shoots kubernetes version has to be maintained and in case returns the target version
func MaintainKubernetesVersion(shoot *gardencorev1beta1.Shoot, profile *gardencorev1beta1.CloudProfile) (*string, error) {
	shouldBeUpdated, err := shouldKubernetesVersionBeUpdated(shoot, profile)
	if err != nil {
		return nil, err
	}
	if shouldBeUpdated {
		newerPatchVersionFound, latestPatchVersion, err := gardencorev1beta1helper.DetermineLatestKubernetesPatchVersion(profile, shoot.Spec.Kubernetes.Version)
		if err != nil {
			return nil, fmt.Errorf("failure while determining the latest Kubernetes patch version in the CloudProfile: %s", err.Error())
		}
		if newerPatchVersionFound {
			return &latestPatchVersion, nil
		}
	}
	return nil, nil
}

func shouldKubernetesVersionBeUpdated(shoot *gardencorev1beta1.Shoot, profile *gardencorev1beta1.CloudProfile) (bool, error) {
	versionExistsInCloudProfile, offeredVersion, err := gardencorev1beta1helper.KubernetesVersionExistsInCloudProfile(profile, shoot.Spec.Kubernetes.Version)
	if err != nil {
		return false, err
	}

	if !versionExistsInCloudProfile && !shoot.Spec.Maintenance.AutoUpdate.KubernetesVersion {
		return false, nil
	}

	return shoot.Spec.Maintenance.AutoUpdate.KubernetesVersion || ExpirationDateExpired(offeredVersion.ExpirationDate), nil
}

func mustMaintainNow(shoot *gardencorev1beta1.Shoot) bool {
	return hasMaintainNowAnnotation(shoot) || common.IsNowInEffectiveShootMaintenanceTimeWindow(shoot)
}

func hasMaintainNowAnnotation(shoot *gardencorev1beta1.Shoot) bool {
	operation, ok := common.GetShootOperationAnnotation(shoot.Annotations)
	return ok && operation == common.ShootOperationMaintain
}

// MaintainMachineImages determines if a shoots machine images have to be maintained and in case returns the target images
func MaintainMachineImages(shoot *gardencorev1beta1.Shoot, cloudProfile *gardencorev1beta1.CloudProfile, shootCurrentImages []*gardencorev1beta1.ShootMachineImage) ([]*gardencorev1beta1.ShootMachineImage, error) {
	var shootMachineImagesForUpdate []*gardencorev1beta1.ShootMachineImage
	for _, shootImage := range shootCurrentImages {
		machineImageFromCloudProfile, err := determineMachineImage(cloudProfile, shootImage)
		if err != nil {
			return nil, err
		}

		shouldBeUpdated, shootMachineImage, err := shouldMachineImageBeUpdated(shoot, &machineImageFromCloudProfile, shootImage)
		if err != nil {
			return nil, err
		}

		if shouldBeUpdated {
			shootMachineImagesForUpdate = append(shootMachineImagesForUpdate, shootMachineImage)
		}
	}

	return shootMachineImagesForUpdate, nil
}

func determineMachineImage(cloudProfile *gardencorev1beta1.CloudProfile, shootMachineImage *gardencorev1beta1.ShootMachineImage) (gardencorev1beta1.MachineImage, error) {
	machineImagesFound, machineImageFromCloudProfile, err := gardencorev1beta1helper.DetermineMachineImageForName(cloudProfile, shootMachineImage.Name)
	if err != nil {
		return gardencorev1beta1.MachineImage{}, fmt.Errorf("failure while determining the default machine image in the CloudProfile: %s", err.Error())
	}
	if !machineImagesFound {
		return gardencorev1beta1.MachineImage{}, fmt.Errorf("failure while determining the default machine image in the CloudProfile: no machineImage with name '%s' (specified in shoot) could be found in the cloudProfile '%s'", shootMachineImage.Name, cloudProfile.Name)
	}

	return machineImageFromCloudProfile, nil
}

func shouldMachineImageBeUpdated(shoot *gardencorev1beta1.Shoot, machineImage *gardencorev1beta1.MachineImage, shootMachineImage *gardencorev1beta1.ShootMachineImage) (bool, *gardencorev1beta1.ShootMachineImage, error) {
	versionExistsInCloudProfile, _ := gardencorev1beta1helper.ShootMachineImageVersionExists(*machineImage, *shootMachineImage)
	if !versionExistsInCloudProfile || shoot.Spec.Maintenance.AutoUpdate.MachineImageVersion || ForceMachineImageUpdateRequired(shootMachineImage, *machineImage) {
		shootMachineImage, err := updateToLatestMachineImageVersion(*machineImage)
		if err != nil {
			return false, nil, fmt.Errorf("failure while updating machineImage to the latest version: %s", err.Error())
		}

		return true, shootMachineImage, nil
	}

	return false, nil, nil
}

// updateToLatestMachineImageVersion returns the latest machine image and requiring an image update
func updateToLatestMachineImageVersion(machineImage gardencorev1beta1.MachineImage) (*gardencorev1beta1.ShootMachineImage, error) {
	_, latestMachineImage, err := gardencorev1beta1helper.GetShootMachineImageFromLatestMachineImageVersion(machineImage)
	if err != nil {
		return nil, fmt.Errorf("failed to determine latest machine image in cloud profile: %s", err.Error())
	}
	return &latestMachineImage, nil
}

// ForceMachineImageUpdateRequired checks if the shoots current machine image has to be forcefully updated
func ForceMachineImageUpdateRequired(shootCurrentImage *gardencorev1beta1.ShootMachineImage, imageCloudProvider gardencorev1beta1.MachineImage) bool {
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

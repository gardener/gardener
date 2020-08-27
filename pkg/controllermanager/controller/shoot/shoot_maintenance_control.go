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
	"context"
	"fmt"
	"strconv"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/utils"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"github.com/Masterminds/semver"
	"github.com/sirupsen/logrus"
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
		nextMaintenance = time.Now().UTC().Add(duration)
	)

	logger.Logger.Infof("[SHOOT MAINTENANCE] %s - Scheduled maintenance in %s at %s", key, duration.Round(time.Minute), nextMaintenance.UTC().Round(time.Minute))
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
func NewDefaultMaintenanceControl(clientMap clientmap.ClientMap, k8sGardenCoreInformers gardencoreinformers.Interface, config config.ShootMaintenanceControllerConfiguration, recorder record.EventRecorder) MaintenanceControlInterface {
	return &defaultMaintenanceControl{clientMap, k8sGardenCoreInformers, config, recorder}
}

type defaultMaintenanceControl struct {
	clientMap              clientmap.ClientMap
	k8sGardenCoreInformers gardencoreinformers.Interface
	config                 config.ShootMaintenanceControllerConfiguration
	recorder               record.EventRecorder
}

func (c *defaultMaintenanceControl) Maintain(shootObj *gardencorev1beta1.Shoot, key string) error {
	var (
		ctx         = context.TODO()
		shoot       = shootObj.DeepCopy()
		shootLogger = logger.NewShootLogger(logger.Logger, shoot.Name, shoot.Namespace)
		handleError = func(msg string) {
			shootLogger.Error(msg)
		}
	)

	shootLogger.Infof("[SHOOT MAINTENANCE] %s", key)

	cloudProfile, err := c.k8sGardenCoreInformers.CloudProfiles().Lister().Get(shoot.Spec.CloudProfileName)
	if err != nil {
		return err
	}

	updatedMachineImages, reasonForImageUpdatePerPool, err := MaintainMachineImages(shootLogger, shootObj, cloudProfile)
	if err != nil {
		// continue execution to allow the kubernetes version update
		handleError(fmt.Sprintf("Could not maintain machine image version: %s", err.Error()))
	}

	updatedKubernetesVersion, reasonForKubernetesUpdate, err := MaintainKubernetesVersion(shootObj, cloudProfile, shootLogger)
	if err != nil {
		// continue execution to allow the machine image version update
		handleError(fmt.Sprintf("Could not maintain kubernetes version: %s", err.Error()))
	}

	gardenClient, err := c.clientMap.GetClient(ctx, keys.ForGarden())
	if err != nil {
		return fmt.Errorf("failed to get garden client: %w", err)
	}

	// Update the Shoot resource object.
	_, err = kutil.TryUpdateShoot(ctx, gardenClient.GardenCore(), retry.DefaultBackoff, shoot.ObjectMeta, func(s *gardencorev1beta1.Shoot) (*gardencorev1beta1.Shoot, error) {
		if !apiequality.Semantic.DeepEqual(shootObj.Spec.Maintenance.AutoUpdate, s.Spec.Maintenance.AutoUpdate) {
			return nil, fmt.Errorf("auto update section of Shoot %s/%s changed mid-air", s.Namespace, s.Name)
		}

		// do not add reconcile annotation if shoot was once set to failed or if shoot is already in an ongoing reconciliation
		if s.Status.LastOperation != nil && s.Status.LastOperation.State == gardencorev1beta1.LastOperationStateSucceeded {
			metav1.SetMetaDataAnnotation(&s.ObjectMeta, v1beta1constants.GardenerOperation, common.ShootOperationReconcile)
		}

		var needsRetry bool
		if val, ok := s.Annotations[common.FailedShootNeedsRetryOperation]; ok {
			needsRetry, _ = strconv.ParseBool(val)
		}
		delete(s.Annotations, common.FailedShootNeedsRetryOperation)

		if needsRetry {
			metav1.SetMetaDataAnnotation(&s.ObjectMeta, v1beta1constants.GardenerOperation, common.ShootOperationRetry)
		}

		if !gardencorev1beta1helper.HibernationIsEnabled(s) {
			controllerutils.AddTasks(s.Annotations, common.ShootTaskDeployInfrastructure)
		}
		if utils.IsTrue(c.config.EnableShootControlPlaneRestarter) {
			controllerutils.AddTasks(s.Annotations, common.ShootTaskRestartControlPlanePods)
		}

		if updatedMachineImages != nil {
			gardencorev1beta1helper.UpdateMachineImages(s.Spec.Provider.Workers, updatedMachineImages)
		}
		if updatedKubernetesVersion != nil {
			s.Spec.Kubernetes.Version = *updatedKubernetesVersion
		}

		if hasMaintainNowAnnotation(s) {
			delete(s.Annotations, v1beta1constants.GardenerOperation)
		}

		return s, nil
	})
	if err != nil {
		handleError(fmt.Sprintf("Could not update the Shoot specification: %s", err.Error()))
		return err
	}

	if updatedKubernetesVersion != nil {
		c.recorder.Eventf(shoot, corev1.EventTypeNormal, gardencorev1beta1.ShootEventK8sVersionMaintenance, "%s",
			fmt.Sprintf("Updated %s.", *reasonForKubernetesUpdate))
	}

	if updatedMachineImages != nil {
		for _, reason := range reasonForImageUpdatePerPool {
			c.recorder.Eventf(shoot, corev1.EventTypeNormal, gardencorev1beta1.ShootEventImageVersionMaintenance, "%s",
				fmt.Sprintf("Updated %s.", reason))
		}
	}

	shootLogger.Infof("[SHOOT MAINTENANCE] completed for %s/%s", shoot.Namespace, shoot.Name)
	return nil
}

// MaintainKubernetesVersion determines if a shoots kubernetes version has to be maintained and in case returns the target version
func MaintainKubernetesVersion(shoot *gardencorev1beta1.Shoot, profile *gardencorev1beta1.CloudProfile, logger *logrus.Entry) (updatedKubernetesVersion *string, messageKubernetesUpdate *string, error error) {
	shouldBeUpdated, reason, isExpired, err := shouldKubernetesVersionBeUpdated(shoot, profile)
	if err != nil {
		return nil, nil, err
	}
	if shouldBeUpdated {
		// get latest version that qualifies for a patch update
		newerPatchVersionFound, latestPatchVersion, err := gardencorev1beta1helper.GetKubernetesVersionForPatchUpdate(profile, shoot.Spec.Kubernetes.Version)
		if err != nil {
			return nil, nil, fmt.Errorf("failure while determining the latest Kubernetes patch version in the CloudProfile: %s", err.Error())
		}
		if newerPatchVersionFound {
			msg := fmt.Sprintf("Kubernetes version '%s' to version '%s'. This is an increase in the patch level. Reason: %s", shoot.Spec.Kubernetes.Version, latestPatchVersion, *reason)
			logger.Debugf("[SHOOT MAINTENANCE] Updating %s", msg)
			return &latestPatchVersion, &msg, nil
		}
		// no newer patch version found & is expired -> forcefully update to latest patch of next minor version
		if isExpired {
			// get latest version that qualifies for a minor update
			newMinorAvailable, latestPatchVersionNewMinor, err := gardencorev1beta1helper.GetKubernetesVersionForMinorUpdate(profile, shoot.Spec.Kubernetes.Version)
			if err != nil {
				return nil, nil, fmt.Errorf("failure while determining newer Kubernetes minor version in the CloudProfile: %s", err.Error())
			}
			// cannot update as there is no consecutive minor version available (e.g shoot is on 1.13.X, but there is only 1.15.X, available and not 1.14.X)
			if !newMinorAvailable {
				return nil, nil, fmt.Errorf("cannot perform minor Kubernetes version update for expired Kubernetes version '%s'. No suitable version found in CloudProfile - this is most likely a misconfiguration of the CloudProfile", shoot.Spec.Kubernetes.Version)
			}

			msg := fmt.Sprintf("Kubernetes version '%s' to version '%s'. This is an increase in the minor level. Reason: %s", shoot.Spec.Kubernetes.Version, latestPatchVersionNewMinor, *reason)
			logger.Debugf("[SHOOT MAINTENANCE] Updating %s", msg)
			return &latestPatchVersionNewMinor, &msg, nil
		}
	}
	return nil, nil, nil
}

func shouldKubernetesVersionBeUpdated(shoot *gardencorev1beta1.Shoot, profile *gardencorev1beta1.CloudProfile) (shouldBeUpdated bool, reason *string, isExpired bool, error error) {
	versionExistsInCloudProfile, version, err := gardencorev1beta1helper.KubernetesVersionExistsInCloudProfile(profile, shoot.Spec.Kubernetes.Version)
	if err != nil {
		return false, nil, false, err
	}

	var updateReason string
	if !versionExistsInCloudProfile {
		updateReason = "Version does not exist in CloudProfile"
		return true, &updateReason, true, nil
	}

	if ExpirationDateExpired(version.ExpirationDate) {
		updateReason = "Kubernetes version expired - force update required"
		return true, &updateReason, true, nil
	}

	if shoot.Spec.Maintenance.AutoUpdate.KubernetesVersion {
		updateReason = "AutoUpdate of Kubernetes version configured"
		return true, &updateReason, false, nil
	}

	return false, nil, false, nil
}

func mustMaintainNow(shoot *gardencorev1beta1.Shoot) bool {
	return hasMaintainNowAnnotation(shoot) || common.IsNowInEffectiveShootMaintenanceTimeWindow(shoot)
}

func hasMaintainNowAnnotation(shoot *gardencorev1beta1.Shoot) bool {
	operation, ok := common.GetShootOperationAnnotation(shoot.Annotations)
	return ok && operation == common.ShootOperationMaintain
}

// MaintainMachineImages determines if a shoots machine images have to be maintained and in case returns the target images
func MaintainMachineImages(logger *logrus.Entry, shoot *gardencorev1beta1.Shoot, cloudProfile *gardencorev1beta1.CloudProfile) (updatedMachineImages []*gardencorev1beta1.ShootMachineImage, reasons []string, error error) {
	var (
		shootMachineImagesForUpdate []*gardencorev1beta1.ShootMachineImage
		reasonsForUpdate            []string
	)
	for _, worker := range shoot.Spec.Provider.Workers {
		workerImage := worker.Machine.Image
		machineImageFromCloudProfile, err := determineMachineImage(cloudProfile, workerImage)
		if err != nil {
			return nil, nil, err
		}

		shouldBeUpdated, reason, updatedMachineImage, err := shouldMachineImageBeUpdated(logger, shoot.Spec.Maintenance.AutoUpdate.MachineImageVersion, &machineImageFromCloudProfile, workerImage)
		if err != nil {
			return nil, nil, err
		}

		if !shouldBeUpdated {
			continue
		}

		message := fmt.Sprintf("image of worker-pool '%s' from '%s' version '%s' to version '%s'. Reason: %s", worker.Name, workerImage.Name, *workerImage.Version, *updatedMachineImage.Version, *reason)
		reasonsForUpdate = append(reasonsForUpdate, message)
		logger.Debugf("[SHOOT MAINTENANCE] Updating %s", message)
		shootMachineImagesForUpdate = append(shootMachineImagesForUpdate, updatedMachineImage)
	}
	if len(shootMachineImagesForUpdate) == 0 {
		return nil, nil, nil
	}
	return shootMachineImagesForUpdate, reasonsForUpdate, nil
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

// shouldMachineImageBeUpdated determines if a machine image should be updated based on whether it exists in the CloudProfile, auto update applies or a force update is required.
func shouldMachineImageBeUpdated(logger *logrus.Entry, autoUpdateMachineImageVersion bool, machineImage *gardencorev1beta1.MachineImage, shootMachineImage *gardencorev1beta1.ShootMachineImage) (shouldBeUpdated bool, reason *string, updatedMachineImage *gardencorev1beta1.ShootMachineImage, error error) {
	versionExistsInCloudProfile, versionIndex := gardencorev1beta1helper.ShootMachineImageVersionExists(*machineImage, *shootMachineImage)
	var reasonForUpdate string

	forceUpdateRequired := ForceMachineImageUpdateRequired(shootMachineImage, *machineImage)
	if !versionExistsInCloudProfile || autoUpdateMachineImageVersion || forceUpdateRequired {
		// safe operation, as Shoot machine image version can only be a valid semantic version
		shootSemanticVersion := *semver.MustParse(*shootMachineImage.Version)

		// get latest version qualifying for Shoot machine image update
		qualifyingVersionFound, latestShootMachineImage, err := gardencorev1beta1helper.GetLatestQualifyingShootMachineImage(*machineImage, gardencorev1beta1helper.FilterLowerVersion(shootSemanticVersion))
		if err != nil {
			return false, nil, nil, fmt.Errorf("an error occured while determining the latest Shoot Machine Image for machine image %q: %s", machineImage.Name, err.Error())
		}

		// this is a special case when a Shoot is using a preview version. Preview versions should not be updated-to and are therefore not part of the qualifying versions.
		// if no qualifying version can be found and the Shoot is already using a preview version, then do nothing.
		if !qualifyingVersionFound && versionExistsInCloudProfile && machineImage.Versions[versionIndex].Classification != nil && *machineImage.Versions[versionIndex].Classification == gardencorev1beta1.ClassificationPreview {
			logger.Debugf("MachineImage update not required. Already using preview version.")
			return false, nil, nil, nil
		}

		// otherwise, there should always be a qualifying version (at least the Shoot's machine image version itself).
		if !qualifyingVersionFound {
			return false, nil, nil, fmt.Errorf("no latest qualifying Shoot machine image could be determined for machine image %q. Either the machine image is reaching end of life and migration to another machine image is required or there is a misconfiguration in the CloudProfile. If it is the latter, make sure the machine image in the CloudProfile has at least one version that is not expired, not in preview and greater or equal to the current Shoot image version %q", machineImage.Name, *shootMachineImage.Version)
		}

		if *latestShootMachineImage.Version == *shootMachineImage.Version {
			logger.Debugf("MachineImage update not required. Already up to date.")
			return false, nil, nil, nil
		}

		if !versionExistsInCloudProfile {
			// deletion a machine image that is still in use by a Shoot is blocked in the apiserver. However it is still required,
			// because old installations might still have shoot's that have no corresponding version in the CloudProfile.
			reasonForUpdate = "Version does not exist in CloudProfile"
		} else if autoUpdateMachineImageVersion {
			reasonForUpdate = "AutoUpdate of MachineImage configured"
		} else if forceUpdateRequired {
			reasonForUpdate = "MachineImage expired - force update required"
		}

		return true, &reasonForUpdate, latestShootMachineImage, nil
	}

	return false, nil, nil, nil
}

// ForceMachineImageUpdateRequired checks if the shoots current machine image has to be forcefully updated
func ForceMachineImageUpdateRequired(shootCurrentImage *gardencorev1beta1.ShootMachineImage, imageCloudProvider gardencorev1beta1.MachineImage) bool {
	for _, image := range imageCloudProvider.Versions {
		if shootCurrentImage.Version != nil && *shootCurrentImage.Version != image.Version {
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

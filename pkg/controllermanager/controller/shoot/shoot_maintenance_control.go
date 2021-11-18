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
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	controllermanagerfeatures "github.com/gardener/gardener/pkg/controllermanager/features"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/utils"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"github.com/Masterminds/semver"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
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

// NewShootMaintenanceReconciler creates a new instance of a reconciler which maintains Shoots.
func NewShootMaintenanceReconciler(l logrus.FieldLogger, gardenClient client.Client, config config.ShootMaintenanceControllerConfiguration, recorder record.EventRecorder) reconcile.Reconciler {
	return &shootMaintenanceReconciler{
		logger:       l,
		gardenClient: gardenClient,
		config:       config,
		recorder:     recorder,
	}
}

type shootMaintenanceReconciler struct {
	logger       logrus.FieldLogger
	gardenClient client.Client
	config       config.ShootMaintenanceControllerConfiguration
	recorder     record.EventRecorder
}

func (r *shootMaintenanceReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	shoot := &gardencorev1beta1.Shoot{}
	if err := r.gardenClient.Get(ctx, request.NamespacedName, shoot); err != nil {
		if apierrors.IsNotFound(err) {
			r.logger.Infof("Object %q is gone, stop reconciling: %v", request.Name, err)
			return reconcile.Result{}, nil
		}
		r.logger.Infof("Unable to retrieve object %q from store: %v", request.Name, err)
		return reconcile.Result{}, err
	}

	if shoot.DeletionTimestamp != nil {
		r.logger.Debug("[SHOOT MAINTENANCE] - skipping because Shoot is marked as to be deleted")
		return reconcile.Result{}, nil
	}

	requeueAfter := requeueAfterDuration(shoot)

	if !mustMaintainNow(shoot) {
		logger.Logger.Infof("[SHOOT MAINTENANCE] %s/%s - skipping because Shoot must not be maintained now.", shoot.Namespace, shoot.Name)
		return reconcile.Result{RequeueAfter: requeueAfter}, nil
	}

	return reconcile.Result{RequeueAfter: requeueAfter}, r.reconcile(ctx, shoot, r.gardenClient)
}

func requeueAfterDuration(shoot *gardencorev1beta1.Shoot) time.Duration {
	var (
		now             = time.Now()
		window          = gutil.EffectiveShootMaintenanceTimeWindow(shoot)
		duration        = window.RandomDurationUntilNext(now, false)
		nextMaintenance = time.Now().UTC().Add(duration)
	)

	logger.Logger.Infof("[SHOOT MAINTENANCE] %s/%s - Scheduled maintenance in %s at %s", shoot.Namespace, shoot.Name, duration.Round(time.Minute), nextMaintenance.UTC().Round(time.Minute))
	return duration
}

func (r *shootMaintenanceReconciler) reconcile(ctx context.Context, shoot *gardencorev1beta1.Shoot, gardenClient client.Client) error {
	key := fmt.Sprintf("%s/%s", shoot.Namespace, shoot.Name)

	shootLogger := r.logger.WithField("shoot", key)
	shootLogger.Infof("[SHOOT MAINTENANCE] %s", key)

	cloudProfile := &gardencorev1beta1.CloudProfile{}
	if err := r.gardenClient.Get(ctx, kutil.Key(shoot.Spec.CloudProfileName), cloudProfile); err != nil {
		return err
	}

	reasonForImageUpdatePerPool, err := MaintainMachineImages(shootLogger, shoot, cloudProfile)
	if err != nil {
		// continue execution to allow the kubernetes version update
		shootLogger.Error(fmt.Sprintf("Could not maintain machine image version: %s", err.Error()))
	}

	reasonForKubernetesUpdate, err := maintainKubernetesVersion(shoot.Spec.Kubernetes.Version, shoot.Spec.Maintenance.AutoUpdate.KubernetesVersion, cloudProfile, func(v string) error {
		shoot.Spec.Kubernetes.Version = v
		return nil
	})
	if err != nil {
		// continue execution to allow the machine image version update
		shootLogger.Error(fmt.Sprintf("Could not maintain kubernetes version: %s", err.Error()))
	}

	shootSemver, err := semver.NewVersion(shoot.Spec.Kubernetes.Version)
	if err != nil {
		return err
	}

	// Now its time to update worker pool kubernetes version if specified
	var reasonsForWorkerPoolKubernetesUpdate = make(map[string]string)
	for i, w := range shoot.Spec.Provider.Workers {
		if w.Kubernetes == nil || w.Kubernetes.Version == nil {
			continue
		}

		reasonForWorkerPoolKubernetesUpdate, err := maintainKubernetesVersion(*w.Kubernetes.Version, shoot.Spec.Maintenance.AutoUpdate.KubernetesVersion, cloudProfile, func(v string) error {
			workerPoolSemver, err := semver.NewVersion(v)
			if err != nil {
				return err
			}
			// If during autoupdate a worker pool kubernetes gets forcefully updated to the next minor which might be higher than the same minor of the shoot, take this
			if workerPoolSemver.GreaterThan(shootSemver) {
				workerPoolSemver = shootSemver
			}
			v = workerPoolSemver.String()
			shoot.Spec.Provider.Workers[i].Kubernetes.Version = &v
			return nil
		})
		if err != nil {
			// continue execution to allow the machine image version update
			shootLogger.Error(fmt.Sprintf("Could not maintain kubernetes version for worker pool:%s: %s", w.Name, err.Error()))
		}
		reasonsForWorkerPoolKubernetesUpdate[w.Name] = reasonForWorkerPoolKubernetesUpdate
	}

	// do not add reconcile annotation if shoot was once set to failed or if shoot is already in an ongoing reconciliation
	if shoot.Status.LastOperation != nil && shoot.Status.LastOperation.State == gardencorev1beta1.LastOperationStateSucceeded {
		metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationReconcile)
	}

	var needsRetry bool
	if val, ok := shoot.Annotations[v1beta1constants.FailedShootNeedsRetryOperation]; ok {
		needsRetry, _ = strconv.ParseBool(val)
	}
	delete(shoot.Annotations, v1beta1constants.FailedShootNeedsRetryOperation)

	// Failed shoots need to be retried first; healthy shoots instead
	// default to rotating their SSH keypair on each maintenance interval if the RotateSSHKeypairOnMaintenance is enabled.
	if needsRetry {
		metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationRetry)
	} else if controllermanagerfeatures.FeatureGate.Enabled(features.RotateSSHKeypairOnMaintenance) {
		metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationRotateSSHKeypair)
	}

	controllerutils.AddTasks(shoot.Annotations, v1beta1constants.ShootTaskDeployInfrastructure)
	if utils.IsTrue(r.config.EnableShootControlPlaneRestarter) {
		controllerutils.AddTasks(shoot.Annotations, v1beta1constants.ShootTaskRestartControlPlanePods)
	}

	if utils.IsTrue(r.config.EnableShootCoreAddonRestarter) {
		controllerutils.AddTasks(shoot.Annotations, v1beta1constants.ShootTaskRestartCoreAddons)
	}

	if hasMaintainNowAnnotation(shoot) {
		delete(shoot.Annotations, v1beta1constants.GardenerOperation)
	}

	// try to maintain shoot, but don't retry on conflict, because a conflict means that we potentially operated on stale
	// data (e.g. when calculating the updated k8s version), so rather return error and backoff
	if err := gardenClient.Update(ctx, shoot); err != nil {
		shootLogger.Errorf("Failed to update Shoot spec: %+v", err)
		return err
	}

	for _, reason := range reasonForImageUpdatePerPool {
		r.recorder.Eventf(shoot, corev1.EventTypeNormal, gardencorev1beta1.ShootEventImageVersionMaintenance, "%s",
			fmt.Sprintf("Updated %s.", reason))
		shootLogger.Debugf("[SHOOT MAINTENANCE] Updating %s", reason)
	}

	if reasonForKubernetesUpdate != "" {
		r.recorder.Eventf(shoot, corev1.EventTypeNormal, gardencorev1beta1.ShootEventK8sVersionMaintenance, "%s",
			fmt.Sprintf("Updated %s.", reasonForKubernetesUpdate))
		shootLogger.Debugf("[SHOOT MAINTENANCE] Updating %s", reasonForKubernetesUpdate)
	}

	for name, reason := range reasonsForWorkerPoolKubernetesUpdate {
		r.recorder.Eventf(shoot, corev1.EventTypeNormal, gardencorev1beta1.ShootEventK8sVersionMaintenance, "%s",
			fmt.Sprintf("Updated worker pool %q %s.", name, reason))
		shootLogger.Debugf("[SHOOT MAINTENANCE] Updating worker pool %q %s", name, reason)
	}

	shootLogger.Infof("[SHOOT MAINTENANCE] completed")
	return nil
}

// MaintainMachineImages updates the machine images of a Shoot's worker pools if necessary
func MaintainMachineImages(shootLogger *logrus.Entry, shoot *gardencorev1beta1.Shoot, cloudProfile *gardencorev1beta1.CloudProfile) ([]string, error) {
	var reasonsForUpdate []string

	for i, worker := range shoot.Spec.Provider.Workers {
		workerImage := worker.Machine.Image
		machineImageFromCloudProfile, err := determineMachineImage(cloudProfile, workerImage)
		if err != nil {
			return nil, err
		}

		filteredMachineImageVersionsFromCloudProfile := filterForCRI(&machineImageFromCloudProfile, worker.CRI)
		shouldBeUpdated, reason, updatedMachineImage, err := shouldMachineImageBeUpdated(shootLogger, shoot.Spec.Maintenance.AutoUpdate.MachineImageVersion, filteredMachineImageVersionsFromCloudProfile, workerImage)
		if err != nil {
			return nil, err
		}

		if !shouldBeUpdated {
			continue
		}

		shoot.Spec.Provider.Workers[i].Machine.Image = updatedMachineImage

		message := fmt.Sprintf("image of worker-pool %q from %q version %q to version %q. Reason: %s", worker.Name, workerImage.Name, *workerImage.Version, *updatedMachineImage.Version, reason)
		reasonsForUpdate = append(reasonsForUpdate, message)
	}

	return reasonsForUpdate, nil
}

// maintainKubernetesVersion updates a Shoot's Kubernetes version if necessary and returns the reason why an update was done
func maintainKubernetesVersion(kubernetesVersion string, autoUpdate bool, profile *gardencorev1beta1.CloudProfile, updateFunc func(string) error) (string, error) {
	shouldBeUpdated, reason, isExpired, err := shouldKubernetesVersionBeUpdated(kubernetesVersion, autoUpdate, profile)
	if err != nil {
		return "", err
	}
	if !shouldBeUpdated {
		return "", nil
	}

	updatedKubernetesVersion, err := determineKubernetesVersion(kubernetesVersion, profile, isExpired)
	if err != nil {
		return "", err
	}
	if updatedKubernetesVersion == "" {
		return "", nil
	}
	reasonForKubernetesUpdate := fmt.Sprintf("Kubernetes version %q to version %q. Reason: %s", kubernetesVersion, updatedKubernetesVersion, reason)
	err = updateFunc(updatedKubernetesVersion)
	if err != nil {
		return "", err
	}
	return reasonForKubernetesUpdate, err
}

func determineKubernetesVersion(kubernetesVersion string, profile *gardencorev1beta1.CloudProfile, isExpired bool) (string, error) {
	// get latest version that qualifies for a patch update
	newerPatchVersionFound, latestPatchVersion, err := gardencorev1beta1helper.GetKubernetesVersionForPatchUpdate(profile, kubernetesVersion)
	if err != nil {
		return "", fmt.Errorf("failure while determining the latest Kubernetes patch version in the CloudProfile: %s", err.Error())
	}
	if newerPatchVersionFound {
		return latestPatchVersion, nil
	}
	// no newer patch version found & is expired -> forcefully update to latest patch of next minor version
	if isExpired {
		// get latest version that qualifies for a minor update
		newMinorAvailable, latestPatchVersionNewMinor, err := gardencorev1beta1helper.GetKubernetesVersionForMinorUpdate(profile, kubernetesVersion)
		if err != nil {
			return "", fmt.Errorf("failure while determining newer Kubernetes minor version in the CloudProfile: %s", err.Error())
		}
		// cannot update as there is no consecutive minor version available (e.g shoot is on 1.16.X, but there is only 1.18.X, available and not 1.17.X)
		if !newMinorAvailable {
			return "", fmt.Errorf("cannot perform minor Kubernetes version update for expired Kubernetes version %q. No suitable version found in CloudProfile - this is most likely a misconfiguration of the CloudProfile", kubernetesVersion)
		}

		return latestPatchVersionNewMinor, nil
	}
	return "", nil
}

func shouldKubernetesVersionBeUpdated(kubernetesVersion string, autoUpdate bool, profile *gardencorev1beta1.CloudProfile) (shouldBeUpdated bool, reason string, isExpired bool, error error) {
	versionExistsInCloudProfile, version, err := gardencorev1beta1helper.KubernetesVersionExistsInCloudProfile(profile, kubernetesVersion)
	if err != nil {
		return false, "", false, err
	}

	var updateReason string
	if !versionExistsInCloudProfile {
		updateReason = "Version does not exist in CloudProfile"
		return true, updateReason, true, nil
	}

	if ExpirationDateExpired(version.ExpirationDate) {
		updateReason = "Kubernetes version expired - force update required"
		return true, updateReason, true, nil
	}

	if autoUpdate {
		updateReason = "AutoUpdate of Kubernetes version configured"
		return true, updateReason, false, nil
	}

	return false, "", false, nil
}

func mustMaintainNow(shoot *gardencorev1beta1.Shoot) bool {
	return hasMaintainNowAnnotation(shoot) || gutil.IsNowInEffectiveShootMaintenanceTimeWindow(shoot)
}

func hasMaintainNowAnnotation(shoot *gardencorev1beta1.Shoot) bool {
	operation, ok := shoot.Annotations[v1beta1constants.GardenerOperation]
	return ok && operation == v1beta1constants.ShootOperationMaintain
}

func filterForCRI(machineImageFromCloudProfile *gardencorev1beta1.MachineImage, workerCRI *gardencorev1beta1.CRI) *gardencorev1beta1.MachineImage {
	if workerCRI == nil {
		return filterForCRI(machineImageFromCloudProfile, &gardencorev1beta1.CRI{Name: gardencorev1beta1.CRINameDocker})
	}

	filteredMachineImages := gardencorev1beta1.MachineImage{Name: machineImageFromCloudProfile.Name,
		Versions: []gardencorev1beta1.MachineImageVersion{}}

	for _, cloudProfileVersion := range machineImageFromCloudProfile.Versions {
		criFromCloudProfileVersion, found := findCRIByName(workerCRI.Name, cloudProfileVersion.CRI)
		if !found {
			continue
		}

		if !areAllWorkerCRsPartOfCloudProfileVersion(workerCRI.ContainerRuntimes, criFromCloudProfileVersion.ContainerRuntimes) {
			continue
		}

		filteredMachineImages.Versions = append(filteredMachineImages.Versions, cloudProfileVersion)
	}

	return &filteredMachineImages
}

func findCRIByName(wanted gardencorev1beta1.CRIName, cris []gardencorev1beta1.CRI) (gardencorev1beta1.CRI, bool) {
	for _, cri := range cris {
		if cri.Name == wanted {
			return cri, true
		}
	}
	return gardencorev1beta1.CRI{}, false
}

func areAllWorkerCRsPartOfCloudProfileVersion(workerCRs []gardencorev1beta1.ContainerRuntime, crsFromCloudProfileVersion []gardencorev1beta1.ContainerRuntime) bool {
	if workerCRs == nil {
		return true
	}
	for _, workerCr := range workerCRs {
		if !isWorkerCRPartOfCloudProfileVersionCRs(workerCr, crsFromCloudProfileVersion) {
			return false
		}
	}
	return true
}

func isWorkerCRPartOfCloudProfileVersionCRs(wanted gardencorev1beta1.ContainerRuntime, cloudProfileVersionCRs []gardencorev1beta1.ContainerRuntime) bool {
	for _, cr := range cloudProfileVersionCRs {
		if wanted.Type == cr.Type {
			return true
		}
	}
	return false
}

func determineMachineImage(cloudProfile *gardencorev1beta1.CloudProfile, shootMachineImage *gardencorev1beta1.ShootMachineImage) (gardencorev1beta1.MachineImage, error) {
	machineImagesFound, machineImageFromCloudProfile, err := gardencorev1beta1helper.DetermineMachineImageForName(cloudProfile, shootMachineImage.Name)
	if err != nil {
		return gardencorev1beta1.MachineImage{}, fmt.Errorf("failure while determining the default machine image in the CloudProfile: %s", err.Error())
	}
	if !machineImagesFound {
		return gardencorev1beta1.MachineImage{}, fmt.Errorf("failure while determining the default machine image in the CloudProfile: no machineImage with name %q (specified in shoot) could be found in the cloudProfile %q", shootMachineImage.Name, cloudProfile.Name)
	}

	return machineImageFromCloudProfile, nil
}

// shouldMachineImageBeUpdated determines if a machine image should be updated based on whether it exists in the CloudProfile, auto update applies or a force update is required.
func shouldMachineImageBeUpdated(logger *logrus.Entry, autoUpdateMachineImageVersion bool, machineImage *gardencorev1beta1.MachineImage, shootMachineImage *gardencorev1beta1.ShootMachineImage) (shouldBeUpdated bool, reason string, updatedMachineImage *gardencorev1beta1.ShootMachineImage, error error) {
	versionExistsInCloudProfile, versionIndex := gardencorev1beta1helper.ShootMachineImageVersionExists(*machineImage, *shootMachineImage)
	var reasonForUpdate string

	forceUpdateRequired := ForceMachineImageUpdateRequired(shootMachineImage, *machineImage)
	if !versionExistsInCloudProfile || autoUpdateMachineImageVersion || forceUpdateRequired {
		// safe operation, as Shoot machine image version can only be a valid semantic version
		shootSemanticVersion := *semver.MustParse(*shootMachineImage.Version)

		// get latest version qualifying for Shoot machine image update
		qualifyingVersionFound, latestShootMachineImage, err := gardencorev1beta1helper.GetLatestQualifyingShootMachineImage(*machineImage, gardencorev1beta1helper.FilterLowerVersion(shootSemanticVersion))
		if err != nil {
			return false, "", nil, fmt.Errorf("an error occured while determining the latest Shoot Machine Image for machine image %q: %s", machineImage.Name, err.Error())
		}

		// this is a special case when a Shoot is using a preview version. Preview versions should not be updated-to and are therefore not part of the qualifying versions.
		// if no qualifying version can be found and the Shoot is already using a preview version, then do nothing.
		if !qualifyingVersionFound && versionExistsInCloudProfile && machineImage.Versions[versionIndex].Classification != nil && *machineImage.Versions[versionIndex].Classification == gardencorev1beta1.ClassificationPreview {
			logger.Debugf("MachineImage update not required. Already using preview version.")
			return false, "", nil, nil
		}

		// otherwise, there should always be a qualifying version (at least the Shoot's machine image version itself).
		if !qualifyingVersionFound {
			return false, "", nil, fmt.Errorf("no latest qualifying Shoot machine image could be determined for machine image %q. Either the machine image is reaching end of life and migration to another machine image is required or there is a misconfiguration in the CloudProfile. If it is the latter, make sure the machine image in the CloudProfile has at least one version that is not expired, not in preview and greater or equal to the current Shoot image version %q", machineImage.Name, *shootMachineImage.Version)
		}

		if *latestShootMachineImage.Version == *shootMachineImage.Version {
			logger.Debugf("MachineImage update not required. Already up to date.")
			return false, "", nil, nil
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

		return true, reasonForUpdate, latestShootMachineImage, nil
	}

	return false, "", nil, nil
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

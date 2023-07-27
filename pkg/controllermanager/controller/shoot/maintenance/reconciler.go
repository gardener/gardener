// Copyright 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package maintenance

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Masterminds/semver"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/clock"
	"k8s.io/utils/pointer"
	"k8s.io/utils/strings/slices"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	"github.com/gardener/gardener/pkg/controllerutils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	versionutils "github.com/gardener/gardener/pkg/utils/version"
)

// Reconciler reconciles Shoots and maintains them by updating versions or triggering operations.
type Reconciler struct {
	Client   client.Client
	Config   config.ShootMaintenanceControllerConfiguration
	Clock    clock.Clock
	Recorder record.EventRecorder
}

// Reconcile reconciles Shoots and maintains them by updating versions or triggering operations.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	ctx, cancel := controllerutils.GetMainReconciliationContext(ctx, controllerutils.DefaultReconciliationTimeout)
	defer cancel()

	shoot := &gardencorev1beta1.Shoot{}
	if err := r.Client.Get(ctx, request.NamespacedName, shoot); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	if shoot.DeletionTimestamp != nil {
		log.V(1).Info("Skipping Shoot because it is marked for deletion")
		return reconcile.Result{}, nil
	}

	requeueAfter, nextMaintenance := requeueAfterDuration(shoot)

	if !mustMaintainNow(shoot, r.Clock) {
		log.V(1).Info("Skipping Shoot because it doesn't need to be maintained now")
		log.V(1).Info("Scheduled next maintenance for Shoot", "duration", requeueAfter.Round(time.Minute), "nextMaintenance", nextMaintenance.Round(time.Minute))
		return reconcile.Result{RequeueAfter: requeueAfter}, nil
	}

	if err := r.reconcile(ctx, log, shoot); err != nil {
		return reconcile.Result{}, err
	}

	log.V(1).Info("Scheduled next maintenance for Shoot", "duration", requeueAfter.Round(time.Minute), "nextMaintenance", nextMaintenance.Round(time.Minute))
	return reconcile.Result{RequeueAfter: requeueAfter}, nil
}

func requeueAfterDuration(shoot *gardencorev1beta1.Shoot) (time.Duration, time.Time) {
	var (
		now             = time.Now()
		window          = gardenerutils.EffectiveShootMaintenanceTimeWindow(shoot)
		duration        = window.RandomDurationUntilNext(now, false)
		nextMaintenance = time.Now().UTC().Add(duration)
	)

	return duration, nextMaintenance
}

func (r *Reconciler) reconcile(ctx context.Context, log logr.Logger, shoot *gardencorev1beta1.Shoot) error {
	log.Info("Maintaining Shoot")

	var (
		maintainedShoot             = shoot.DeepCopy()
		operations                  []string
		reasonForImageUpdatePerPool []string
		err                         error
	)

	cloudProfile := &gardencorev1beta1.CloudProfile{}
	if err = r.Client.Get(ctx, kubernetesutils.Key(shoot.Spec.CloudProfileName), cloudProfile); err != nil {
		return err
	}

	if !v1beta1helper.IsWorkerless(shoot) {
		reasonForImageUpdatePerPool, err = maintainMachineImages(log, maintainedShoot, cloudProfile)
		if err != nil {
			// continue execution to allow the kubernetes version update
			log.Error(err, "Failed to maintain Shoot machine images")
		}
		operations = append(operations, reasonForImageUpdatePerPool...)
	}

	reasonForKubernetesUpdate, err := maintainKubernetesVersion(log, maintainedShoot.Spec.Kubernetes.Version, maintainedShoot.Spec.Maintenance.AutoUpdate.KubernetesVersion, cloudProfile, func(v string) error {
		maintainedShoot.Spec.Kubernetes.Version = v
		return nil
	}, "Control Plane")
	if err != nil {
		// continue execution to allow the machine image version update
		log.Error(err, "Failed to maintain Shoot kubernetes version")
	}

	oldShootKubernetesVersion, err := semver.NewVersion(shoot.Spec.Kubernetes.Version)
	if err != nil {
		return err
	}

	shootKubernetesVersion, err := semver.NewVersion(maintainedShoot.Spec.Kubernetes.Version)
	if err != nil {
		return err
	}

	// Force containerd when shoot cluster is updated to k8s version >= 1.23
	if versionutils.ConstraintK8sLess123.Check(oldShootKubernetesVersion) && versionutils.ConstraintK8sGreaterEqual123.Check(shootKubernetesVersion) {
		reasonsForContainerdUpdate, err := updateToContainerd(log, maintainedShoot, fmt.Sprintf("Updating Kubernetes to %q", shootKubernetesVersion.String()))
		if err != nil {
			// continue execution to allow other updates
			log.Error(err, "Failed updating worker pools' CRI to containerd")
		}
		operations = append(operations, reasonsForContainerdUpdate...)
	}

	// Reset the `EnableStaticTokenKubeconfig` value to false, when shoot cluster is updated to  k8s version >= 1.27.
	if versionutils.ConstraintK8sLess127.Check(oldShootKubernetesVersion) && pointer.BoolDeref(maintainedShoot.Spec.Kubernetes.EnableStaticTokenKubeconfig, false) && versionutils.ConstraintK8sGreaterEqual127.Check(shootKubernetesVersion) {
		maintainedShoot.Spec.Kubernetes.EnableStaticTokenKubeconfig = pointer.Bool(false)

		reason := "EnableStaticTokenKubeconfig is set to false. Reason: The static token kubeconfig can no longer be enabled for Shoot clusters using Kubernetes version 1.27 and higher"
		operations = append(operations, reason)
	}

	// Now it's time to update worker pool kubernetes version if specified
	for i, pool := range maintainedShoot.Spec.Provider.Workers {
		if pool.Kubernetes == nil || pool.Kubernetes.Version == nil {
			continue
		}

		workerLog := log.WithValues("worker", pool.Name)
		name := "Worker Pool " + pool.Name
		reasonForWorkerPoolKubernetesUpdate, err := maintainKubernetesVersion(workerLog, *pool.Kubernetes.Version, maintainedShoot.Spec.Maintenance.AutoUpdate.KubernetesVersion, cloudProfile, func(v string) error {
			workerPoolSemver, err := semver.NewVersion(v)
			if err != nil {
				return err
			}
			// If during autoupdate a worker pool kubernetes gets forcefully updated to the next minor which might be higher than the same minor of the shoot, take this
			if workerPoolSemver.GreaterThan(shootKubernetesVersion) {
				workerPoolSemver = shootKubernetesVersion
			}
			v = workerPoolSemver.String()
			maintainedShoot.Spec.Provider.Workers[i].Kubernetes.Version = &v
			return nil
		}, name)
		if err != nil {
			// continue execution to allow the machine image version update
			workerLog.Error(err, "Could not maintain kubernetes version for worker pool")
		}

		reasonForKubernetesUpdate = append(reasonForKubernetesUpdate, reasonForWorkerPoolKubernetesUpdate...)
	}
	operations = append(operations, reasonForKubernetesUpdate...)

	operation := maintainOperation(maintainedShoot)
	if operation != "" {
		operations = append(operations, fmt.Sprintf("Added %q operation annotation", operation))
	}

	if len(operations) > 0 {
		patch := client.MergeFrom(shoot.DeepCopy())
		shoot.Status.LastMaintenance = &gardencorev1beta1.LastMaintenance{
			Description:   strings.Join(operations, ", "),
			TriggeredTime: metav1.Time{Time: r.Clock.Now()},
			State:         gardencorev1beta1.LastOperationStateProcessing,
		}

		// First dry run the update call to check if it can be executed successfully.
		// If not shoot maintenance is marked as failed and is retried only in
		// next maintenance window.
		if err := r.Client.Update(ctx, maintainedShoot, &client.UpdateOptions{
			DryRun: []string{metav1.DryRunAll},
		}); err != nil {
			// If shoot maintenance is triggered by `gardener.cloud/operation=maintain` annotation and if it fails in dry run,
			// `maintain` operation annotation needs to be removed so that if reason for failure is fixed and maintenance is triggered
			// again via `maintain` operation annotation then it should not fail with the reason that annotation is already present.
			// Removal of annotation during shoot status patch is possible cause only spec is kept in original form during status update
			// https://github.com/gardener/gardener/blob/a2f7de0badaae6170d7b9b84c163b8cab43a84d2/pkg/registry/core/shoot/strategy.go#L258-L267
			if hasMaintainNowAnnotation(shoot) {
				delete(shoot.Annotations, v1beta1constants.GardenerOperation)
			}
			shoot.Status.LastMaintenance.State = gardencorev1beta1.LastOperationStateFailed
			shoot.Status.LastMaintenance.FailureReason = pointer.String(fmt.Sprintf("Maintenance failed: %s", err.Error()))
			if err := r.Client.Status().Patch(ctx, shoot, patch); err != nil {
				return err
			}

			log.Info("Shoot maintenance failed", "reason", err)
			return nil
		}

		if err := r.Client.Status().Patch(ctx, shoot, patch); err != nil {
			return err
		}
	}

	// update shoot spec changes in maintenance call
	shoot.Spec = *maintainedShoot.Spec.DeepCopy()
	_ = maintainOperation(shoot)
	maintainTasks(shoot, r.Config)

	// try to maintain shoot, but don't retry on conflict, because a conflict means that we potentially operated on stale
	// data (e.g. when calculating the updated k8s version), so rather return error and backoff
	if err := r.Client.Update(ctx, shoot); err != nil {
		r.Recorder.Event(shoot, corev1.EventTypeWarning, gardencorev1beta1.ShootMaintenanceFailed, err.Error())
		return err
	}

	if shoot.Status.LastMaintenance != nil && shoot.Status.LastMaintenance.State == gardencorev1beta1.LastOperationStateProcessing {
		patch := client.MergeFrom(shoot.DeepCopy())
		shoot.Status.LastMaintenance.State = gardencorev1beta1.LastOperationStateSucceeded
		if err := r.Client.Status().Patch(ctx, shoot, patch); err != nil {
			return err
		}
	}

	for _, reason := range reasonForImageUpdatePerPool {
		r.Recorder.Eventf(shoot, corev1.EventTypeNormal, gardencorev1beta1.ShootEventImageVersionMaintenance, "%s",
			fmt.Sprintf("Updated %s.", reason))
	}

	for _, reason := range reasonForKubernetesUpdate {
		r.Recorder.Eventf(shoot, corev1.EventTypeNormal, gardencorev1beta1.ShootEventK8sVersionMaintenance, "%s", reason)
	}

	log.Info("Shoot maintenance completed")
	return nil
}

func maintainOperation(shoot *gardencorev1beta1.Shoot) string {
	var operation string
	if hasMaintainNowAnnotation(shoot) {
		delete(shoot.Annotations, v1beta1constants.GardenerOperation)
	}

	if shoot.Status.LastOperation == nil {
		return ""
	}

	switch {
	case shoot.Status.LastOperation.State == gardencorev1beta1.LastOperationStateFailed:
		if needsRetry(shoot) {
			metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationRetry)
			delete(shoot.Annotations, v1beta1constants.FailedShootNeedsRetryOperation)
		}
	default:
		operation = getOperation(shoot)
		metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, v1beta1constants.GardenerOperation, operation)
		delete(shoot.Annotations, v1beta1constants.GardenerMaintenanceOperation)
	}

	if operation == v1beta1constants.GardenerOperationReconcile {
		return ""
	}

	return operation
}

func maintainTasks(shoot *gardencorev1beta1.Shoot, config config.ShootMaintenanceControllerConfiguration) {
	controllerutils.AddTasks(shoot.Annotations,
		v1beta1constants.ShootTaskDeployInfrastructure,
		v1beta1constants.ShootTaskDeployDNSRecordInternal,
		v1beta1constants.ShootTaskDeployDNSRecordExternal,
		v1beta1constants.ShootTaskDeployDNSRecordIngress,
	)

	if pointer.BoolDeref(config.EnableShootControlPlaneRestarter, false) {
		controllerutils.AddTasks(shoot.Annotations, v1beta1constants.ShootTaskRestartControlPlanePods)
	}

	if pointer.BoolDeref(config.EnableShootCoreAddonRestarter, false) {
		controllerutils.AddTasks(shoot.Annotations, v1beta1constants.ShootTaskRestartCoreAddons)
	}
}

// maintainMachineImages updates the machine images of a Shoot's worker pools if necessary
func maintainMachineImages(log logr.Logger, shoot *gardencorev1beta1.Shoot, cloudProfile *gardencorev1beta1.CloudProfile) ([]string, error) {
	var reasonsForUpdate []string

	controlPlaneVersion, err := semver.NewVersion(shoot.Spec.Kubernetes.Version)
	if err != nil {
		return nil, err
	}

	for i, worker := range shoot.Spec.Provider.Workers {
		workerImage := worker.Machine.Image
		workerLog := log.WithValues("worker", worker.Name, "image", workerImage.Name, "version", workerImage.Version)

		machineImageFromCloudProfile, err := determineMachineImage(cloudProfile, workerImage)
		if err != nil {
			return nil, err
		}

		kubeletVersion, err := v1beta1helper.CalculateEffectiveKubernetesVersion(controlPlaneVersion, worker.Kubernetes)
		if err != nil {
			return nil, err
		}

		filteredMachineImageVersionsFromCloudProfile := filterForArchitecture(&machineImageFromCloudProfile, worker.Machine.Architecture)
		filteredMachineImageVersionsFromCloudProfile = filterForCRI(filteredMachineImageVersionsFromCloudProfile, worker.CRI)
		filteredMachineImageVersionsFromCloudProfile = filterForKubeleteVersionConstraint(filteredMachineImageVersionsFromCloudProfile, kubeletVersion)
		shouldBeUpdated, reason, updatedMachineImage, err := shouldMachineImageBeUpdated(workerLog, *shoot.Spec.Maintenance.AutoUpdate.MachineImageVersion, filteredMachineImageVersionsFromCloudProfile, workerImage)
		if err != nil {
			return nil, err
		}

		if !shouldBeUpdated {
			continue
		}

		shoot.Spec.Provider.Workers[i].Machine.Image = updatedMachineImage

		workerLog.Info("MachineImage will be updated", "newVersion", *updatedMachineImage.Version, "reason", reason)
		reasonsForUpdate = append(reasonsForUpdate, fmt.Sprintf("Machine image of worker-pool %q upgraded from %q version %q to version %q. Reason: %s", worker.Name, workerImage.Name, *workerImage.Version, *updatedMachineImage.Version, reason))
	}

	return reasonsForUpdate, nil
}

// maintainKubernetesVersion updates a Shoot's Kubernetes version if necessary and returns the reason why an update was done
func maintainKubernetesVersion(log logr.Logger, kubernetesVersion string, autoUpdate bool, profile *gardencorev1beta1.CloudProfile, updateFunc func(string) error, name string) ([]string, error) {
	var reasonsForUpdate []string
	shouldBeUpdated, reason, isExpired, err := shouldKubernetesVersionBeUpdated(kubernetesVersion, autoUpdate, profile)
	if err != nil {
		return reasonsForUpdate, err
	}
	if !shouldBeUpdated {
		return reasonsForUpdate, nil
	}

	updatedKubernetesVersion, err := determineKubernetesVersion(kubernetesVersion, profile, isExpired)
	if err != nil {
		return reasonsForUpdate, err
	}
	if updatedKubernetesVersion == "" {
		return reasonsForUpdate, nil
	}

	err = updateFunc(updatedKubernetesVersion)
	if err != nil {
		return reasonsForUpdate, err
	}

	reason = fmt.Sprintf("For %q: Kubernetes version upgraded %q to version %q. Reason: %s", name, kubernetesVersion, updatedKubernetesVersion, reason)
	reasonsForUpdate = append(reasonsForUpdate, reason)
	log.Info("Kubernetes version will be updated", "version", kubernetesVersion, "newVersion", updatedKubernetesVersion, "reason", reason)
	return reasonsForUpdate, err
}

func determineKubernetesVersion(kubernetesVersion string, profile *gardencorev1beta1.CloudProfile, isExpired bool) (string, error) {
	// get latest version that qualifies for a patch update
	newerPatchVersionFound, latestPatchVersion, err := v1beta1helper.GetKubernetesVersionForPatchUpdate(profile, kubernetesVersion)
	if err != nil {
		return "", fmt.Errorf("failure while determining the latest Kubernetes patch version in the CloudProfile: %s", err.Error())
	}
	if newerPatchVersionFound {
		return latestPatchVersion, nil
	}
	// no newer patch version found & is expired -> forcefully update to latest patch of next minor version
	if isExpired {
		// get latest version that qualifies for a minor update
		newMinorAvailable, latestPatchVersionNewMinor, err := v1beta1helper.GetKubernetesVersionForMinorUpdate(profile, kubernetesVersion)
		if err != nil {
			return "", fmt.Errorf("failure while determining newer Kubernetes minor version in the CloudProfile: %s", err.Error())
		}
		// cannot update as there is no consecutive minor version available (e.g shoot is on 1.24.X, but there is only 1.26.X, available and not 1.25.X)
		if !newMinorAvailable {
			return "", fmt.Errorf("cannot perform minor Kubernetes version update for expired Kubernetes version %q. No suitable version found in CloudProfile - this is most likely a misconfiguration of the CloudProfile", kubernetesVersion)
		}

		return latestPatchVersionNewMinor, nil
	}
	return "", nil
}

func shouldKubernetesVersionBeUpdated(kubernetesVersion string, autoUpdate bool, profile *gardencorev1beta1.CloudProfile) (shouldBeUpdated bool, reason string, isExpired bool, error error) {
	versionExistsInCloudProfile, version, err := v1beta1helper.KubernetesVersionExistsInCloudProfile(profile, kubernetesVersion)
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

func mustMaintainNow(shoot *gardencorev1beta1.Shoot, clock clock.Clock) bool {
	return hasMaintainNowAnnotation(shoot) || gardenerutils.IsNowInEffectiveShootMaintenanceTimeWindow(shoot, clock)
}

func hasMaintainNowAnnotation(shoot *gardencorev1beta1.Shoot) bool {
	operation, ok := shoot.Annotations[v1beta1constants.GardenerOperation]
	return ok && operation == v1beta1constants.ShootOperationMaintain
}

func needsRetry(shoot *gardencorev1beta1.Shoot) bool {
	needsRetryOperation := false

	if val, ok := shoot.Annotations[v1beta1constants.FailedShootNeedsRetryOperation]; ok {
		needsRetryOperation, _ = strconv.ParseBool(val)
	}

	return needsRetryOperation
}

func getOperation(shoot *gardencorev1beta1.Shoot) string {
	var (
		operation            = v1beta1constants.GardenerOperationReconcile
		maintenanceOperation = shoot.Annotations[v1beta1constants.GardenerMaintenanceOperation]
	)

	if maintenanceOperation != "" {
		operation = maintenanceOperation
	}

	return operation
}

func filterForArchitecture(machineImageFromCloudProfile *gardencorev1beta1.MachineImage, arch *string) *gardencorev1beta1.MachineImage {
	filteredMachineImages := gardencorev1beta1.MachineImage{Name: machineImageFromCloudProfile.Name,
		Versions: []gardencorev1beta1.MachineImageVersion{}}

	for _, cloudProfileVersion := range machineImageFromCloudProfile.Versions {
		if slices.Contains(cloudProfileVersion.Architectures, *arch) {
			filteredMachineImages.Versions = append(filteredMachineImages.Versions, cloudProfileVersion)
		}
	}

	return &filteredMachineImages
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

func filterForKubeleteVersionConstraint(machineImageFromCloudProfile *gardencorev1beta1.MachineImage, kubeletVersion *semver.Version) *gardencorev1beta1.MachineImage {
	filteredMachineImages := gardencorev1beta1.MachineImage{Name: machineImageFromCloudProfile.Name,
		Versions: []gardencorev1beta1.MachineImageVersion{}}

	for _, cloudProfileVersion := range machineImageFromCloudProfile.Versions {
		if cloudProfileVersion.KubeletVersionConstraint != nil {
			// CloudProfile cannot contain an invalid kubeletVersionConstraint
			constraint, _ := semver.NewConstraint(*cloudProfileVersion.KubeletVersionConstraint)
			if !constraint.Check(kubeletVersion) {
				continue
			}
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
	machineImagesFound, machineImageFromCloudProfile := v1beta1helper.DetermineMachineImageForName(cloudProfile, shootMachineImage.Name)
	if !machineImagesFound {
		return gardencorev1beta1.MachineImage{}, fmt.Errorf("failure while determining the default machine image in the CloudProfile: no machineImage with name %q (specified in shoot) could be found in the cloudProfile %q", shootMachineImage.Name, cloudProfile.Name)
	}

	return machineImageFromCloudProfile, nil
}

// shouldMachineImageBeUpdated determines if a machine image should be updated based on whether it exists in the CloudProfile, auto update applies or a force update is required.
func shouldMachineImageBeUpdated(log logr.Logger, autoUpdateMachineImageVersion bool, machineImage *gardencorev1beta1.MachineImage, shootMachineImage *gardencorev1beta1.ShootMachineImage) (shouldBeUpdated bool, reason string, updatedMachineImage *gardencorev1beta1.ShootMachineImage, error error) {
	versionExistsInCloudProfile, versionIndex := v1beta1helper.ShootMachineImageVersionExists(*machineImage, *shootMachineImage)
	var reasonForUpdate string

	forceUpdateRequired := ForceMachineImageUpdateRequired(shootMachineImage, *machineImage)
	if !versionExistsInCloudProfile || autoUpdateMachineImageVersion || forceUpdateRequired {
		// safe operation, as Shoot machine image version can only be a valid semantic version
		shootSemanticVersion := *semver.MustParse(*shootMachineImage.Version)

		// get latest version qualifying for Shoot machine image update
		qualifyingVersionFound, latestShootMachineImage, err := v1beta1helper.GetLatestQualifyingShootMachineImage(*machineImage, v1beta1helper.FilterLowerVersion(shootSemanticVersion))
		if err != nil {
			return false, "", nil, fmt.Errorf("an error occured while determining the latest Shoot Machine Image for machine image %q: %w", machineImage.Name, err)
		}

		// this is a special case when a Shoot is using a preview version. Preview versions should not be updated-to and are therefore not part of the qualifying versions.
		// if no qualifying version can be found and the Shoot is already using a preview version, then do nothing.
		if !qualifyingVersionFound && versionExistsInCloudProfile && machineImage.Versions[versionIndex].Classification != nil && *machineImage.Versions[versionIndex].Classification == gardencorev1beta1.ClassificationPreview {
			log.V(1).Info("MachineImage update not required, shoot worker is already using preview version")
			return false, "", nil, nil
		}

		// otherwise, there should always be a qualifying version (at least the Shoot's machine image version itself).
		if !qualifyingVersionFound {
			return false, "", nil, fmt.Errorf("no latest qualifying Shoot machine image could be determined for machine image %q. Either the machine image is reaching end of life and migration to another machine image is required or there is a misconfiguration in the CloudProfile. If it is the latter, make sure the machine image in the CloudProfile has at least one version that is not expired, not in preview and greater or equal to the current Shoot image version %q", machineImage.Name, *shootMachineImage.Version)
		}

		if *latestShootMachineImage.Version == *shootMachineImage.Version {
			log.V(1).Info("MachineImage update not required, shoot worker is already up to date")
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

// updateToContainerd updates the CRI of a Shoot's worker pools to containerd
func updateToContainerd(log logr.Logger, shoot *gardencorev1beta1.Shoot, reason string) ([]string, error) {
	var reasonsForUpdate []string

	for i, worker := range shoot.Spec.Provider.Workers {
		workerLog := log.WithValues("worker", worker.Name)

		if worker.CRI == nil || worker.CRI.Name != gardencorev1beta1.CRINameDocker {
			continue
		}

		shoot.Spec.Provider.Workers[i].CRI.Name = gardencorev1beta1.CRINameContainerD

		workerLog.Info("CRI will be updated to containerd", "reason", reason)
		reasonsForUpdate = append(reasonsForUpdate, fmt.Sprintf("CRI of worker-pool %q upgraded from %q to %q. Reason: %s", worker.Name, gardencorev1beta1.CRINameDocker, gardencorev1beta1.CRINameContainerD, reason))
	}

	return reasonsForUpdate, nil
}

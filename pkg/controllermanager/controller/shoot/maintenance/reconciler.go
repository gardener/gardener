// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package maintenance

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/clock"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	controllermanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/controllermanager/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	admissionpluginsvalidation "github.com/gardener/gardener/pkg/utils/validation/admissionplugins"
	featuresvalidation "github.com/gardener/gardener/pkg/utils/validation/features"
	versionutils "github.com/gardener/gardener/pkg/utils/version"
)

// Reconciler reconciles Shoots and maintains them by updating versions or triggering operations.
type Reconciler struct {
	Client   client.Client
	Config   controllermanagerconfigv1alpha1.ShootMaintenanceControllerConfiguration
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

// updateResult represents the result of a Kubernetes or Machine image maintenance operation
// Such maintenance operations can fail if a version must be updated, but the GCM cannot find a suitable version to update to.
// Note: the updates might still be rejected by APIServer validation.
type updateResult struct {
	description  string
	reason       string
	isSuccessful bool
}

func (r *Reconciler) reconcile(ctx context.Context, log logr.Logger, shoot *gardencorev1beta1.Shoot) error {
	log.Info("Maintaining Shoot")

	var (
		maintainedShoot = shoot.DeepCopy()
		// for maintenance operations unrelated to machine images and Kubernetes versions
		operations []string
		err        error
	)

	workerToKubernetesUpdate := make(map[string]updateResult)
	workerToMachineImageUpdate := make(map[string]updateResult)

	cloudProfile, err := gardenerutils.GetCloudProfile(ctx, r.Client, shoot)
	if err != nil {
		return err
	}

	if !v1beta1helper.IsWorkerless(shoot) {
		workerToMachineImageUpdate, err = maintainMachineImages(log, maintainedShoot, cloudProfile)
		if err != nil {
			// continue execution to allow the kubernetes version update
			log.Error(err, "Failed to maintain Shoot machine images")
		}
	}

	kubernetesControlPlaneUpdate, err := maintainKubernetesVersion(log, maintainedShoot.Spec.Kubernetes.Version, maintainedShoot.Spec.Maintenance.AutoUpdate.KubernetesVersion, cloudProfile, func(v string) (string, error) {
		maintainedShoot.Spec.Kubernetes.Version = v
		return v, nil
	})
	if err != nil {
		// continue execution to allow the machine image version update and Kubernetes updates to worker pools
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

	// Set the .spec.kubernetes.kubeAPIServer.oidcConfig.clientAuthentication field to nil, when Shoot cluster is being forcefully updated to K8s >= 1.31.
	// Gardener forbids setting the field for Shoots with K8s 1.31+. See https://github.com/gardener/gardener/pull/10253
	{
		if versionutils.ConstraintK8sLess131.Check(oldShootKubernetesVersion) && versionutils.ConstraintK8sGreaterEqual131.Check(shootKubernetesVersion) {
			if maintainedShoot.Spec.Kubernetes.KubeAPIServer != nil && maintainedShoot.Spec.Kubernetes.KubeAPIServer.OIDCConfig != nil &&
				maintainedShoot.Spec.Kubernetes.KubeAPIServer.OIDCConfig.ClientAuthentication != nil {
				maintainedShoot.Spec.Kubernetes.KubeAPIServer.OIDCConfig.ClientAuthentication = nil

				reason := ".spec.kubernetes.kubeAPIServer.oidcConfig.clientAuthentication is set to nil. Reason: The field was no-op since its introduction and can no longer be enabled for Shoot clusters using Kubernetes version 1.31+"
				operations = append(operations, reason)
			}
		}
	}

	// Set the .spec.kubernetes.kubeAPIServer.oidcConfig field to nil, when Shoot cluster is being forcefully updated to K8s >= 1.32.
	// Gardener forbids setting the field for Shoots with K8s 1.32+. See https://github.com/gardener/gardener/pull/10666
	{
		oldK8sLess132, _ := versionutils.CheckVersionMeetsConstraint(oldShootKubernetesVersion.String(), "< 1.32")
		newK8sGreaterEqual132, _ := versionutils.CheckVersionMeetsConstraint(shootKubernetesVersion.String(), ">= 1.32")
		if oldK8sLess132 && newK8sGreaterEqual132 {
			if maintainedShoot.Spec.Kubernetes.KubeAPIServer != nil && maintainedShoot.Spec.Kubernetes.KubeAPIServer.OIDCConfig != nil {
				maintainedShoot.Spec.Kubernetes.KubeAPIServer.OIDCConfig = nil

				reason := ".spec.kubernetes.kubeAPIServer.oidcConfig is set to nil. Reason: The field has been deprecated in favor of structured authentication and can no longer be enabled for Shoot clusters using Kubernetes version 1.32+"
				operations = append(operations, reason)
			}
		}
	}

	// Now it's time to update worker pool kubernetes version if specified
	for i, pool := range maintainedShoot.Spec.Provider.Workers {
		if pool.Kubernetes == nil || pool.Kubernetes.Version == nil {
			continue
		}

		workerLog := log.WithValues("worker", pool.Name)
		workerKubernetesUpdate, err := maintainKubernetesVersion(workerLog, *pool.Kubernetes.Version, maintainedShoot.Spec.Maintenance.AutoUpdate.KubernetesVersion, cloudProfile, func(v string) (string, error) {
			workerPoolSemver, err := semver.NewVersion(v)
			if err != nil {
				return "", err
			}
			// If during autoupdate a worker pool kubernetes gets forcefully updated to the next minor which might be higher than the same minor of the shoot, take this
			if workerPoolSemver.GreaterThan(shootKubernetesVersion) {
				workerPoolSemver = shootKubernetesVersion
			}
			v = workerPoolSemver.String()
			maintainedShoot.Spec.Provider.Workers[i].Kubernetes.Version = &v
			return v, nil
		})
		if err != nil {
			// continue execution to allow other maintenance activities to continue
			workerLog.Error(err, "Could not maintain Kubernetes version for worker pool")
		}

		if workerKubernetesUpdate != nil {
			result := updateResult{
				reason: workerKubernetesUpdate.reason,
			}
			result.isSuccessful = workerKubernetesUpdate.isSuccessful
			result.description = workerKubernetesUpdate.description
			workerToKubernetesUpdate[pool.Name] = result
		}
	}

	if reasons := maintainFeatureGatesForShoot(maintainedShoot); len(reasons) > 0 {
		operations = append(operations, reasons...)
	}

	if reasons := maintainAdmissionPluginsForShoot(maintainedShoot); len(reasons) > 0 {
		operations = append(operations, reasons...)
	}

	// Set the swap behavior to `LimitedSwap`, when the shoot cluster and/or worker pool is updated to k8s version >= 1.30.
	{
		if versionutils.ConstraintK8sLess130.Check(oldShootKubernetesVersion) &&
			versionutils.ConstraintK8sGreaterEqual130.Check(shootKubernetesVersion) &&
			maintainedShoot.Spec.Kubernetes.Kubelet != nil {
			operations = append(operations, setLimitedSwap(maintainedShoot.Spec.Kubernetes.Kubelet, "spec.kubernetes.kubelet.memorySwap.swapBehavior")...)
		}

		for i := range maintainedShoot.Spec.Provider.Workers {
			if maintainedShoot.Spec.Provider.Workers[i].Kubernetes != nil && maintainedShoot.Spec.Provider.Workers[i].Kubernetes.Kubelet != nil {
				kubeletVersion := ptr.Deref(maintainedShoot.Spec.Provider.Workers[i].Kubernetes.Version, maintainedShoot.Spec.Kubernetes.Version)
				kubeletSemverVersion, err := semver.NewVersion(kubeletVersion)
				if err != nil {
					return fmt.Errorf("error parsing kubelet version for worker pool %q: %w", maintainedShoot.Spec.Provider.Workers[i].Name, err)
				}

				if versionutils.ConstraintK8sGreaterEqual130.Check(kubeletSemverVersion) {
					operations = append(operations, setLimitedSwap(maintainedShoot.Spec.Provider.Workers[i].Kubernetes.Kubelet, fmt.Sprintf("spec.provider.workers[%d].kubernetes.kubelet.memorySwap.swapBehavior", i))...)
				}
			}
		}
	}

	// Move kubernetes.kubelet.systemReserved for a Shoot or worker pool to kubernetes.kubelet.kubeReserved, when Shoot cluster is being forcefully updated to K8s >= 1.31.
	// Gardener forbids specifying kubernetes.kubelet.systemReserved for Shoots with K8s 1.31+. See https://github.com/gardener/gardener/pull/10290
	{
		if versionutils.ConstraintK8sLess131.Check(oldShootKubernetesVersion) && versionutils.ConstraintK8sGreaterEqual131.Check(shootKubernetesVersion) {
			if maintainedShoot.Spec.Kubernetes.Kubelet != nil && maintainedShoot.Spec.Kubernetes.Kubelet.SystemReserved != nil {
				maintainedShoot.Spec.Kubernetes.Kubelet.KubeReserved = v1beta1helper.SumResourceReservations(maintainedShoot.Spec.Kubernetes.Kubelet.KubeReserved, maintainedShoot.Spec.Kubernetes.Kubelet.SystemReserved)
				maintainedShoot.Spec.Kubernetes.Kubelet.SystemReserved = nil

				reason := ".spec.kubernetes.kubelet.systemReserved is added to .spec.kubernetes.kubelet.kubeReserved. Reason: The systemReserved field is forbidden for Shoot clusters using Kubernetes version 1.31+, its value has to be added to kubeReserved"
				operations = append(operations, reason)
			}
		}

		for i := range maintainedShoot.Spec.Provider.Workers {
			if maintainedShoot.Spec.Provider.Workers[i].Kubernetes != nil && maintainedShoot.Spec.Provider.Workers[i].Kubernetes.Kubelet != nil &&
				maintainedShoot.Spec.Provider.Workers[i].Kubernetes.Kubelet.SystemReserved != nil {
				kubeletVersion := ptr.Deref(maintainedShoot.Spec.Provider.Workers[i].Kubernetes.Version, maintainedShoot.Spec.Kubernetes.Version)
				kubeletSemverVersion, err := semver.NewVersion(kubeletVersion)
				if err != nil {
					return fmt.Errorf("error parsing kubelet version for worker pool %q: %w", maintainedShoot.Spec.Provider.Workers[i].Name, err)
				}

				if versionutils.ConstraintK8sGreaterEqual131.Check(kubeletSemverVersion) {
					maintainedShoot.Spec.Provider.Workers[i].Kubernetes.Kubelet.KubeReserved = v1beta1helper.SumResourceReservations(maintainedShoot.Spec.Provider.Workers[i].Kubernetes.Kubelet.KubeReserved, maintainedShoot.Spec.Provider.Workers[i].Kubernetes.Kubelet.SystemReserved)
					maintainedShoot.Spec.Provider.Workers[i].Kubernetes.Kubelet.SystemReserved = nil

					reason := fmt.Sprintf(".spec.provider.workers[%[1]d].kubernetes.kubelet.systemReserved is added to .spec.provider.workers[%[1]d].kubernetes.kubelet.kubeReserved. Reason: The systemReserved field is forbidden for Shoot clusters using Kubernetes version 1.31+, its value has to be added to kubeReserved", i)
					operations = append(operations, reason)
				}
			}
		}
	}

	operation := maintainOperation(maintainedShoot)
	if operation != "" {
		operations = append(operations, fmt.Sprintf("Added %q operation annotation", operation))
	}

	requirePatch := len(operations) > 0 || kubernetesControlPlaneUpdate != nil || len(workerToKubernetesUpdate) > 0 || len(workerToMachineImageUpdate) > 0
	if requirePatch {
		patch := client.MergeFrom(shoot.DeepCopy())

		// make sure to include both successful and failed maintenance operations
		description, failureReason := buildMaintenanceMessages(
			kubernetesControlPlaneUpdate,
			workerToKubernetesUpdate,
			workerToMachineImageUpdate,
		)

		// append also other maintenance operation
		if len(operations) > 0 {
			description = fmt.Sprintf("%s, %s", description, strings.Join(operations, ", "))
		}

		shoot.Status.LastMaintenance = &gardencorev1beta1.LastMaintenance{
			Description:   description,
			TriggeredTime: metav1.Time{Time: r.Clock.Now()},
			State:         gardencorev1beta1.LastOperationStateProcessing,
		}

		// if any maintenance operation failed, set the status to 'Failed' and retry in the next maintenance cycle
		if failureReason != "" {
			shoot.Status.LastMaintenance.State = gardencorev1beta1.LastOperationStateFailed
			shoot.Status.LastMaintenance.FailureReason = &failureReason
		}

		// First dry run the update call to check if it can be executed successfully (maintenance might yield a Shoot configuration that is rejected by the ApiServer).
		// If the dry run fails, the shoot maintenance is marked as failed and is retried only in
		// next maintenance window.
		if err := r.Client.Update(ctx, maintainedShoot.DeepCopy(), &client.UpdateOptions{
			DryRun: []string{metav1.DryRunAll},
		}); err != nil {
			// If shoot maintenance is triggered by `gardener.cloud/operation=maintain` annotation and if it fails in dry run,
			// `maintain` operation annotation needs to be removed so that if reason for failure is fixed and maintenance is triggered
			// again via `maintain` operation annotation then it should not fail with the reason that annotation is already present.
			// Removal of annotation during shoot status patch is possible cause only spec is kept in original form during status update
			// https://github.com/gardener/gardener/blob/a2f7de0badaae6170d7b9b84c163b8cab43a84d2/pkg/apiserver/registry/core/shoot/strategy.go#L258-L267
			if hasMaintainNowAnnotation(shoot) {
				delete(shoot.Annotations, v1beta1constants.GardenerOperation)
			}
			shoot.Status.LastMaintenance.Description = "Maintenance failed"
			shoot.Status.LastMaintenance.State = gardencorev1beta1.LastOperationStateFailed
			shoot.Status.LastMaintenance.FailureReason = ptr.To(fmt.Sprintf("Updates to the Shoot failed to be applied: %s", err.Error()))
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

	// if the maintenance patch is not required and the last maintenance operation state is failed,
	// this means the maintenance was retried and succeeded. Alternatively, changes could have been made
	// outside of the maintenance window to fix the maintenance error. In either case, remove the failed state.
	if !requirePatch && shoot.Status.LastMaintenance != nil && shoot.Status.LastMaintenance.State == gardencorev1beta1.LastOperationStateFailed {
		patch := client.MergeFrom(shoot.DeepCopy())
		shoot.Status.LastMaintenance.State = gardencorev1beta1.LastOperationStateSucceeded
		shoot.Status.LastMaintenance.Description = "Maintenance succeeded"
		shoot.Status.LastMaintenance.FailureReason = nil

		if err := r.Client.Status().Patch(ctx, shoot, patch); err != nil {
			return err
		}
	}

	if shoot.Status.LastMaintenance != nil && shoot.Status.LastMaintenance.State == gardencorev1beta1.LastOperationStateProcessing {
		patch := client.MergeFrom(shoot.DeepCopy())
		shoot.Status.LastMaintenance.State = gardencorev1beta1.LastOperationStateSucceeded

		if err := r.Client.Status().Patch(ctx, shoot, patch); err != nil {
			return err
		}
	}

	// make sure to report (partial) maintenance failures
	if kubernetesControlPlaneUpdate != nil {
		if kubernetesControlPlaneUpdate.isSuccessful {
			r.Recorder.Eventf(shoot, corev1.EventTypeNormal, gardencorev1beta1.ShootEventK8sVersionMaintenance, "%s", fmt.Sprintf("Control Plane: %s. Reason: %s.", kubernetesControlPlaneUpdate.description, kubernetesControlPlaneUpdate.reason))
		} else {
			r.Recorder.Eventf(shoot, corev1.EventTypeWarning, gardencorev1beta1.ShootEventK8sVersionMaintenance, "%s", fmt.Sprintf("Control Plane: Kubernetes version maintenance failed. Reason for update: %s. Error: %v", kubernetesControlPlaneUpdate.reason, kubernetesControlPlaneUpdate.description))
		}
	}

	r.recordMaintenanceEventsForPool(workerToKubernetesUpdate, shoot, gardencorev1beta1.ShootEventK8sVersionMaintenance, "Kubernetes")
	r.recordMaintenanceEventsForPool(workerToMachineImageUpdate, shoot, gardencorev1beta1.ShootEventImageVersionMaintenance, "Machine image")

	log.Info("Shoot maintenance completed")
	return nil
}

// buildMaintenanceMessages builds a combined message containing the performed maintenance operations over all worker pools. If the maintenance operation failed, the description
// contains an indication for the failure and the reason the update was triggered. Details for failed maintenance operations are returned in the second return string.
func buildMaintenanceMessages(kubernetesControlPlaneUpdate *updateResult, workerToKubernetesUpdate map[string]updateResult, workerToMachineImageUpdate map[string]updateResult) (string, string) {
	countSuccessfulOperations := 0
	countFailedOperations := 0
	description := ""
	failureReason := ""

	if kubernetesControlPlaneUpdate != nil {
		if kubernetesControlPlaneUpdate.isSuccessful {
			countSuccessfulOperations++
			description = fmt.Sprintf("%s, %s", description, fmt.Sprintf("Control Plane: %s. Reason: %s", kubernetesControlPlaneUpdate.description, kubernetesControlPlaneUpdate.reason))
		} else {
			countFailedOperations++
			description = fmt.Sprintf("%s, %s", description, fmt.Sprintf("Control Plane: Kubernetes version update failed. Reason for update: %s", kubernetesControlPlaneUpdate.reason))
			failureReason = fmt.Sprintf("%s, Control Plane: Kubernetes maintenance failure due to: %s", failureReason, kubernetesControlPlaneUpdate.description)
		}
	}

	for worker, result := range workerToKubernetesUpdate {
		if result.isSuccessful {
			countSuccessfulOperations++
			description = fmt.Sprintf("%s, %s", description, fmt.Sprintf("Worker pool %q: %s. Reason: %s", worker, result.description, result.reason))
			continue
		}

		countFailedOperations++
		description = fmt.Sprintf("%s, %s", description, fmt.Sprintf("Worker pool %q: Kubernetes version maintenance failed. Reason for update: %s", worker, result.reason))
		failureReason = fmt.Sprintf("%s, Worker pool %q: Kubernetes maintenance failure due to: %s", failureReason, worker, result.description)
	}

	for worker, result := range workerToMachineImageUpdate {
		if result.isSuccessful {
			countSuccessfulOperations++
			description = fmt.Sprintf("%s, %s", description, fmt.Sprintf("Worker pool %q: %s. Reason: %s", worker, result.description, result.reason))
			continue
		}

		countFailedOperations++
		description = fmt.Sprintf("%s, %s", description, fmt.Sprintf("Worker pool %q: machine image version maintenance failed. Reason for update: %s", worker, result.reason))
		failureReason = fmt.Sprintf("%s, Worker pool %q: %s", failureReason, worker, result.description)
	}

	description = strings.TrimPrefix(description, ", ")
	failureReason = strings.TrimPrefix(failureReason, ", ")

	if countFailedOperations == 0 {
		return fmt.Sprintf("All maintenance operations successful. %s", description), failureReason
	}

	return fmt.Sprintf("(%d/%d) maintenance operations successful. %s", countSuccessfulOperations, countSuccessfulOperations+countFailedOperations, description), failureReason
}

// recordMaintenanceEventsForPool records dedicated events for each failed/succeeded maintenance operation per pool
func (r *Reconciler) recordMaintenanceEventsForPool(workerToUpdateResult map[string]updateResult, shoot *gardencorev1beta1.Shoot, eventType string, maintenanceType string) {
	for worker, reason := range workerToUpdateResult {
		if reason.isSuccessful {
			r.Recorder.Eventf(shoot, corev1.EventTypeNormal, eventType, "%s", fmt.Sprintf("Worker pool %q: %v. Reason: %s.",
				worker, reason.description, reason.reason))
			continue
		}

		r.Recorder.Eventf(shoot, corev1.EventTypeWarning, eventType, "%s", fmt.Sprintf("Worker pool %q: %s version maintenance failed. Reason for update: %s. Error: %v",
			worker, maintenanceType, reason.reason, reason.description))
	}
}

func maintainOperation(shoot *gardencorev1beta1.Shoot) string {
	var operation string
	if hasMaintainNowAnnotation(shoot) {
		delete(shoot.Annotations, v1beta1constants.GardenerOperation)
	}

	if shoot.Status.LastOperation == nil {
		return ""
	}

	switch shoot.Status.LastOperation.State {
	case gardencorev1beta1.LastOperationStateFailed:
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

func maintainTasks(shoot *gardencorev1beta1.Shoot, config controllermanagerconfigv1alpha1.ShootMaintenanceControllerConfiguration) {
	controllerutils.AddTasks(shoot.Annotations,
		v1beta1constants.ShootTaskDeployInfrastructure,
		v1beta1constants.ShootTaskDeployDNSRecordInternal,
		v1beta1constants.ShootTaskDeployDNSRecordExternal,
		v1beta1constants.ShootTaskDeployDNSRecordIngress,
	)

	if ptr.Deref(config.EnableShootControlPlaneRestarter, false) {
		controllerutils.AddTasks(shoot.Annotations, v1beta1constants.ShootTaskRestartControlPlanePods)
	}

	if ptr.Deref(config.EnableShootCoreAddonRestarter, false) {
		controllerutils.AddTasks(shoot.Annotations, v1beta1constants.ShootTaskRestartCoreAddons)
	}
}

// maintainMachineImages updates the machine images of a Shoot's worker pools if necessary
func maintainMachineImages(log logr.Logger, shoot *gardencorev1beta1.Shoot, cloudProfile *gardencorev1beta1.CloudProfile) (map[string]updateResult, error) {
	maintenanceResults := make(map[string]updateResult)

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

		// first check if the machine image version should be updated
		shouldBeUpdated, reason, isExpired := shouldMachineImageVersionBeUpdated(workerImage, filteredMachineImageVersionsFromCloudProfile, *shoot.Spec.Maintenance.AutoUpdate.MachineImageVersion)
		if !shouldBeUpdated {
			continue
		}

		updatedMachineImageVersion, err := determineMachineImageVersion(workerImage, filteredMachineImageVersionsFromCloudProfile, isExpired)
		if err != nil {
			log.Error(err, "Maintenance of machine image failed", "workerPool", worker.Name, "machineImage", workerImage.Name)
			maintenanceResults[worker.Name] = updateResult{
				description:  fmt.Sprintf("failed to update machine image %q: %s", workerImage.Name, err.Error()),
				reason:       reason,
				isSuccessful: false,
			}
			continue
		}
		// current version is already the latest
		if updatedMachineImageVersion == "" {
			continue
		}

		workerLog.Info("MachineImage will be updated", "newVersion", updatedMachineImageVersion, "reason", reason)
		maintenanceResults[worker.Name] = updateResult{
			description:  fmt.Sprintf("Updated machine image %q from %q to %q", workerImage.Name, *workerImage.Version, updatedMachineImageVersion),
			reason:       reason,
			isSuccessful: true,
		}

		// update the machine image version
		shoot.Spec.Provider.Workers[i].Machine.Image.Version = &updatedMachineImageVersion
	}

	return maintenanceResults, nil
}

// maintainKubernetesVersion updates the Kubernetes version if necessary and returns the reason why an update was done
func maintainKubernetesVersion(log logr.Logger, kubernetesVersion string, autoUpdate bool, profile *gardencorev1beta1.CloudProfile, updateFunc func(string) (string, error)) (*updateResult, error) {
	shouldBeUpdated, reason, isExpired, err := shouldKubernetesVersionBeUpdated(kubernetesVersion, autoUpdate, profile)
	if err != nil {
		return nil, err
	}
	if !shouldBeUpdated {
		return nil, nil
	}

	updatedKubernetesVersion, err := determineKubernetesVersion(kubernetesVersion, profile, isExpired)
	if err != nil {
		return &updateResult{
			description:  fmt.Sprintf("could not determine higher suitable version than %q: %v", kubernetesVersion, err),
			reason:       reason,
			isSuccessful: false,
		}, err
	}
	// current version is already the latest
	if updatedKubernetesVersion == "" {
		return nil, nil
	}

	// In case the updatedKubernetesVersion for workerpool is higher than the controlplane version, actualUpdatedKubernetesVersion is set to controlplane version
	actualUpdatedKubernetesVersion, err := updateFunc(updatedKubernetesVersion)
	if err != nil {
		return &updateResult{
			description:  err.Error(),
			reason:       reason,
			isSuccessful: false,
		}, err
	}

	log.Info("Kubernetes version will be updated", "version", kubernetesVersion, "newVersion", actualUpdatedKubernetesVersion, "reason", reason)
	return &updateResult{
		description:  fmt.Sprintf("Updated Kubernetes version from %q to %q", kubernetesVersion, actualUpdatedKubernetesVersion),
		reason:       reason,
		isSuccessful: true,
	}, nil
}

func determineKubernetesVersion(kubernetesVersion string, profile *gardencorev1beta1.CloudProfile, isExpired bool) (string, error) {
	getHigherVersionAutoUpdate := v1beta1helper.GetLatestVersionForPatchAutoUpdate
	getHigherVersionForceUpdate := v1beta1helper.GetVersionForForcefulUpdateToConsecutiveMinor

	version, err := determineVersionForStrategy(profile.Spec.Kubernetes.Versions, kubernetesVersion, getHigherVersionAutoUpdate, getHigherVersionForceUpdate, isExpired)
	if err != nil {
		return "", err
	}
	return version, nil
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
		updateReason = "Automatic update of Kubernetes version configured"
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
	filteredMachineImages := gardencorev1beta1.MachineImage{
		Name:           machineImageFromCloudProfile.Name,
		UpdateStrategy: machineImageFromCloudProfile.UpdateStrategy,
		Versions:       []gardencorev1beta1.MachineImageVersion{},
	}

	for _, cloudProfileVersion := range machineImageFromCloudProfile.Versions {
		if slices.Contains(cloudProfileVersion.Architectures, *arch) {
			filteredMachineImages.Versions = append(filteredMachineImages.Versions, cloudProfileVersion)
		}
	}

	return &filteredMachineImages
}

func filterForCRI(machineImageFromCloudProfile *gardencorev1beta1.MachineImage, workerCRI *gardencorev1beta1.CRI) *gardencorev1beta1.MachineImage {
	if workerCRI == nil {
		return filterForCRI(machineImageFromCloudProfile, &gardencorev1beta1.CRI{Name: gardencorev1beta1.CRINameContainerD})
	}

	filteredMachineImages := gardencorev1beta1.MachineImage{
		Name:           machineImageFromCloudProfile.Name,
		UpdateStrategy: machineImageFromCloudProfile.UpdateStrategy,
		Versions:       []gardencorev1beta1.MachineImageVersion{},
	}

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
	filteredMachineImages := gardencorev1beta1.MachineImage{
		Name:           machineImageFromCloudProfile.Name,
		UpdateStrategy: machineImageFromCloudProfile.UpdateStrategy,
		Versions:       []gardencorev1beta1.MachineImageVersion{},
	}

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

func shouldMachineImageVersionBeUpdated(shootMachineImage *gardencorev1beta1.ShootMachineImage, machineImage *gardencorev1beta1.MachineImage, autoUpdate bool) (shouldBeUpdated bool, reason string, isExpired bool) {
	versionExistsInCloudProfile, versionIndex := v1beta1helper.ShootMachineImageVersionExists(*machineImage, *shootMachineImage)

	var updateReason string
	if !versionExistsInCloudProfile {
		updateReason = "Version does not exist in CloudProfile"
		return true, updateReason, true
	}

	if ExpirationDateExpired(machineImage.Versions[versionIndex].ExpirationDate) {
		updateReason = fmt.Sprintf("Machine image version expired - force update required (image update strategy: %s)", *machineImage.UpdateStrategy)
		return true, updateReason, true
	}

	if autoUpdate {
		updateReason = fmt.Sprintf("Automatic update of the machine image version is configured (image update strategy: %s)", *machineImage.UpdateStrategy)
		return true, updateReason, false
	}

	return false, "", false
}

// GetHigherVersion takes a slice of versions and returns if higher suitable version could be found, the version or an error
type GetHigherVersion func(versions []gardencorev1beta1.ExpirableVersion, currentVersion string) (bool, string, error)

func determineMachineImageVersion(shootMachineImage *gardencorev1beta1.ShootMachineImage, machineImage *gardencorev1beta1.MachineImage, isExpired bool) (string, error) {
	var (
		getHigherVersionAutoUpdate  GetHigherVersion
		getHigherVersionForceUpdate GetHigherVersion
	)

	switch *machineImage.UpdateStrategy {
	case gardencorev1beta1.UpdateStrategyPatch:
		getHigherVersionAutoUpdate = v1beta1helper.GetLatestVersionForPatchAutoUpdate
		getHigherVersionForceUpdate = v1beta1helper.GetVersionForForcefulUpdateToNextHigherMinor
	case gardencorev1beta1.UpdateStrategyMinor:
		getHigherVersionAutoUpdate = v1beta1helper.GetLatestVersionForMinorAutoUpdate
		getHigherVersionForceUpdate = v1beta1helper.GetVersionForForcefulUpdateToNextHigherMajor
	default:
		// auto-update strategy: "major"
		getHigherVersionAutoUpdate = v1beta1helper.GetOverallLatestVersionForAutoUpdate
		// cannot force update the overall latest version if it is expired
		getHigherVersionForceUpdate = func(_ []gardencorev1beta1.ExpirableVersion, _ string) (bool, string, error) {
			return false, "", fmt.Errorf("either the machine image %q is reaching end of life and migration to another machine image is required or there is a misconfiguration in the CloudProfile. If it is the latter, make sure the machine image in the CloudProfile has at least one version that is not expired, not in preview and greater or equal to the current Shoot image version %q", shootMachineImage.Name, *shootMachineImage.Version)
		}
	}

	version, err := determineVersionForStrategy(
		v1beta1helper.ToExpirableVersions(machineImage.Versions),
		*shootMachineImage.Version,
		getHigherVersionAutoUpdate,
		getHigherVersionForceUpdate,
		isExpired)
	if err != nil {
		return version, fmt.Errorf("failed to determine the target version for maintenance of machine image %q with strategy %q: %w", machineImage.Name, *machineImage.UpdateStrategy, err)
	}

	return version, nil
}

func determineVersionForStrategy(expirableVersions []gardencorev1beta1.ExpirableVersion, currentVersion string, getHigherVersionAutoUpdate GetHigherVersion, getHigherVersionForceUpdate GetHigherVersion, isCurrentVersionExpired bool) (string, error) {
	higherQualifyingVersionFound, latestVersionForMajor, err := getHigherVersionAutoUpdate(expirableVersions, currentVersion)
	if err != nil {
		return "", fmt.Errorf("failed to determine a higher patch version for automatic update: %w", err)
	}

	if higherQualifyingVersionFound {
		return latestVersionForMajor, nil
	}

	// The current version is already up-to date
	//  - Kubernetes version / Auto update strategy "patch": the latest patch version for the current minor version
	//  - Auto update strategy "minor": the latest patch and minor version for the current major version
	//  - Auto update strategy "major": the latest overall version
	if !isCurrentVersionExpired {
		return "", nil
	}

	// The version is already the latest version according to the strategy, but is expired. Force update.
	forceUpdateVersionAvailable, versionForForceUpdate, err := getHigherVersionForceUpdate(expirableVersions, currentVersion)
	if err != nil {
		return "", fmt.Errorf("failed to determine version for forceful update: %w", err)
	}

	// Unable to force update
	//  - Kubernetes version: no consecutive minor version available (e.g. shoot is on 1.24.X, but there is only 1.26.X, available and not 1.25.X)
	//  - Auto update strategy "patch": no higher next minor version available (e.g. shoot is on 1.0.X, but there is only 2.2.X, available and not 1.X.X)
	//  - Auto update strategy "minor": no higher next major version available (e.g. shoot is on 576.3.0, but there is no higher major version available)
	//  - Auto update strategy "major": already on latest overall version, but the latest version is expired. EOL for image or CloudProfile misconfiguration.
	if !forceUpdateVersionAvailable {
		return "", fmt.Errorf("cannot perform forceful update of expired version %q. No suitable version found in CloudProfile - this is most likely a misconfiguration of the CloudProfile", currentVersion)
	}

	return versionForForceUpdate, nil
}

// ExpirationDateExpired returns if now is equal or after the given expirationDate
func ExpirationDateExpired(timestamp *metav1.Time) bool {
	if timestamp == nil {
		return false
	}
	return time.Now().UTC().After(timestamp.Time) || time.Now().UTC().Equal(timestamp.Time)
}

// setLimitedSwap sets the swap behavior to `LimitedSwap` if it's currently set to `UnlimitedSwap`
func setLimitedSwap(kubelet *gardencorev1beta1.KubeletConfig, reason string) []string {
	var reasonsForUpdate []string

	if kubelet.MemorySwap != nil &&
		kubelet.MemorySwap.SwapBehavior != nil &&
		*kubelet.MemorySwap.SwapBehavior == gardencorev1beta1.UnlimitedSwap {
		kubelet.MemorySwap.SwapBehavior = ptr.To(gardencorev1beta1.LimitedSwap)
		reasonsForUpdate = append(reasonsForUpdate, reason+" is set to 'LimitedSwap'. Reason: 'UnlimitedSwap' cannot be used for Kubernetes version 1.30 and higher.")
	}

	return reasonsForUpdate
}

func maintainFeatureGatesForShoot(shoot *gardencorev1beta1.Shoot) []string {
	var reasons []string

	if shoot.Spec.Kubernetes.KubeAPIServer != nil && shoot.Spec.Kubernetes.KubeAPIServer.FeatureGates != nil {
		if reason := maintainFeatureGates(&shoot.Spec.Kubernetes.KubeAPIServer.KubernetesConfig, shoot.Spec.Kubernetes.Version, "spec.kubernetes.kubeAPIServer.featureGates"); len(reason) > 0 {
			reasons = append(reasons, reason...)
		}
	}

	if shoot.Spec.Kubernetes.KubeControllerManager != nil && shoot.Spec.Kubernetes.KubeControllerManager.FeatureGates != nil {
		if reason := maintainFeatureGates(&shoot.Spec.Kubernetes.KubeControllerManager.KubernetesConfig, shoot.Spec.Kubernetes.Version, "spec.kubernetes.kubeControllerManager.featureGates"); len(reason) > 0 {
			reasons = append(reasons, reason...)
		}
	}

	if shoot.Spec.Kubernetes.KubeScheduler != nil && shoot.Spec.Kubernetes.KubeScheduler.FeatureGates != nil {
		if reason := maintainFeatureGates(&shoot.Spec.Kubernetes.KubeScheduler.KubernetesConfig, shoot.Spec.Kubernetes.Version, "spec.kubernetes.kubeScheduler.featureGates"); len(reason) > 0 {
			reasons = append(reasons, reason...)
		}
	}

	if shoot.Spec.Kubernetes.KubeProxy != nil && shoot.Spec.Kubernetes.KubeProxy.FeatureGates != nil {
		if reason := maintainFeatureGates(&shoot.Spec.Kubernetes.KubeProxy.KubernetesConfig, shoot.Spec.Kubernetes.Version, "spec.kubernetes.kubeProxy.featureGates"); len(reason) > 0 {
			reasons = append(reasons, reason...)
		}
	}

	if shoot.Spec.Kubernetes.Kubelet != nil && shoot.Spec.Kubernetes.Kubelet.FeatureGates != nil {
		if reason := maintainFeatureGates(&shoot.Spec.Kubernetes.Kubelet.KubernetesConfig, shoot.Spec.Kubernetes.Version, "spec.kubernetes.kubelet.featureGates"); len(reason) > 0 {
			reasons = append(reasons, reason...)
		}
	}

	for i := range shoot.Spec.Provider.Workers {
		if shoot.Spec.Provider.Workers[i].Kubernetes != nil && shoot.Spec.Provider.Workers[i].Kubernetes.Kubelet != nil {
			kubeletVersion := ptr.Deref(shoot.Spec.Provider.Workers[i].Kubernetes.Version, shoot.Spec.Kubernetes.Version)

			if reason := maintainFeatureGates(&shoot.Spec.Provider.Workers[i].Kubernetes.Kubelet.KubernetesConfig, kubeletVersion, fmt.Sprintf("spec.provider.workers[%d].kubernetes.kubelet.featureGates", i)); len(reason) > 0 {
				reasons = append(reasons, reason...)
			}
		}
	}

	return reasons
}

// IsFeatureGateSupported is an alias for featuresvalidation.IsFeatureGateSupported. Exposed for testing purposes.
var IsFeatureGateSupported = featuresvalidation.IsFeatureGateSupported

func maintainFeatureGates(kubernetes *gardencorev1beta1.KubernetesConfig, version, fieldPath string) []string {
	var (
		reasons             []string
		validFeatureGates   = make(map[string]bool, len(kubernetes.FeatureGates))
		removedFeatureGates []string
	)

	for fg, enabled := range kubernetes.FeatureGates {
		// err should never be non-nil, because the feature gates are part of the existing spec and are already validated by the GAPI server
		if supported, err := IsFeatureGateSupported(fg, version); err == nil && supported {
			validFeatureGates[fg] = enabled
		} else {
			removedFeatureGates = append(removedFeatureGates, fg)
		}
	}

	kubernetes.FeatureGates = validFeatureGates

	if len(removedFeatureGates) > 0 {
		slices.Sort(removedFeatureGates)
		reasons = append(reasons, fmt.Sprintf("Removed feature gates from %q because they are not supported in Kubernetes version %q: %s", fieldPath, version, strings.Join(removedFeatureGates, ", ")))
	}

	return reasons
}

// IsAdmissionPluginSupported is an alias for admissionpluginsvalidation.IsAdmissionPluginSupported. Exposed for testing purposes.
var IsAdmissionPluginSupported = admissionpluginsvalidation.IsAdmissionPluginSupported

func maintainAdmissionPluginsForShoot(shoot *gardencorev1beta1.Shoot) []string {
	var (
		reasons                 []string
		removedAdmissionPlugins []string
	)

	if shoot.Spec.Kubernetes.KubeAPIServer != nil && shoot.Spec.Kubernetes.KubeAPIServer.AdmissionPlugins != nil {
		validAdmissionPlugins := []gardencorev1beta1.AdmissionPlugin{}
		for _, plugin := range shoot.Spec.Kubernetes.KubeAPIServer.AdmissionPlugins {
			// err should never be non-nil, because the admission plugins are part of the existing spec and are already validated by the GAPI server
			if supported, err := IsAdmissionPluginSupported(plugin.Name, shoot.Spec.Kubernetes.Version); err == nil && supported {
				validAdmissionPlugins = append(validAdmissionPlugins, plugin)
			} else {
				removedAdmissionPlugins = append(removedAdmissionPlugins, plugin.Name)
			}
		}

		shoot.Spec.Kubernetes.KubeAPIServer.AdmissionPlugins = validAdmissionPlugins

		if len(removedAdmissionPlugins) > 0 {
			slices.Sort(removedAdmissionPlugins)
			reasons = append(reasons, fmt.Sprintf("Removed admission plugins from %q because they are not supported in Kubernetes version %q: %s", "spec.kubernetes.kubeAPIServer.admissionPlugins", shoot.Spec.Kubernetes.Version, strings.Join(removedAdmissionPlugins, ", ")))
		}
	}

	return reasons
}

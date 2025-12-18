// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
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
	securityv1alpha1 "github.com/gardener/gardener/pkg/apis/security/v1alpha1"
	controllermanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/controllermanager/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/controllermanager/controller/shoot/maintenance/helper"
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

	credentialsToRotationUpdate := computeCredentialsToRotationResults(log, maintainedShoot, metav1.Time{Time: r.Clock.Now()})

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

	// Set the .spec.kubernetes.kubeControllerManager.podEvictionTimeout field to nil, when Shoot cluster is being forcefully updated to K8s >= 1.33.
	// Gardener forbids setting the field for Shoots with K8s 1.33+. See https://github.com/gardener/gardener/pull/12343
	{
		oldK8sLess133, _ := versionutils.CheckVersionMeetsConstraint(oldShootKubernetesVersion.String(), "< 1.33")
		newK8sGreaterEqual133, _ := versionutils.CheckVersionMeetsConstraint(shootKubernetesVersion.String(), ">= 1.33")
		if oldK8sLess133 && newK8sGreaterEqual133 {
			if maintainedShoot.Spec.Kubernetes.KubeControllerManager != nil && maintainedShoot.Spec.Kubernetes.KubeControllerManager.PodEvictionTimeout != nil {
				maintainedShoot.Spec.Kubernetes.KubeControllerManager.PodEvictionTimeout = nil

				reason := ".spec.kubernetes.kubeControllerManager.podEvictionTimeout is set to nil. Reason: The field was deprecated in favour of `spec.kubernetes.kubeAPIServer.defaultNotReadyTolerationSeconds` and `spec.kubernetes.kubeAPIServer.defaultUnreachableTolerationSeconds` and can no longer be enabled for Shoot clusters using Kubernetes version 1.33+"
				operations = append(operations, reason)
			}
		}
	}

	// Set the .spec.kubernetes.clusterAutoscaler.maxEmptyBulkDelete field to nil, when Shoot cluster is being forcefully updated to K8s >= 1.33.
	// Gardener forbids setting the field for Shoots with K8s 1.33+. See https://github.com/gardener/gardener/pull/12413
	{
		oldK8sLess133, _ := versionutils.CheckVersionMeetsConstraint(oldShootKubernetesVersion.String(), "< 1.33")
		newK8sGreaterEqual133, _ := versionutils.CheckVersionMeetsConstraint(shootKubernetesVersion.String(), ">= 1.33")
		if oldK8sLess133 && newK8sGreaterEqual133 {
			if maintainedShoot.Spec.Kubernetes.ClusterAutoscaler != nil && maintainedShoot.Spec.Kubernetes.ClusterAutoscaler.MaxEmptyBulkDelete != nil {
				maintainedShoot.Spec.Kubernetes.ClusterAutoscaler.MaxEmptyBulkDelete = nil

				reason := ".spec.kubernetes.clusterAutoscaler.maxEmptyBulkDelete is set to nil. Reason: The field was deprecated in favour of `.spec.kubernetes.clusterAutoscaler.maxScaleDownParallelism` and can no longer be enabled for Shoot clusters using Kubernetes version 1.33+"
				operations = append(operations, reason)
			}
		}
	}

	// Migrate from secretBindingName to credentialsBindingName when Shoot cluster is being forcefully updated to K8s >= 1.34.
	// Gardener forbids setting secretBindingName for Shoots with K8s 1.34+.
	{
		oldK8sLess134 := versionutils.ConstraintK8sLess134.Check(oldShootKubernetesVersion)
		newK8sGreaterEqual134 := versionutils.ConstraintK8sGreaterEqual134.Check(shootKubernetesVersion)
		if oldK8sLess134 && newK8sGreaterEqual134 && maintainedShoot.Spec.SecretBindingName != nil && maintainedShoot.Spec.CredentialsBindingName == nil {
			if err := r.migrateSecretBindingToCredentialsBinding(ctx, maintainedShoot); err != nil {
				log.Error(err, "Failed to migrate SecretBinding to CredentialsBinding")
				operations = append(operations, fmt.Sprintf("Failed to migrate from secretBindingName to credentialsBindingName: %v", err))
			} else {
				reason := ".spec.secretBindingName was migrated to .spec.credentialsBindingName. Reason: SecretBinding is deprecated and can no longer be used for Shoot clusters using Kubernetes version 1.34+"
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

	operation := maintainOperation(maintainedShoot, credentialsToRotationUpdate)
	if operation != "" {
		operations = append(operations, fmt.Sprintf("Added %q operation annotation", operation))
	}

	requirePatch := len(operations) > 0 || kubernetesControlPlaneUpdate != nil || len(workerToKubernetesUpdate) > 0 || len(workerToMachineImageUpdate) > 0 || len(credentialsToRotationUpdate) > 0
	if requirePatch {
		patch := client.MergeFrom(shoot.DeepCopy())

		// make sure to include both successful and failed maintenance operations
		description, failureReason := buildMaintenanceMessages(
			kubernetesControlPlaneUpdate,
			workerToKubernetesUpdate,
			workerToMachineImageUpdate,
			credentialsToRotationUpdate,
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
	_ = maintainOperation(shoot, credentialsToRotationUpdate)
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
func buildMaintenanceMessages(kubernetesControlPlaneUpdate *updateResult, workerToKubernetesUpdate, workerToMachineImageUpdate, credentialsToRotationUpdate map[string]updateResult) (string, string) {
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

	for credentials, result := range credentialsToRotationUpdate {
		if result.isSuccessful {
			countSuccessfulOperations++
			description = fmt.Sprintf("%s, %s", description, fmt.Sprintf("Credentials %q: %s. Reason: %s", credentials, result.description, result.reason))
			continue
		}

		countFailedOperations++
		description = fmt.Sprintf("%s, %s", description, fmt.Sprintf("Credentials %q: Automatic rotation failed. Reason for update: %s", credentials, result.reason))
		failureReason = fmt.Sprintf("%s, Credentials %q: Automatic rotation failure due to: %s", failureReason, credentials, result.description)
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

func maintainOperation(shoot *gardencorev1beta1.Shoot, credentialsToRotationUpdate map[string]updateResult) string {
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
		operation = getOperation(shoot, credentialsToRotationUpdate)
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

		machineTypeFromCloudProfile := v1beta1helper.FindMachineTypeByName(cloudProfile.Spec.MachineTypes, worker.Machine.Type)
		if machineTypeFromCloudProfile == nil {
			return nil, fmt.Errorf("machine type %q of worker %q does not exist in cloudprofile", worker.Machine.Type, worker.Name)
		}

		machineImageFromCloudProfile, err := helper.DetermineMachineImage(cloudProfile, workerImage)
		if err != nil {
			return nil, err
		}

		kubeletVersion, err := v1beta1helper.CalculateEffectiveKubernetesVersion(controlPlaneVersion, worker.Kubernetes)
		if err != nil {
			return nil, err
		}

		filteredMachineImageVersionsFromCloudProfile := helper.FilterMachineImageVersions(&machineImageFromCloudProfile, worker, kubeletVersion, machineTypeFromCloudProfile, cloudProfile.Spec.MachineCapabilities)

		// first check if the machine image version should be updated
		shouldBeUpdated, reason, isExpired := shouldMachineImageVersionBeUpdated(workerImage, filteredMachineImageVersionsFromCloudProfile, *shoot.Spec.Maintenance.AutoUpdate.MachineImageVersion)
		if !shouldBeUpdated {
			continue
		}

		updatedMachineImageVersion, err := helper.DetermineMachineImageVersion(workerImage, filteredMachineImageVersionsFromCloudProfile, isExpired)
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

// computeCredentialsToRotationResults starts the credentials rotation if necessary and returns the reason why an update was done
func computeCredentialsToRotationResults(log logr.Logger, shoot *gardencorev1beta1.Shoot, now metav1.Time) map[string]updateResult {
	var (
		maintenanceResults                    = make(map[string]updateResult)
		sshKeypairRotationEnabled             = v1beta1helper.IsSSHKeypairAutoRotationEnabled(shoot)
		observabilityPasswordsRotationEnabled = v1beta1helper.IsObservabilityAutoRotationEnabled(shoot)
		etcdEncryptionKeyRotationEnabled      = v1beta1helper.IsETCDEncryptionKeyAutoRotationEnabled(shoot)
		etcdEncryptionKeyRotationPhase        = v1beta1helper.GetShootETCDEncryptionKeyRotationPhase(shoot.Status.Credentials)
	)

	if sshKeypairRotationEnabled && v1beta1helper.ShootEnablesSSHAccess(shoot) &&
		sshKeypairRotationPassedRotationPeriod(shoot, now.Time, *shoot.Spec.Maintenance.AutoRotation.Credentials.SSHKeypair.RotationPeriod) {
		reason := "Automatic rotation of SSH keypair configured"
		log.Info("SSH keypair for workers will be rotated", "reason", reason)
		maintenanceResults[v1beta1constants.ShootOperationRotateSSHKeypair] = updateResult{
			description:  "SSH keypair rotation started",
			reason:       reason,
			isSuccessful: true,
		}
	}

	if observabilityPasswordsRotationEnabled &&
		observabilityPasswordsRotationPassedRotationPeriod(shoot, now.Time, *shoot.Spec.Maintenance.AutoRotation.Credentials.Observability.RotationPeriod) {
		reason := "Automatic rotation of observability passwords configured"
		log.Info("Observability passwords will be rotated", "reason", reason)
		maintenanceResults[v1beta1constants.OperationRotateObservabilityCredentials] = updateResult{
			description:  "Observability passwords rotation started",
			reason:       reason,
			isSuccessful: true,
		}
	}

	if etcdEncryptionKeyRotationEnabled &&
		etcdEncryptionKeyRotationPassedRotationPeriod(shoot, now.Time, *shoot.Spec.Maintenance.AutoRotation.Credentials.ETCDEncryptionKey.RotationPeriod) {
		if len(etcdEncryptionKeyRotationPhase) == 0 || etcdEncryptionKeyRotationPhase == gardencorev1beta1.RotationCompleted {
			reason := "Automatic rotation of etcd encryption key configured"
			log.Info("ETCD Encryption key will be rotated", "reason", reason)
			maintenanceResults[v1beta1constants.OperationRotateETCDEncryptionKey] = updateResult{
				description:  "ETCD Encryption key rotation started",
				reason:       reason,
				isSuccessful: true,
			}
		} else {
			reason := "ETCD encryption key rotation is already in progress"
			maintenanceResults[v1beta1constants.OperationRotateETCDEncryptionKey] = updateResult{
				description:  "Could not start ETCD encryption key rotation",
				reason:       reason,
				isSuccessful: false,
			}
		}
	}

	return maintenanceResults
}

// sshKeypairRotationPassedRotationPeriod checks if the rotation period for ssh keypair has passed.
func sshKeypairRotationPassedRotationPeriod(shoot *gardencorev1beta1.Shoot, now time.Time, period metav1.Duration) bool {
	// If the shoot has just been created or the credentials have never been rotated, use the shoot's creation timestamp to determine whether the rotation period has passed.
	latestRotationCompletionTime := shoot.CreationTimestamp.Time

	if shoot.Status.Credentials != nil &&
		shoot.Status.Credentials.Rotation != nil &&
		shoot.Status.Credentials.Rotation.SSHKeypair != nil &&
		shoot.Status.Credentials.Rotation.SSHKeypair.LastCompletionTime != nil {
		latestRotationCompletionTime = shoot.Status.Credentials.Rotation.SSHKeypair.LastCompletionTime.Time
	}

	return latestRotationCompletionTime.Before(now.Add(-period.Duration))
}

// observabilityPasswordsRotationPassedRotationPeriod checks if the rotation period for observability passwords has passed.
func observabilityPasswordsRotationPassedRotationPeriod(shoot *gardencorev1beta1.Shoot, now time.Time, period metav1.Duration) bool {
	// If the shoot has just been created or the credentials have never been rotated, use the shoot's creation timestamp to determine whether the rotation period has passed.
	latestRotationCompletionTime := shoot.CreationTimestamp.Time

	if shoot.Status.Credentials != nil &&
		shoot.Status.Credentials.Rotation != nil &&
		shoot.Status.Credentials.Rotation.Observability != nil &&
		shoot.Status.Credentials.Rotation.Observability.LastCompletionTime != nil {
		latestRotationCompletionTime = shoot.Status.Credentials.Rotation.Observability.LastCompletionTime.Time
	}

	return latestRotationCompletionTime.Before(now.Add(-period.Duration))
}

// etcdEncryptionKeyRotationPassedRotationPeriod checks if the rotation period for the etcd encryption key has passed.
func etcdEncryptionKeyRotationPassedRotationPeriod(shoot *gardencorev1beta1.Shoot, now time.Time, period metav1.Duration) bool {
	// If the shoot has just been created or the credentials have never been rotated, use the shoot's creation timestamp to determine whether the rotation period has passed.
	latestRotationCompletionTime := shoot.CreationTimestamp.Time

	if shoot.Status.Credentials != nil &&
		shoot.Status.Credentials.Rotation != nil &&
		shoot.Status.Credentials.Rotation.ETCDEncryptionKey != nil &&
		shoot.Status.Credentials.Rotation.ETCDEncryptionKey.LastCompletionTime != nil {
		latestRotationCompletionTime = shoot.Status.Credentials.Rotation.ETCDEncryptionKey.LastCompletionTime.Time
	}

	return latestRotationCompletionTime.Before(now.Add(-period.Duration))
}

func determineKubernetesVersion(kubernetesVersion string, profile *gardencorev1beta1.CloudProfile, isExpired bool) (string, error) {
	getHigherVersionAutoUpdate := v1beta1helper.GetLatestVersionForPatchAutoUpdate
	getHigherVersionForceUpdate := v1beta1helper.GetVersionForForcefulUpdateToConsecutiveMinor

	version, err := helper.DetermineVersionForStrategy(profile.Spec.Kubernetes.Versions, kubernetesVersion, getHigherVersionAutoUpdate, getHigherVersionForceUpdate, isExpired)
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

	if v1beta1helper.CurrentLifecycleClassification(version) == gardencorev1beta1.ClassificationExpired {
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
	operations := v1beta1helper.GetShootGardenerOperations(shoot.Annotations)
	return slices.Contains(operations, v1beta1constants.ShootOperationMaintain)
}

func needsRetry(shoot *gardencorev1beta1.Shoot) bool {
	needsRetryOperation := false

	if val, ok := shoot.Annotations[v1beta1constants.FailedShootNeedsRetryOperation]; ok {
		needsRetryOperation, _ = strconv.ParseBool(val)
	}

	return needsRetryOperation
}

func getOperation(shoot *gardencorev1beta1.Shoot, credentialsToRotationUpdate map[string]updateResult) string {
	maintenanceOperations := v1beta1helper.GetShootMaintenanceOperations(shoot.Annotations)

	// Always reconcile the Shoot in maintenance cycle.
	if !slices.Contains(maintenanceOperations, v1beta1constants.GardenerOperationReconcile) {
		maintenanceOperations = append(maintenanceOperations, v1beta1constants.GardenerOperationReconcile)
	}

	// Add pending automatic credentials rotations in the current maintenance cycle.
	for credentials, updateResult := range credentialsToRotationUpdate {
		switch {
		case credentials == v1beta1constants.ShootOperationRotateSSHKeypair && updateResult.isSuccessful:
			maintenanceOperations = append(maintenanceOperations, v1beta1constants.ShootOperationRotateSSHKeypair)
		case credentials == v1beta1constants.OperationRotateObservabilityCredentials && updateResult.isSuccessful:
			maintenanceOperations = append(maintenanceOperations, v1beta1constants.OperationRotateObservabilityCredentials)
		case credentials == v1beta1constants.OperationRotateETCDEncryptionKey && updateResult.isSuccessful &&
			!slices.Contains(maintenanceOperations, v1beta1constants.OperationRotateETCDEncryptionKeyStart):
			maintenanceOperations = append(maintenanceOperations, v1beta1constants.OperationRotateETCDEncryptionKey)
		}
	}

	return strings.Join(maintenanceOperations, v1beta1constants.GardenerOperationsSeparator)
}

func shouldMachineImageVersionBeUpdated(shootMachineImage *gardencorev1beta1.ShootMachineImage, machineImage *gardencorev1beta1.MachineImage, autoUpdate bool) (shouldBeUpdated bool, reason string, isExpired bool) {
	versionExistsInCloudProfile, versionIndex := v1beta1helper.ShootMachineImageVersionExists(*machineImage, *shootMachineImage)

	var updateReason string
	if !versionExistsInCloudProfile {
		updateReason = "Version does not exist in CloudProfile"
		return true, updateReason, true
	}

	if v1beta1helper.CurrentLifecycleClassification(machineImage.Versions[versionIndex].ExpirableVersion) == gardencorev1beta1.ClassificationExpired {
		updateReason = fmt.Sprintf("Machine image version expired - force update required (image update strategy: %s)", *machineImage.UpdateStrategy)
		return true, updateReason, true
	}

	if autoUpdate {
		updateReason = fmt.Sprintf("Automatic update of the machine image version is configured (image update strategy: %s)", *machineImage.UpdateStrategy)
		return true, updateReason, false
	}

	return false, "", false
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

// migrateSecretBindingToCredentialsBinding migrates a shoot from using SecretBinding to CredentialsBinding
func (r *Reconciler) migrateSecretBindingToCredentialsBinding(ctx context.Context, shoot *gardencorev1beta1.Shoot) error {
	secretBindingName := *shoot.Spec.SecretBindingName

	secretBinding := &gardencorev1beta1.SecretBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretBindingName,
			Namespace: shoot.Namespace,
		},
	}
	if err := r.Client.Get(ctx, client.ObjectKeyFromObject(secretBinding), secretBinding); err != nil {
		return fmt.Errorf("failed to get SecretBinding %s: %w", client.ObjectKeyFromObject(secretBinding), err)
	}

	// First, check if the migration-created CredentialsBinding exists
	migratedCredentialsBindingName := "force-migrated-" + secretBindingName
	migratedCredentialsBinding := &securityv1alpha1.CredentialsBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      migratedCredentialsBindingName,
			Namespace: shoot.Namespace,
		},
	}

	if err := r.Client.Get(ctx, client.ObjectKeyFromObject(migratedCredentialsBinding), migratedCredentialsBinding); err == nil {
		// Migration-created CredentialsBinding exists, validate it
		if migratedCredentialsBinding.CredentialsRef.Kind != "Secret" ||
			migratedCredentialsBinding.CredentialsRef.APIVersion != "v1" ||
			migratedCredentialsBinding.CredentialsRef.Name != secretBinding.SecretRef.Name ||
			migratedCredentialsBinding.CredentialsRef.Namespace != secretBinding.SecretRef.Namespace {
			return fmt.Errorf("existing CredentialsBinding %s/%s does not reference the same Secret as SecretBinding %s/%s",
				shoot.Namespace, migratedCredentialsBindingName, shoot.Namespace, secretBindingName)
		}

		if !quotasEqual(migratedCredentialsBinding.Quotas, secretBinding.Quotas) {
			return fmt.Errorf("existing CredentialsBinding %s/%s does not have the same Quotas as SecretBinding %s/%s",
				shoot.Namespace, migratedCredentialsBindingName, shoot.Namespace, secretBindingName)
		}

		shoot.Spec.CredentialsBindingName = &migratedCredentialsBindingName
		shoot.Spec.SecretBindingName = nil
		return nil
	} else if !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to check for existing CredentialsBinding %s: %w", client.ObjectKeyFromObject(migratedCredentialsBinding), err)
	}

	// Migration-created CredentialsBinding doesn't exist, search for user-created ones
	credentialsBindingList := &securityv1alpha1.CredentialsBindingList{}
	if err := r.Client.List(ctx, credentialsBindingList, client.InNamespace(shoot.Namespace)); err != nil {
		return fmt.Errorf("failed to list CredentialsBindings in namespace %s: %w", shoot.Namespace, err)
	}

	// Find matching CredentialsBindings that reference the same Secret and have the same Quotas
	var matchingCredentialsBindings []securityv1alpha1.CredentialsBinding
	for _, cb := range credentialsBindingList.Items {
		if cb.CredentialsRef.Kind == "Secret" &&
			cb.CredentialsRef.APIVersion == "v1" &&
			cb.CredentialsRef.Name == secretBinding.SecretRef.Name &&
			cb.CredentialsRef.Namespace == secretBinding.SecretRef.Namespace &&
			quotasEqual(cb.Quotas, secretBinding.Quotas) {
			matchingCredentialsBindings = append(matchingCredentialsBindings, cb)
		}
	}

	if len(matchingCredentialsBindings) > 0 {
		// Sort by name for stable selection (use the first one alphabetically)
		slices.SortFunc(matchingCredentialsBindings, func(a, b securityv1alpha1.CredentialsBinding) int {
			return strings.Compare(a.Name, b.Name)
		})

		selectedCredentialsBinding := matchingCredentialsBindings[0]
		shoot.Spec.CredentialsBindingName = &selectedCredentialsBinding.Name
		shoot.Spec.SecretBindingName = nil
		return nil
	}

	// No existing CredentialsBinding found, create a new migration-created one
	credentialsBinding := &securityv1alpha1.CredentialsBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      migratedCredentialsBindingName,
			Namespace: shoot.Namespace,
			Labels: map[string]string{
				"credentialsbinding.gardener.cloud/status": "force-migrated",
			},
		},
		Provider: securityv1alpha1.CredentialsBindingProvider{
			Type: secretBinding.Provider.Type,
		},
		CredentialsRef: corev1.ObjectReference{
			APIVersion: "v1",
			Kind:       "Secret",
			Name:       secretBinding.SecretRef.Name,
			Namespace:  secretBinding.SecretRef.Namespace,
		},
		Quotas: secretBinding.Quotas,
	}

	if err := r.Client.Create(ctx, credentialsBinding); err != nil {
		return fmt.Errorf("failed to create CredentialsBinding %s: %w", client.ObjectKeyFromObject(credentialsBinding), err)
	}

	shoot.Spec.CredentialsBindingName = &migratedCredentialsBindingName
	shoot.Spec.SecretBindingName = nil

	return nil
}

// quotasEqual compares two quota slices as sets, ignoring order
func quotasEqual(a, b []corev1.ObjectReference) bool {
	if len(a) != len(b) {
		return false
	}

	aMap := make(map[string]corev1.ObjectReference, len(a))
	for _, quota := range a {
		name := quota.Name
		if quota.Namespace != "" {
			name = quota.Namespace + "/" + name
		}
		aMap[name] = quota
	}

	for _, quota := range b {
		name := quota.Name
		if quota.Namespace != "" {
			name = quota.Namespace + "/" + name
		}

		aQuota, exists := aMap[name]
		if !exists {
			return false
		}

		if aQuota.APIVersion != quota.APIVersion ||
			aQuota.Kind != quota.Kind ||
			aQuota.Name != quota.Name ||
			aQuota.Namespace != quota.Namespace {
			return false
		}
	}

	return true
}

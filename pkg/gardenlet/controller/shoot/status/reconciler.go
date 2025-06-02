// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package status

import (
	"context"
	"fmt"
	"slices"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// Reconciler updates the Shoot status with Manual In-Place pending workers from the Worker extension status.
type Reconciler struct {
	GardenClient client.Client
	SeedClient   client.Client
	Config       gardenletconfigv1alpha1.ShootStatusControllerConfiguration
	SeedName     string
}

// Reconcile updates the Shoot status with Manual In-Place pending workers from the Worker extension status.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	ctx, cancel := controllerutils.GetMainReconciliationContext(ctx, controllerutils.DefaultReconciliationTimeout)
	defer cancel()

	shoot := &gardencorev1beta1.Shoot{}
	if err := r.GardenClient.Get(ctx, request.NamespacedName, shoot); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	// if shoot got deleted or is no longer managed by this gardenlet (e.g., due to migration to another seed) then don't requeue
	if shoot.DeletionTimestamp != nil || ptr.Deref(shoot.Spec.SeedName, "") != r.SeedName {
		log.Info("Shoot is being deleted or is no longer managed by this gardenlet, stop reconciling", "shoot", shoot.Name)
		return reconcile.Result{}, nil
	}

	worker := &extensionsv1alpha1.Worker{
		ObjectMeta: metav1.ObjectMeta{
			Name:      shoot.Name,
			Namespace: v1beta1helper.ControlPlaneNamespaceForShoot(shoot),
		},
	}
	if err := r.SeedClient.Get(ctx, client.ObjectKeyFromObject(worker), worker); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Worker is gone, nothing to update", "worker", worker.Name)
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	if worker.Status.InPlaceUpdates == nil || worker.Status.InPlaceUpdates.WorkerPoolToHashMap == nil {
		// This means that the worker is not reconciled yet or doesn't have in-place update workers, nothing to do here
		return reconcile.Result{}, nil
	}

	// If there are no manual in-place update workers in the shoot status, then nothing to do
	if shoot.Status.InPlaceUpdates == nil || shoot.Status.InPlaceUpdates.PendingWorkerUpdates == nil || len(shoot.Status.InPlaceUpdates.PendingWorkerUpdates.ManualInPlaceUpdate) == 0 {
		return reconcile.Result{}, nil
	}

	var (
		manualInPlacePendingWorkers  = sets.New[string]()
		inPlaceUpdatesWorkerPoolHash = worker.Status.InPlaceUpdates.WorkerPoolToHashMap
	)

	for _, pool := range shoot.Spec.Provider.Workers {
		if !v1beta1helper.IsUpdateStrategyManualInPlace(pool.UpdateStrategy) {
			continue
		}

		var (
			kubernetesVersion    = shoot.Spec.Kubernetes.Version
			kubeletConfiguration = shoot.Spec.Kubernetes.Kubelet
		)

		if pool.Kubernetes != nil {
			if pool.Kubernetes.Version != nil {
				kubernetesVersion = *pool.Kubernetes.Version
			}

			if pool.Kubernetes.Kubelet != nil {
				kubeletConfiguration = pool.Kubernetes.Kubelet
			}
		}

		shootWorkerPoolHash, err := gardenerutils.CalculateWorkerPoolHashForInPlaceUpdate(
			pool.Name,
			&kubernetesVersion,
			kubeletConfiguration,
			ptr.Deref(pool.Machine.Image.Version, ""),
			shoot.Status.Credentials,
		)
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to calculate worker pool %q hash: %w", pool.Name, err)
		}

		// If the pool is not at all present in the worker status or the hash is different, then add it to the manual in-place pending workers
		if workerStatusWorkerPoolHash, ok := inPlaceUpdatesWorkerPoolHash[pool.Name]; !ok || workerStatusWorkerPoolHash != shootWorkerPoolHash {
			manualInPlacePendingWorkers.Insert(pool.Name)
		}
	}

	// If both the slices are equal, then nothing to do here
	if sets.New(shoot.Status.InPlaceUpdates.PendingWorkerUpdates.ManualInPlaceUpdate...).Equal(manualInPlacePendingWorkers) {
		log.Info("Manual in-place pending workers are already up-to-date")
		return reconcile.Result{}, nil
	}

	// gardenlet's shoot reconciler might concurrently try to update the status.inPlaceUpdates field.
	// Hence, we need to use optimistic locking to ensure we don't accidentally overwrite concurrent updates.
	patch := client.MergeFromWithOptions(shoot.DeepCopy(), client.MergeFromWithOptimisticLock{})
	shoot.Status.InPlaceUpdates.PendingWorkerUpdates.ManualInPlaceUpdate = slices.DeleteFunc(shoot.Status.InPlaceUpdates.PendingWorkerUpdates.ManualInPlaceUpdate, func(pool string) bool {
		return !manualInPlacePendingWorkers.Has(pool)
	})

	var (
		noManualInPlacePendingWorkers = len(shoot.Status.InPlaceUpdates.PendingWorkerUpdates.ManualInPlaceUpdate) == 0
		noInPlacePendingWorkers       = noManualInPlacePendingWorkers && len(shoot.Status.InPlaceUpdates.PendingWorkerUpdates.AutoInPlaceUpdate) == 0
		shootNeedsReconcile           = false
	)

	if noManualInPlacePendingWorkers {
		shoot.Status.InPlaceUpdates.PendingWorkerUpdates.ManualInPlaceUpdate = nil

		if noInPlacePendingWorkers {
			shoot.Status.InPlaceUpdates = nil
		}
	}

	log.Info("Updating Shoot status with manual in-place pending workers", "manualInPlacePendingWorkers", sets.List(manualInPlacePendingWorkers))
	if err := r.GardenClient.Status().Patch(ctx, shoot, patch); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to patch Shoot status: %w", err)
	}

	// The credentials rotation phases are not set to Prepared in the Shoot reconciliation flow if there are manual in-place pending workers. So we need to trigger a reconciliation after they are all updated.
	if noManualInPlacePendingWorkers {
		shootNeedsReconcile = needsReconcile(shoot)
	}

	if shootNeedsReconcile || (noInPlacePendingWorkers && kubernetesutils.HasMetaDataAnnotation(shoot, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationForceInPlaceUpdate)) {
		patch := client.MergeFromWithOptions(shoot.DeepCopy(), client.MergeFromWithOptimisticLock{})
		if shootNeedsReconcile {
			log.Info("Triggering a Shoot reconciliation after credential rotations for manual in-place workers are prepared")
			metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationReconcile)
		} else {
			delete(shoot.Annotations, v1beta1constants.GardenerOperation)
		}

		if err := r.GardenClient.Patch(ctx, shoot, patch); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to patch Shoot: %w", err)
		}
	}

	return reconcile.Result{}, nil
}

func needsReconcile(shoot *gardencorev1beta1.Shoot) bool {
	caRotationPhase := v1beta1helper.GetShootCARotationPhase(shoot.Status.Credentials)
	serviceAccountKeyRotationPhase := v1beta1helper.GetShootServiceAccountKeyRotationPhase(shoot.Status.Credentials)

	if caRotationPhase == gardencorev1beta1.RotationPreparing || serviceAccountKeyRotationPhase == gardencorev1beta1.RotationPreparing {
		return true
	}

	// If there are no pending workers rollouts for either CA or ServiceAccountKey rotation, then we need to reconcile the Shoot.
	if caRotationPhase == gardencorev1beta1.RotationWaitingForWorkersRollout && len(shoot.Status.Credentials.Rotation.CertificateAuthorities.PendingWorkersRollouts) == 0 {
		return true
	}

	return serviceAccountKeyRotationPhase == gardencorev1beta1.RotationWaitingForWorkersRollout && len(shoot.Status.Credentials.Rotation.ServiceAccountKey.PendingWorkersRollouts) == 0
}

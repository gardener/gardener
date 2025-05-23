// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package seed

import (
	"context"
	"fmt"
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
	"github.com/gardener/gardener/pkg/client/kubernetes"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	seedpkg "github.com/gardener/gardener/pkg/gardenlet/operation/seed"
	"github.com/gardener/gardener/pkg/utils/flow"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	gardenletutils "github.com/gardener/gardener/pkg/utils/gardener/gardenlet"
	"github.com/gardener/gardener/pkg/utils/imagevector"
)

// Reconciler reconciles Seed resources and provisions or de-provisions the seed system components.
type Reconciler struct {
	GardenClient                         client.Client
	SeedClientSet                        kubernetes.Interface
	SeedVersion                          *semver.Version
	Config                               gardenletconfigv1alpha1.GardenletConfiguration
	Clock                                clock.Clock
	Recorder                             record.EventRecorder
	Identity                             *gardencorev1beta1.Gardener
	ComponentImageVectors                imagevector.ComponentImageVectors
	ClientCertificateExpirationTimestamp *metav1.Time
	GardenNamespace                      string
}

// Reconcile reconciles Seed resources and provisions or de-provisions the seed system components.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	seed := &gardencorev1beta1.Seed{}
	if err := r.GardenClient.Get(ctx, request.NamespacedName, seed); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	var seedIsShoot bool
	if err := r.SeedClientSet.Client().Get(ctx, client.ObjectKey{
		Namespace: metav1.NamespaceSystem,
		Name:      v1beta1constants.ConfigMapNameShootInfo,
	}, &corev1.ConfigMap{}); err != nil {
		if !apierrors.IsNotFound(err) {
			return reconcile.Result{}, fmt.Errorf("failed to check if this seed is a shoot: %w", err)
		}
		seedIsShoot = false
	} else {
		seedIsShoot = true
	}

	operationType := gardencorev1beta1.LastOperationTypeReconcile
	if seed.DeletionTimestamp != nil {
		operationType = gardencorev1beta1.LastOperationTypeDelete
	}

	if err := r.updateStatusOperationStart(ctx, seed, operationType); err != nil {
		return reconcile.Result{}, r.updateStatusOperationError(ctx, seed, err, operationType)
	}

	// Check if seed namespace is already available.
	if err := r.GardenClient.Get(ctx, client.ObjectKey{Name: gardenerutils.ComputeGardenNamespace(seed.Name)}, &corev1.Namespace{}); err != nil {
		return reconcile.Result{}, r.updateStatusOperationError(ctx, seed, fmt.Errorf("failed to get seed namespace in garden cluster: %w", err), operationType)
	}

	seedObj, err := seedpkg.NewBuilder().WithSeedObject(seed).Build(ctx)
	if err != nil {
		return reconcile.Result{}, r.updateStatusOperationError(ctx, seed, err, operationType)
	}

	if seed.Status.ClusterIdentity == nil {
		seedClusterIdentity, err := determineClusterIdentity(ctx, r.SeedClientSet.Client())
		if err != nil {
			return reconcile.Result{}, r.updateStatusOperationError(ctx, seed, err, operationType)
		}

		log.Info("Setting cluster identity", "identity", seedClusterIdentity)
		seed.Status.ClusterIdentity = &seedClusterIdentity
		if err := r.GardenClient.Status().Update(ctx, seed); err != nil {
			return reconcile.Result{}, r.updateStatusOperationError(ctx, seed, err, operationType)
		}
	}

	seedIsGarden, err := gardenletutils.SeedIsGarden(ctx, r.SeedClientSet.Client())
	if err != nil {
		return reconcile.Result{}, r.updateStatusOperationError(ctx, seed, err, operationType)
	}

	if seed.DeletionTimestamp != nil {
		result, err := r.delete(ctx, log, seedObj, seedIsGarden, seedIsShoot)
		if err != nil {
			return result, r.updateStatusOperationError(ctx, seed, err, operationType)
		}
		return result, nil
	}

	if err := r.reconcile(ctx, log, seedObj, seedIsGarden, seedIsShoot); err != nil {
		return reconcile.Result{}, r.updateStatusOperationError(ctx, seed, err, operationType)
	}

	return reconcile.Result{RequeueAfter: r.Config.Controllers.Seed.SyncPeriod.Duration}, r.updateStatusOperationSuccess(ctx, seed, operationType)
}

func (r *Reconciler) reportProgress(log logr.Logger, seed *gardencorev1beta1.Seed) flow.ProgressReporter {
	return flow.NewDelayingProgressReporter(clock.RealClock{}, func(ctx context.Context, stats *flow.Stats) {
		patch := client.MergeFrom(seed.DeepCopy())

		if seed.Status.LastOperation == nil {
			seed.Status.LastOperation = &gardencorev1beta1.LastOperation{}
		}
		seed.Status.LastOperation.Description = flow.MakeDescription(stats)
		seed.Status.LastOperation.Progress = stats.ProgressPercent()
		seed.Status.LastOperation.LastUpdateTime = metav1.NewTime(r.Clock.Now().UTC())

		if err := r.GardenClient.Status().Patch(ctx, seed, patch); err != nil {
			log.Error(err, "Could not report reconciliation progress")
		}
	}, 5*time.Second)
}

func (r *Reconciler) updateStatusOperationStart(ctx context.Context, seed *gardencorev1beta1.Seed, operationType gardencorev1beta1.LastOperationType) error {
	var (
		now         = metav1.NewTime(r.Clock.Now().UTC())
		description string
	)

	switch operationType {
	case gardencorev1beta1.LastOperationTypeReconcile:
		description = "Reconciliation of Seed cluster initialized."
	case gardencorev1beta1.LastOperationTypeDelete:
		description = "Deletion of Seed cluster in progress."
	}

	seed.Status.LastOperation = &gardencorev1beta1.LastOperation{
		Type:           operationType,
		State:          gardencorev1beta1.LastOperationStateProcessing,
		Progress:       0,
		Description:    description,
		LastUpdateTime: now,
	}
	seed.Status.Gardener = r.Identity
	seed.Status.ObservedGeneration = seed.Generation
	seed.Status.ClientCertificateExpirationTimestamp = r.ClientCertificateExpirationTimestamp
	seed.Status.KubernetesVersion = ptr.To(r.SeedClientSet.Version())

	// Initialize capacity and allocatable
	var capacity, allocatable corev1.ResourceList
	if r.Config.Resources != nil && len(r.Config.Resources.Capacity) > 0 {
		capacity = make(corev1.ResourceList, len(r.Config.Resources.Capacity))
		allocatable = make(corev1.ResourceList, len(r.Config.Resources.Capacity))

		for resourceName, quantity := range r.Config.Resources.Capacity {
			capacity[resourceName] = quantity
			allocatable[resourceName] = quantity

			if reservedQuantity, ok := r.Config.Resources.Reserved[resourceName]; ok {
				allocatableQuantity := quantity.DeepCopy()
				allocatableQuantity.Sub(reservedQuantity)
				allocatable[resourceName] = allocatableQuantity
			}
		}
	}

	if capacity != nil {
		seed.Status.Capacity = capacity
	}
	if allocatable != nil {
		seed.Status.Allocatable = allocatable
	}

	return r.GardenClient.Status().Update(ctx, seed)
}

func (r *Reconciler) updateStatusOperationSuccess(ctx context.Context, seed *gardencorev1beta1.Seed, operationType gardencorev1beta1.LastOperationType) error {
	var (
		now                        = metav1.NewTime(r.Clock.Now().UTC())
		description                string
		setConditionsToProgressing bool
	)

	switch operationType {
	case gardencorev1beta1.LastOperationTypeReconcile:
		description = "Seed cluster has been successfully reconciled."
		setConditionsToProgressing = true
	case gardencorev1beta1.LastOperationTypeDelete:
		description = "Seed cluster has been successfully deleted."
		setConditionsToProgressing = false
	}

	patch := client.StrategicMergeFrom(seed.DeepCopy())

	if setConditionsToProgressing {
		// Set the status of SeedSystemComponentsHealthy condition to Progressing so that the Seed does not immediately
		// become ready after being successfully reconciled in case the system components got updated. The
		// SeedSystemComponentsHealthy condition will be set to either True, False or Progressing by the seed care
		// reconciler depending on the health of the system components after the necessary checks are completed.
		for i, cond := range seed.Status.Conditions {
			switch cond.Type {
			case gardencorev1beta1.SeedBackupBucketsReady,
				gardencorev1beta1.SeedExtensionsReady,
				gardencorev1beta1.SeedGardenletReady,
				gardencorev1beta1.SeedSystemComponentsHealthy:
				if cond.Status != gardencorev1beta1.ConditionFalse {
					seed.Status.Conditions[i].Status = gardencorev1beta1.ConditionProgressing
					seed.Status.Conditions[i].LastUpdateTime = metav1.Now()
				}
			}
		}
	}

	seed.Status.LastOperation = &gardencorev1beta1.LastOperation{
		Type:           operationType,
		State:          gardencorev1beta1.LastOperationStateSucceeded,
		Progress:       100,
		Description:    description,
		LastUpdateTime: now,
	}

	return r.GardenClient.Status().Patch(ctx, seed, patch)
}

func (r *Reconciler) updateStatusOperationError(ctx context.Context, seed *gardencorev1beta1.Seed, err error, operationType gardencorev1beta1.LastOperationType) error {
	patch := client.StrategicMergeFrom(seed.DeepCopy())

	seed.Status.Gardener = r.Identity
	if seed.Status.LastOperation == nil {
		seed.Status.LastOperation = &gardencorev1beta1.LastOperation{}
	}
	seed.Status.LastOperation.Type = operationType
	seed.Status.LastOperation.State = gardencorev1beta1.LastOperationStateError
	seed.Status.LastOperation.Description = err.Error() + " Operation will be retried."
	seed.Status.LastOperation.LastUpdateTime = metav1.NewTime(r.Clock.Now().UTC())

	if err2 := r.GardenClient.Status().Patch(ctx, seed, patch); err2 != nil {
		return fmt.Errorf("failed updating last operation to state 'Error' (due to %s): %w", err.Error(), err2)
	}

	return err
}

// determineClusterIdentity determines the identity of a cluster, in cases where the identity was
// created manually or the Seed was created as Shoot, and later registered as Seed and already has
// an identity, it should not be changed.
func determineClusterIdentity(ctx context.Context, c client.Client) (string, error) {
	clusterIdentity := &corev1.ConfigMap{}
	if err := c.Get(ctx, client.ObjectKey{Namespace: metav1.NamespaceSystem, Name: v1beta1constants.ClusterIdentity}, clusterIdentity); err != nil {
		if !apierrors.IsNotFound(err) {
			return "", err
		}

		gardenNamespace := &corev1.Namespace{}
		if err := c.Get(ctx, client.ObjectKey{Name: metav1.NamespaceSystem}, gardenNamespace); err != nil {
			return "", err
		}
		return string(gardenNamespace.UID), nil
	}
	return clusterIdentity.Data[v1beta1constants.ClusterIdentity], nil
}

func vpaEnabled(settings *gardencorev1beta1.SeedSettings) bool {
	return settings == nil || settings.VerticalPodAutoscaler == nil || settings.VerticalPodAutoscaler.Enabled
}

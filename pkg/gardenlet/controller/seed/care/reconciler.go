// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package care

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/controllerutils"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// NewHealthCheck is used to create a new Health check instance.
var NewHealthCheck = defaultNewHealthCheck

// Reconciler reconciles Seed resources and executes health check operations.
type Reconciler struct {
	GardenClient client.Client
	SeedClient   client.Client
	Config       gardenletconfigv1alpha1.SeedCareControllerConfiguration
	Clock        clock.Clock
	Namespace    *string
	SeedName     string
}

// Reconcile reconciles Seed resources and executes health check operations.
func (r *Reconciler) Reconcile(reconcileCtx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(reconcileCtx)

	// Timeout for all calls (e.g. status updates), give status updates a bit of headroom if health checks
	// themselves run into timeouts, so that we will still update the status with that timeout error.
	reconcileCtx, cancel := controllerutils.GetMainReconciliationContext(reconcileCtx, r.Config.SyncPeriod.Duration)
	defer cancel()

	seed := &gardencorev1beta1.Seed{}
	if err := r.GardenClient.Get(reconcileCtx, req.NamespacedName, seed); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	ctx, cancel := controllerutils.GetChildReconciliationContext(reconcileCtx, r.Config.SyncPeriod.Duration)
	defer cancel()

	log.V(1).Info("Starting seed care")

	// Initialize conditions based on the current status.
	seedConditions := NewSeedConditions(r.Clock, seed.Status)

	// Trigger health check
	updatedConditions := NewHealthCheck(
		seed,
		r.SeedClient,
		r.Clock,
		r.Namespace,
		r.conditionThresholdsToProgressingMapping(),
	).Check(
		ctx,
		seedConditions,
	)

	// Update Seed status conditions if necessary
	if v1beta1helper.ConditionsNeedUpdate(seedConditions.ConvertToSlice(), updatedConditions) {
		// Rebuild seed conditions to ensure that only the conditions with the
		// correct types will be updated, and any other conditions will remain intact
		conditions := v1beta1helper.BuildConditions(seed.Status.Conditions, updatedConditions, seedConditions.ConditionTypes())

		log.Info("Updating seed status conditions")
		patch := client.StrategicMergeFrom(seed.DeepCopy())
		seed.Status.Conditions = conditions
		if err := r.GardenClient.Status().Patch(ctx, seed, patch); err != nil {
			log.Error(err, "Could not update Seed status")
			return reconcile.Result{}, err
		}
	}

	// Trigger garbage collection
	if err := r.performGarbageCollection(ctx, log); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed performing garbage collection: %w", err)
	}

	return reconcile.Result{RequeueAfter: r.Config.SyncPeriod.Duration}, nil
}

func (r *Reconciler) conditionThresholdsToProgressingMapping() map[gardencorev1beta1.ConditionType]time.Duration {
	out := make(map[gardencorev1beta1.ConditionType]time.Duration)
	for _, threshold := range r.Config.ConditionThresholds {
		out[gardencorev1beta1.ConditionType(threshold.Type)] = threshold.Duration.Duration
	}
	return out
}

func (r *Reconciler) performGarbageCollection(ctx context.Context, log logr.Logger) error {
	podList := &corev1.PodList{}
	if err := r.SeedClient.List(ctx, podList); err != nil {
		return fmt.Errorf("failed listing pods: %w", err)
	}

	for i := len(podList.Items) - 1; i >= 0; i-- {
		if podList.Items[i].Namespace == metav1.NamespaceSystem {
			podList.Items = append(podList.Items[:i], podList.Items[i+1:]...)
		}
	}

	return kubernetesutils.DeleteStalePods(ctx, log, r.SeedClient, podList.Items)
}

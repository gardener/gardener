// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package care

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1beta1helper "github.com/gardener/gardener/pkg/api/core/v1beta1/helper"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/gardenlet/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/utils/flow"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health/checker"
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
	seedConstraints := NewSeedConstraints(r.Clock, seed.Status)

	var updatedConditions, updatedConstraints []gardencorev1beta1.Condition
	_ = flow.Parallel(
		// Trigger health check
		func(ctx context.Context) error {
			updatedConditions = NewHealthCheck(
				seed,
				r.SeedClient,
				r.Clock,
				r.Namespace,
				checker.NewHealthChecker(
					log,
					r.SeedClient,
					r.Clock,
					checker.WithConditionThresholds(r.conditionThresholdsToProgressingMapping())),
			).Check(ctx, seedConditions)
			return nil
		},
		// Trigger constraint check
		func(ctx context.Context) error {
			updatedConstraints = NewConstraintCheck(r.SeedClient, r.Clock, r.Namespace).Check(ctx, seedConstraints)
			return nil
		},
	)(ctx)

	conditionsNeedUpdate := v1beta1helper.ConditionsNeedUpdate(seedConditions.ConvertToSlice(), updatedConditions)
	constraintsNeedUpdate := v1beta1helper.ConditionsNeedUpdate(seedConstraints.ConvertToSlice(), updatedConstraints)

	if conditionsNeedUpdate || constraintsNeedUpdate {
		// Rebuild seed conditions/constraints to ensure that only the entries with the
		// correct types will be updated, and any other entries will remain intact
		patch := client.StrategicMergeFrom(seed.DeepCopy())
		if conditionsNeedUpdate {
			seed.Status.Conditions = v1beta1helper.BuildConditions(seed.Status.Conditions, updatedConditions, seedConditions.ConditionTypes())
		}
		if constraintsNeedUpdate {
			seed.Status.Constraints = v1beta1helper.BuildConditions(seed.Status.Constraints, updatedConstraints, seedConstraints.ConstraintTypes())
		}

		log.Info("Updating seed status conditions and constraints")
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

	for i, v := range slices.Backward(podList.Items) {
		if v.Namespace == metav1.NamespaceSystem {
			podList.Items = append(podList.Items[:i], podList.Items[i+1:]...)
		}
	}

	return kubernetesutils.DeleteStalePods(ctx, log, r.SeedClient, podList.Items)
}

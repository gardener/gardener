// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package care

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/utils/clock"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/controllerutils"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	gardenlethelper "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1/helper"
	"github.com/gardener/gardener/pkg/gardenlet/operation"
	"github.com/gardener/gardener/pkg/utils/flow"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

var (
	// NewOperation is used to create a new `operation.Operation` instance.
	NewOperation = defaultNewOperationFunc
	// NewHealthCheck is used to create a new Health check instance.
	NewHealthCheck = defaultNewHealthCheck
	// NewConstraintCheck is used to create a new Constraint check instance.
	NewConstraintCheck = defaultNewConstraintCheck
	// NewGarbageCollector is used to create a new garbage collection instance.
	NewGarbageCollector = defaultNewGarbageCollector
	// NewWebhookRemediator is used to create a new webhook remediation instance.
	NewWebhookRemediator = defaultNewWebhookRemediator
)

// Reconciler reconciles Shoot resources and executes care operations, e.g. health checks or garbage collection.
type Reconciler struct {
	GardenClient          client.Client
	SeedClientSet         kubernetes.Interface
	ShootClientMap        clientmap.ClientMap
	Config                gardenletconfigv1alpha1.GardenletConfiguration
	Clock                 clock.Clock
	Identity              *gardencorev1beta1.Gardener
	GardenClusterIdentity string
	SeedName              string

	gardenSecrets map[string]*corev1.Secret
}

// Reconcile executes care operations, e.g. health checks or garbage collection.
func (r *Reconciler) Reconcile(ctx context.Context, req Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	// Timeout for all calls (e.g. status updates), give status updates a bit of headroom if health checks
	// themselves run into timeouts, so that we will still update the status with that timeout error.
	ctx, cancel := controllerutils.GetMainReconciliationContext(ctx, r.Config.Controllers.ShootCare.SyncPeriod.Duration)
	defer cancel()

	if !req.IsManagedResource {
		return r.reconcileShoot(ctx, log, req.NamespacedName)
	}

	shootList := &gardencorev1beta1.ShootList{}
	if err := r.GardenClient.List(context.Background(), shootList, client.MatchingFields{
		core.ShootStatusTechnicalID: req.Namespace,
	}); err != nil {
		return reconcile.Result{}, fmt.Errorf("error looking up shoot by technical id: %w", err)
	}

	if len(shootList.Items) == 0 {
		log.V(1).Info("No shoot found for managed resource, ignoring it")
		return reconcile.Result{}, nil
	}

	result, err := r.reconcileShoot(ctx, log, client.ObjectKey{
		Name:      shootList.Items[0].Name,
		Namespace: shootList.Items[0].Namespace,
	})
	if err != nil {
		return result, err
	}
	// Reconciles triggered due to a changed managed resource are one time checks.
	// Periodic checks are already performed based on the existing shoot objects.
	return reconcile.Result{}, nil
}

// Reconcile executes care operations, e.g. health checks or garbage collection.
func (r *Reconciler) reconcileShoot(ctx context.Context, log logr.Logger, key client.ObjectKey) (reconcile.Result, error) {
	shoot := &gardencorev1beta1.Shoot{}
	if err := r.GardenClient.Get(ctx, key, shoot); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	// if shoot has not been picked up by gardenlet yet, requeue
	if shoot.Status.SeedName == nil {
		requeueAfter := 30 * time.Second
		log.V(1).Info("Shoot has not been picked up by gardenlet yet, requeue", "requeueAfter", requeueAfter)
		return reconcile.Result{RequeueAfter: requeueAfter}, nil
	}

	// if shoot is no longer managed by this gardenlet (e.g., due to migration to another seed) then don't requeue.
	if ptr.Deref(shoot.Status.SeedName, "") != r.SeedName {
		return reconcile.Result{}, nil
	}

	careCtx, cancel := controllerutils.GetChildReconciliationContext(ctx, r.Config.Controllers.ShootCare.SyncPeriod.Duration)
	defer cancel()

	// Initialize conditions based on the current status.
	shootConditions := NewShootConditions(r.Clock, shoot)

	// Initialize constraints based on the current status.
	shootConstraints := NewShootConstraints(r.Clock, shoot)

	// Only read Garden secrets once because we don't rely on up-to-date secrets for health checks.
	if r.gardenSecrets == nil {
		secrets, err := gardenerutils.ReadGardenSecrets(careCtx, log, r.GardenClient, gardenerutils.ComputeGardenNamespace(*shoot.Status.SeedName), true)
		if err != nil {
			return reconcile.Result{}, fmt.Errorf("error reading Garden secrets: %w", err)
		}
		r.gardenSecrets = secrets
	}

	o, err := NewOperation(
		careCtx,
		log,
		r.GardenClient,
		r.SeedClientSet,
		r.ShootClientMap,
		&r.Config,
		r.Identity,
		r.GardenClusterIdentity,
		r.gardenSecrets,
		shoot,
	)
	if err != nil {
		updatedConditions, updatedConstraints := r.setStatusToUnknown("Precondition failed: operation could not be initialized", shootConditions.ConvertToSlice(), shootConstraints.ConvertToSlice())
		if err := r.patchStatus(ctx, log, shoot, shootConditions, updatedConditions, shootConstraints, updatedConstraints); err != nil {
			log.Error(err, "Error when trying to update the shoot status after failed operation initialization")
		}
		return reconcile.Result{}, err
	}

	var (
		staleExtensionHealthCheckThreshold    = gardenlethelper.StaleExtensionHealthChecksThreshold(r.Config.Controllers.ShootCare.StaleExtensionHealthChecks)
		initializeShootClients                = shootClientInitializer(careCtx, o)
		updatedConditions, updatedConstraints []gardencorev1beta1.Condition
	)

	if err := flow.Parallel(
		// Trigger health check
		func(ctx context.Context) error {
			updatedConditions = NewHealthCheck(
				log,
				o.Shoot,
				o.Seed,
				r.SeedClientSet,
				r.GardenClient,
				initializeShootClients,
				r.Clock,
				&r.Config,
				r.conditionThresholdsToProgressingMapping(),
			).Check(
				ctx,
				staleExtensionHealthCheckThreshold,
				shootConditions,
			)
			return nil
		},
		// Trigger constraint checks
		func(ctx context.Context) error {
			updatedConstraints = NewConstraintCheck(
				log,
				o.Shoot,
				r.SeedClientSet.Client(),
				initializeShootClients,
				clock.RealClock{},
			).Check(
				ctx,
				shootConstraints,
			)
			return nil
		},
		// Trigger garbage collection
		func(ctx context.Context) error {
			NewGarbageCollector(o, initializeShootClients).Collect(ctx)
			// errors during garbage collection are only being logged and do not cause the care operation to fail
			return nil
		},
		// Trigger webhook remediation
		func(ctx context.Context) error {
			if ptr.Deref(r.Config.Controllers.ShootCare.WebhookRemediatorEnabled, false) {
				_ = NewWebhookRemediator(log, shoot, initializeShootClients).Remediate(ctx)
				// errors during webhook remediation are only being logged and do not cause the care operation to fail
			}
			return nil
		},
	)(careCtx); err != nil {
		return reconcile.Result{}, err
	}

	if err := r.patchStatus(ctx, log, shoot, shootConditions, updatedConditions, shootConstraints, updatedConstraints); err != nil {
		log.Error(err, "Error when trying to update the shoot status")
		return reconcile.Result{}, err
	}

	return reconcile.Result{RequeueAfter: r.Config.Controllers.ShootCare.SyncPeriod.Duration}, nil
}

func (r *Reconciler) conditionThresholdsToProgressingMapping() map[gardencorev1beta1.ConditionType]time.Duration {
	out := make(map[gardencorev1beta1.ConditionType]time.Duration)
	for _, threshold := range r.Config.Controllers.ShootCare.ConditionThresholds {
		out[gardencorev1beta1.ConditionType(threshold.Type)] = threshold.Duration.Duration
	}
	return out
}

func (r *Reconciler) patchStatus(ctx context.Context, log logr.Logger, shoot *gardencorev1beta1.Shoot, existingConditions ShootConditions, updatedConditions []gardencorev1beta1.Condition, existingConstraints ShootConstraints, updatedConstraints []gardencorev1beta1.Condition) error {
	// Update Shoot status (conditions, constraints) only if necessary
	if !v1beta1helper.ConditionsNeedUpdate(existingConditions.ConvertToSlice(), updatedConditions) && !v1beta1helper.ConditionsNeedUpdate(existingConstraints.ConvertToSlice(), updatedConstraints) {
		return nil
	}

	// Rebuild shoot conditions and constraints to ensure that only the conditions and constraints with the
	// correct types will be updated, and any other conditions will remain intact
	mergedConditions := v1beta1helper.BuildConditions(shoot.Status.Conditions, updatedConditions, existingConditions.ConditionTypes())
	mergedConstraints := v1beta1helper.BuildConditions(shoot.Status.Constraints, updatedConstraints, existingConstraints.ConstraintTypes())

	log.V(1).Info("Updating status conditions and constraints")

	patch := client.StrategicMergeFrom(shoot.DeepCopy())
	shoot.Status.Conditions = mergedConditions
	shoot.Status.Constraints = mergedConstraints
	return r.GardenClient.Status().Patch(ctx, shoot, patch)
}

func (r *Reconciler) setStatusToUnknown(message string, conditions []gardencorev1beta1.Condition, constraints []gardencorev1beta1.Condition) ([]gardencorev1beta1.Condition, []gardencorev1beta1.Condition) {
	updatedConditions := make([]gardencorev1beta1.Condition, 0, len(conditions))
	for _, cond := range conditions {
		updatedConditions = append(updatedConditions, v1beta1helper.UpdatedConditionUnknownErrorMessageWithClock(r.Clock, cond, message))
	}

	updatedConstraints := make([]gardencorev1beta1.Condition, 0, len(constraints))
	for _, constr := range constraints {
		updatedConstraints = append(updatedConstraints, v1beta1helper.UpdatedConditionUnknownErrorMessageWithClock(r.Clock, constr, message))
	}

	return updatedConditions, updatedConstraints
}

func shootClientInitializer(ctx context.Context, o *operation.Operation) func() (kubernetes.Interface, bool, error) {
	var (
		once             sync.Once
		apiServerRunning bool
		err              error
	)
	return func() (kubernetes.Interface, bool, error) {
		once.Do(func() {
			// Don't initialize clients for Shoots, for which the API server is not running
			apiServerRunning, err = o.IsAPIServerRunning(ctx)
			if err != nil || !apiServerRunning {
				return
			}

			err = o.InitializeShootClients(ctx)

			// b.InitializeShootClients might not initialize b.ShootClientSet in case the Shoot is being hibernated
			// and the API server has just been scaled down. So, double-check if b.ShootClientSet is set/initialized,
			// otherwise we cannot execute health and constraint checks and garbage collection
			// This is done to prevent a race between the two calls to b.IsAPIServerRunning which would cause the care
			// controller to use a nil shoot client (panic)
			if o.ShootClientSet == nil {
				apiServerRunning = false
			}
		})
		return o.ShootClientSet, apiServerRunning, err
	}
}

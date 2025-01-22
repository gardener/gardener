// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package migration

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
)

// Reconciler reconciles Shoots that should be prepared for the migration and sets a constraint when the destination
// seed is ready.
type Reconciler struct {
	Client client.Client
	Config config.ShootMigrationControllerConfiguration
}

// Reconcile reconciles Shoots that should be prepared for the migration and sets a constraint when the destination
// seed is ready.
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

	patch := client.MergeFrom(shoot.DeepCopy())

	if !v1beta1helper.ShouldPrepareShootForMigration(shoot) || v1beta1helper.ShootHasOperationType(shoot.Status.LastOperation, gardencorev1beta1.LastOperationTypeMigrate) {
		if v1beta1helper.GetCondition(shoot.Status.Constraints, gardencorev1beta1.ShootReadyForMigration) == nil {
			return reconcile.Result{}, nil
		}
		log.Info("Removing constraint since shoot is already migrating")
		shoot.Status.Constraints = v1beta1helper.RemoveConditions(shoot.Status.Constraints, gardencorev1beta1.ShootReadyForMigration)
		return reconcile.Result{}, r.Client.Status().Patch(ctx, shoot, patch)
	}

	sourceSeed := &gardencorev1beta1.Seed{ObjectMeta: metav1.ObjectMeta{Name: *shoot.Status.SeedName}}
	if err := r.Client.Get(ctx, client.ObjectKeyFromObject(sourceSeed), sourceSeed); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed reading source seed %s: %w", sourceSeed.Name, err)
	}

	destinationSeed := &gardencorev1beta1.Seed{ObjectMeta: metav1.ObjectMeta{Name: *shoot.Spec.SeedName}}
	if err := r.Client.Get(ctx, client.ObjectKeyFromObject(destinationSeed), destinationSeed); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed reading destination seed %s: %w", destinationSeed.Name, err)
	}

	if destinationSeed.DeletionTimestamp != nil {
		if needsUpdate := updateConstraint(shoot, gardencorev1beta1.ConditionFalse, "DestinationSeedInDeletion", "Seed is being deleted"); !needsUpdate {
			return reconcile.Result{}, nil
		}
		log.Info("Updating constraint to False since destination seed is being deleted")
		if err := r.Client.Status().Patch(ctx, shoot, patch); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed patching shoot constraints: %w", err)
		}
		return reconcile.Result{}, nil
	}

	if err := health.CheckSeedForMigration(destinationSeed, sourceSeed.Status.Gardener); err != nil {
		if needsUpdate := updateConstraint(shoot, gardencorev1beta1.ConditionFalse, "DestinationSeedUnready", err.Error()); !needsUpdate {
			return reconcile.Result{RequeueAfter: 5 * time.Second}, nil
		}
		log.Info("Updating constraint to False since destination seed is unready", "reason", err.Error())
		if err := r.Client.Status().Patch(ctx, shoot, patch); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed patching shoot constraints: %w", err)
		}
		return reconcile.Result{RequeueAfter: 5 * time.Second}, nil
	}

	if needsUpdate := updateConstraint(shoot, gardencorev1beta1.ConditionTrue, "DestinationSeedReady", "Destination seed cluster is ready for the shoot migration"); !needsUpdate {
		return reconcile.Result{RequeueAfter: 5 * time.Second}, nil
	}
	log.Info("Updating constraint to True since destination seed is ready")
	if err := r.Client.Status().Patch(ctx, shoot, patch); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed patching shoot constraints: %w", err)
	}
	return reconcile.Result{RequeueAfter: 5 * time.Second}, nil
}

func updateConstraint(shoot *gardencorev1beta1.Shoot, status gardencorev1beta1.ConditionStatus, reason, message string) bool {
	c := v1beta1helper.GetCondition(shoot.Status.Constraints, gardencorev1beta1.ShootReadyForMigration)
	if c != nil && c.Status == status {
		return false
	}

	builder, _ := v1beta1helper.NewConditionBuilder(gardencorev1beta1.ShootReadyForMigration)
	if c != nil {
		builder = builder.WithOldCondition(*c)
	}

	newConstraint, updated := builder.WithStatus(status).WithReason(reason).WithMessage(message).Build()
	shoot.Status.Constraints = v1beta1helper.MergeConditions(shoot.Status.Constraints, newConstraint)
	return updated
}

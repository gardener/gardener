// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package lifecycle

import (
	"context"
	"fmt"

	coordinationv1 "k8s.io/api/coordination/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	controllermanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/controllermanager/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/utils/flow"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

// Reconciler reconciles Seeds and checks whether the responsible gardenlet is regularly sending heartbeats. If not, it
// sets the GardenletReady condition of the Seed to Unknown after some grace period passed. If the gardenlet still did
// not send heartbeats and another grace period passed then also all shoot conditions and constraints are set to Unknown.
type Reconciler struct {
	Client         client.Client
	Config         controllermanagerconfigv1alpha1.SeedControllerConfiguration
	Clock          clock.Clock
	LeaseNamespace string
}

// Reconcile reconciles Seeds and checks whether the responsible gardenlet is regularly sending heartbeats. If not, it
// sets the GardenletReady condition of the Seed to Unknown after some grace period passed. If the gardenlet still did
// not send heartbeats and another grace period passed then also all shoot conditions and constraints are set to Unknown.
func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	ctx, cancel := controllerutils.GetMainReconciliationContext(ctx, r.Config.SyncPeriod.Duration)
	defer cancel()

	seed := &gardencorev1beta1.Seed{}
	if err := r.Client.Get(ctx, req.NamespacedName, seed); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	// New seeds don't have conditions - gardenlet never reported anything yet. Wait for grace period.
	if len(seed.Status.Conditions) == 0 {
		return reconcile.Result{RequeueAfter: r.Config.SyncPeriod.Duration}, nil
	}

	lease := &coordinationv1.Lease{}
	if err := r.Client.Get(ctx, client.ObjectKey{Namespace: r.LeaseNamespace, Name: seed.Name}, lease); client.IgnoreNotFound(err) != nil {
		return reconcile.Result{}, err
	}

	if lease.Spec.RenewTime != nil {
		if lease.Spec.RenewTime.UTC().Add(r.Config.MonitorPeriod.Duration).After(r.Clock.Now().UTC()) {
			return reconcile.Result{RequeueAfter: r.Config.SyncPeriod.Duration}, nil
		}

		log.Info("Lease was not renewed in time",
			"renewTime", lease.Spec.RenewTime.UTC(),
			"now", r.Clock.Now().UTC(),
			"seedMonitorPeriod", r.Config.MonitorPeriod.Duration,
		)
	}

	log.Info("Setting Seed status to 'Unknown' as gardenlet stopped reporting seed status")

	bldr, err := v1beta1helper.NewConditionBuilder(gardencorev1beta1.SeedGardenletReady)
	if err != nil {
		return reconcile.Result{}, err
	}

	conditionGardenletReady := v1beta1helper.GetCondition(seed.Status.Conditions, gardencorev1beta1.SeedGardenletReady)
	if conditionGardenletReady != nil {
		bldr.WithOldCondition(*conditionGardenletReady)
	}

	bldr.WithStatus(gardencorev1beta1.ConditionUnknown)
	bldr.WithReason("SeedStatusUnknown")
	bldr.WithMessage("Gardenlet stopped posting seed status.")
	if newCondition, update := bldr.WithClock(r.Clock).Build(); update {
		seed.Status.Conditions = v1beta1helper.MergeConditions(seed.Status.Conditions, newCondition)
		if err := r.Client.Status().Update(ctx, seed); err != nil {
			return reconcile.Result{}, err
		}
		conditionGardenletReady = &newCondition
	}

	// If the gardenlet's client certificate is expired and the seed belongs to a `ManagedSeed` then we reconcile it in
	// order to re-bootstrap the gardenlet.
	if seed.Status.ClientCertificateExpirationTimestamp != nil && seed.Status.ClientCertificateExpirationTimestamp.UTC().Before(r.Clock.Now().UTC()) {
		managedSeed, err := kubernetesutils.GetManagedSeedByName(ctx, r.Client, seed.Name)
		if err != nil {
			return reconcile.Result{}, err
		}

		if managedSeed != nil {
			log.Info("Triggering ManagedSeed reconciliation since gardenlet client certificate is expired", "managedSeed", client.ObjectKeyFromObject(managedSeed))

			patch := client.MergeFrom(managedSeed.DeepCopy())
			metav1.SetMetaDataAnnotation(&managedSeed.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationReconcile)
			if err := r.Client.Patch(ctx, managedSeed, patch); err != nil {
				return reconcile.Result{}, err
			}
		}
	}

	// If the `GardenletReady` condition is `Unknown` for at least the configured `shootMonitorPeriod` then we will mark the conditions
	// and constraints for all the shoots that belong to this seed as `Unknown`. The reason is that the gardenlet didn't send a heartbeat
	// anymore, hence, it most likely didn't check the shoot status. This means that the current shoot status might not reflect the truth
	// anymore. We are indicating this by marking it as `Unknown`.
	if conditionGardenletReady != nil && conditionGardenletReady.LastTransitionTime.UTC().Add(r.Config.ShootMonitorPeriod.Duration).After(r.Clock.Now().UTC()) {
		return reconcile.Result{RequeueAfter: r.Config.SyncPeriod.Duration}, nil
	}

	var gardenletOfflineSince any = "Unknown"
	if conditionGardenletReady != nil {
		gardenletOfflineSince = conditionGardenletReady.LastTransitionTime.UTC()
	}

	log.Info("Gardenlet has not sent heartbeats for at least the configured shoot monitor period, setting shoot conditions and constraints to 'Unknown' for all shoots on this seed",
		"gardenletOfflineSince", gardenletOfflineSince,
		"now", r.Clock.Now().UTC(),
		"shootMonitorPeriod", r.Config.ShootMonitorPeriod.Duration,
	)

	shootList := &gardencorev1beta1.ShootList{}
	if err := r.Client.List(ctx, shootList, client.MatchingFields{core.ShootStatusSeedName: seed.Name}); err != nil {
		return reconcile.Result{}, err
	}

	var fns []flow.TaskFn

	for _, s := range shootList.Items {
		shoot := s
		fns = append(fns, func(ctx context.Context) error {
			return setShootStatusToUnknown(ctx, r.Clock, r.Client, &shoot)
		})
	}

	if err := flow.Parallel(fns...)(ctx); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{RequeueAfter: r.Config.SyncPeriod.Duration}, nil
}

func setShootStatusToUnknown(ctx context.Context, clock clock.Clock, c client.StatusClient, shoot *gardencorev1beta1.Shoot) error {
	var (
		reason = "StatusUnknown"
		msg    = "Gardenlet stopped sending heartbeats."

		conditions  = make(map[gardencorev1beta1.ConditionType]gardencorev1beta1.Condition)
		constraints = map[gardencorev1beta1.ConditionType]gardencorev1beta1.Condition{
			gardencorev1beta1.ShootHibernationPossible:               {},
			gardencorev1beta1.ShootMaintenancePreconditionsSatisfied: {},
		}
	)

	for _, conditionType := range gardenerutils.GetShootConditionTypes(v1beta1helper.IsWorkerless(shoot)) {
		c := v1beta1helper.GetOrInitConditionWithClock(clock, shoot.Status.Conditions, conditionType)
		c = v1beta1helper.UpdatedConditionWithClock(clock, c, gardencorev1beta1.ConditionUnknown, reason, msg)
		conditions[conditionType] = c
	}

	for conditionType := range constraints {
		c := v1beta1helper.GetOrInitConditionWithClock(clock, shoot.Status.Constraints, conditionType)
		c = v1beta1helper.UpdatedConditionWithClock(clock, c, gardencorev1beta1.ConditionUnknown, reason, msg)
		constraints[conditionType] = c
	}

	patch := client.StrategicMergeFrom(shoot.DeepCopy())
	shoot.Status.Conditions = v1beta1helper.MergeConditions(shoot.Status.Conditions, conditionMapToConditions(conditions)...)
	shoot.Status.Constraints = v1beta1helper.MergeConditions(shoot.Status.Constraints, conditionMapToConditions(constraints)...)
	return c.Status().Patch(ctx, shoot, patch)
}

func conditionMapToConditions(m map[gardencorev1beta1.ConditionType]gardencorev1beta1.Condition) []gardencorev1beta1.Condition {
	output := make([]gardencorev1beta1.Condition, 0, len(m))

	for _, condition := range m {
		output = append(output, condition)
	}

	return output
}

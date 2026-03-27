// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package backupbucketscheck

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1beta1helper "github.com/gardener/gardener/pkg/api/core/v1beta1/helper"
	controllermanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/controllermanager/v1alpha1"
	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/controllermanager/controller/seed/utils"
)

// Reconciler reconciles Seeds and maintains the BackupBucketsReady condition according to the observed status of the
// referencing BackupBuckets.
type Reconciler struct {
	Client client.Client
	Config controllermanagerconfigv1alpha1.SeedBackupBucketsCheckControllerConfiguration
	Clock  clock.Clock
}

// Reconcile reconciles Seeds and maintains the BackupBucketsReady condition according to the observed status of the
// referencing BackupBuckets.
func (r *Reconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	seed := &gardencorev1beta1.Seed{}
	if err := r.Client.Get(ctx, req.NamespacedName, seed); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	backupBucketList := &gardencorev1beta1.BackupBucketList{}
	if err := r.Client.List(ctx, backupBucketList, client.MatchingFields{core.BackupBucketSeedName: seed.Name}); err != nil {
		return reconcile.Result{}, err
	}

	conditionBackupBucketsReady := v1beta1helper.GetOrInitConditionWithClock(r.Clock, seed.Status.Conditions, gardencorev1beta1.SeedBackupBucketsReady)
	conditionThreshold := utils.GetThresholdForCondition(r.Config.ConditionThresholds, gardencorev1beta1.SeedBackupBucketsReady)

	newCondition := v1beta1helper.ComputeBackupBucketsCondition(r.Clock, conditionBackupBucketsReady, backupBucketList.Items)
	switch newCondition.Status {
	case gardencorev1beta1.ConditionFalse:
		conditionBackupBucketsReady = utils.SetToProgressingOrFalse(r.Clock, conditionThreshold, conditionBackupBucketsReady, newCondition.Reason, newCondition.Message)
	case gardencorev1beta1.ConditionUnknown:
		conditionBackupBucketsReady = utils.SetToProgressingOrUnknown(r.Clock, conditionThreshold, conditionBackupBucketsReady, newCondition.Reason, newCondition.Message)
	default:
		conditionBackupBucketsReady = newCondition
	}

	if updateErr := utils.PatchSeedCondition(ctx, log, r.Client.Status(), seed, conditionBackupBucketsReady); updateErr != nil {
		return reconcile.Result{}, updateErr
	}

	return reconcile.Result{RequeueAfter: r.Config.SyncPeriod.Duration}, nil
}

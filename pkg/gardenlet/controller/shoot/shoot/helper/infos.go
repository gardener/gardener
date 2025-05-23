// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"time"

	"k8s.io/utils/clock"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/timewindow"
)

// This file is responsible for implementing some optimizations described in
// https://github.com/gardener/gardener/blob/master/docs/concepts/gardenlet.md#shoot-controller
// The shoot main controller calls CalculateControllerInfos for determining whether and when it should reconcile a given
// shoot based on its state and the gardenlet's configuration.
// This logic is super verbose for readability and documentation purposes.
// DO NOT TRY TO REFACTOR OR SIMPLIFY THIS!
// Regressions of this logic have high impact on large-scale gardener installations.
// We faced bugs and regressions of this logic in the past because it was short, unreadable, and undocumented.
// Hence, don't try to make it more "elegant"!

// ControllerInfos is the result of CalculateControllerInfos and informs the controller about whether and when a given
// shoot should be reconciled.
// These results are supposed to be used/handled immediately. Always call CalculateControllerInfos to re-calculate the
// results before using them.
// The exported fields are the exposed results, all other fields are for internal computations only.
type ControllerInfos struct {
	// OperationType is the operation that should be executed when acting on the given shoot.
	// Note: this can contain an operation type that is different to the operation that was just performed.
	// E.g., when re-calculating the infos after a successful Create operation, this field will be set to Reconcile, as
	// the next operation that should be performed on the shoot is a usual Reconcile operation.
	OperationType gardencorev1beta1.LastOperationType
	// ShouldReconcileNow is true if the controller should perform reconciliations immediately.
	// This is supposed to be used in the reconciler only.
	ShouldReconcileNow bool
	// ShouldOnlySyncClusterResource is true if the controller should not perform reconciliations right now
	// (ShouldReconcileNow == false) but the controller should still sync the cluster resource to the seed.
	ShouldOnlySyncClusterResource bool
	// EnqueueAfter is the duration after which the shoot should be reconciled after observing an ADD watch event
	// (on shoot creation and on initial listing of all shoots on gardenlet startup).
	// This is supposed to be used in the add event handler only.
	EnqueueAfter time.Duration
	// RequeueAfter is the reconciliation result that should be returned by the reconciler for scheduling the next
	// regular sync in the following two situations only:
	// - after finishing a shoot Create/Reconcile/Restore operation successfully
	// - when handling a reconciliation request for a shoot that should not be reconciled right now (ShouldReconcileNow == false)
	RequeueAfter reconcile.Result

	// internal fields, these are not supposed to be exposed or returned to the controller

	clock clock.Clock

	// based on gardenlet config
	reconcileInMaintenanceOnly bool
	respectSyncPeriodOverwrite bool

	// based on shoot object
	isIgnored                             bool
	isFailed                              bool
	isUpToDate                            bool
	confineSpecUpdateRollout              bool
	maintenanceTimeWindow                 *timewindow.MaintenanceTimeWindow
	isNowInEffectiveMaintenanceTimeWindow bool
	alreadyReconciledDuringThisTimeWindow bool

	// based on gardenlet config and shoot object
	syncPeriod time.Duration
}

// CalculateControllerInfos calculates whether and when a given shoot should be reconciled.
// These results are supposed to be used/handled immediately. Always call CalculateControllerInfos to re-calculate the
// results before using them.
func CalculateControllerInfos(shoot *gardencorev1beta1.Shoot, clock clock.Clock, cfg gardenletconfigv1alpha1.ShootControllerConfiguration) ControllerInfos {
	respectSyncPeriodOverwrite := ptr.Deref(cfg.RespectSyncPeriodOverwrite, false)

	i := ControllerInfos{
		OperationType: ComputeOperationType(shoot),
		clock:         clock,

		reconcileInMaintenanceOnly: ptr.Deref(cfg.ReconcileInMaintenanceOnly, false),
		respectSyncPeriodOverwrite: respectSyncPeriodOverwrite,

		isIgnored:                             gardenerutils.ShouldIgnoreShoot(respectSyncPeriodOverwrite, shoot),
		isFailed:                              gardenerutils.IsShootFailedAndUpToDate(shoot),
		isUpToDate:                            gardenerutils.IsObservedAtLatestGenerationAndSucceeded(shoot),
		confineSpecUpdateRollout:              v1beta1helper.ShootConfinesSpecUpdateRollout(shoot.Spec.Maintenance),
		maintenanceTimeWindow:                 gardenerutils.EffectiveShootMaintenanceTimeWindow(shoot),
		isNowInEffectiveMaintenanceTimeWindow: gardenerutils.IsNowInEffectiveShootMaintenanceTimeWindow(shoot, clock),
		alreadyReconciledDuringThisTimeWindow: gardenerutils.LastReconciliationDuringThisTimeWindow(shoot, clock),

		syncPeriod: gardenerutils.SyncPeriodOfShoot(respectSyncPeriodOverwrite, cfg.SyncPeriod.Duration, shoot),
	}

	i.ShouldReconcileNow = i.shouldReconcileNow()
	i.ShouldOnlySyncClusterResource = i.shouldOnlySyncClusterResource()
	i.EnqueueAfter = i.enqueueAfter()
	i.RequeueAfter = i.requeueAfter()

	return i
}

func (i ControllerInfos) shouldReconcileNow() bool {
	// if the shoot is failed or ignored, it doesn't matter which operation is triggered
	if i.isFailed || i.isIgnored {
		return false
	}

	// in the following checks, we optimize when and how often existing shoots are reconciled
	// all operations other than Reconcile are excluded from these optimizations as we want to perform the operations immediately
	if i.OperationType != gardencorev1beta1.LastOperationTypeReconcile {
		return true
	}

	// if the observedGeneration is outdated (i.e., spec was changed) or the last operation was not successful,
	// we always want to reconcile immediately
	if !i.isUpToDate {
		return true
	}

	// from here on, we have a case of regular reconciliation (i.e., shoot is observed at latest generation and in
	// 'Reconcile Succeeded' state)

	// If we do not confine reconciliations (either by the operator or shoot owner) to the maintenance time window then
	// we allow immediate reconciliations.
	if !i.reconcileInMaintenanceOnly && !i.confineSpecUpdateRollout {
		return true
	}

	// from here on, either:
	// a) the operator configured gardenlet to confine regular reconciliations to the maintenance time window
	// b) the shoot owner confined spec update rollouts to the maintenance time window

	// if the shoot is in its maintenance time window right now but has not been reconciled in the time window so far,
	// now is the time that we have been waiting for :) -> go
	if i.isNowInEffectiveMaintenanceTimeWindow && !i.alreadyReconciledDuringThisTimeWindow {
		return true
	}

	// shoot reconciliations are confined, and the shoot's maintenance time window is not met, or it was already
	// reconciled in this time window -> ¯\_(ツ)_/¯
	return false
}

func (i ControllerInfos) shouldOnlySyncClusterResource() bool {
	return i.isFailed || i.isIgnored
}

func (i ControllerInfos) enqueueAfter() time.Duration {
	// if the shoot is failed or ignored, we need to enqueue the shoot now to sync the cluster resource to the seed
	if i.isFailed || i.isIgnored {
		return 0
	}

	// in the following checks, we optimize when existing shoots are reconciled after gardenlet startup to prevent
	// overload situations caused by immediately reconciling all existing shoots
	// all operations other than Reconcile are excluded from these optimizations as we want to perform the operations immediately
	if i.OperationType != gardencorev1beta1.LastOperationTypeReconcile {
		return 0
	}

	// if the observedGeneration is outdated (i.e., spec was changed) or the last operation was not successful,
	// we always want to reconcile immediately
	if !i.isUpToDate {
		return 0
	}

	// from here on, shoots need to be enqueued for the next regular reconciliation only, which doesn't need to happen
	// immediately (i.e., shoot is observed at latest generation and in 'Reconcile Succeeded' state)

	// If we do not confine reconciliations (either by the operator or shoot owner) to the maintenance time window then
	// we allow immediate reconciliations.
	if !i.reconcileInMaintenanceOnly && !i.confineSpecUpdateRollout {
		return 0
	}

	// from here on, either:
	// a) the operator configured gardenlet to confine regular reconciliations to the maintenance time window
	// b) the shoot owner confined spec update rollouts to the maintenance time window

	// if the shoot is in its maintenance time window right now but has not been reconciled in the time window so far,
	// schedule the regular reconciliation for a random time in this time window
	if i.isNowInEffectiveMaintenanceTimeWindow && !i.alreadyReconciledDuringThisTimeWindow {
		return i.maintenanceTimeWindow.RandomDurationUntilNext(i.clock.Now(), true)
	}

	// shoot reconciliations are confined, and the shoot's maintenance time window is not met, or it was already
	// reconciled in this time window -> schedule a reconciliation during the next maintenance time window
	return i.maintenanceTimeWindow.RandomDurationUntilNext(i.clock.Now(), false)
}

func (i ControllerInfos) requeueAfter() reconcile.Result {
	// if the shoot is failed or ignored, we don't want to requeue the shoot
	if i.isFailed || i.isIgnored {
		return reconcile.Result{}
	}

	// from here on, we have a case of regular reconciliation (i.e., shoot is observed at latest generation and in
	// 'Reconcile Succeeded' state).

	// If we do not confine reconciliations (either by the operator or shoot owner) to the maintenance time window then
	// we requeue according to the sync period configured in gardenlet configuration (which might have been overwritten on
	// the shoot via annotation).
	if !i.reconcileInMaintenanceOnly && !i.confineSpecUpdateRollout {
		return reconcile.Result{RequeueAfter: i.syncPeriod}
	}

	// from here on, either:
	// a) the operator configured gardenlet to confine regular reconciliations to the maintenance time window
	// b) the shoot owner confined spec update rollouts to the maintenance time window

	// shoot reconciliations are confined, and the shoot's maintenance time window is not met, or it was already
	// reconciled in this time window -> schedule a reconciliation during the next maintenance time window
	return reconcile.Result{RequeueAfter: i.maintenanceTimeWindow.RandomDurationUntilNext(i.clock.Now(), false)}
}

// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package hibernation

import (
	"context"
	"fmt"
	"slices"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/events"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	controllermanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/controllermanager/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	hibernationutils "github.com/gardener/gardener/pkg/utils/hibernation"
)

const (
	sevenDays         = 7 * 24 * time.Hour
	nextScheduleDelta = 100 * time.Millisecond
)

// Reconciler reconciles Shoots and hibernates or wakes them up according to their hibernation schedules.
type Reconciler struct {
	Client   client.Client
	Config   controllermanagerconfigv1alpha1.ShootHibernationControllerConfiguration
	Clock    clock.Clock
	Recorder events.EventRecorder
}

// Reconcile reconciles Shoots and hibernates or wakes them up according to their hibernation schedules.
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
		log.Info("Shoot is currently being deleted, stopping reconciliation")
		return reconcile.Result{}, nil
	}

	schedules := getShootHibernationSchedules(shoot.Spec.Hibernation)
	if len(schedules) == 0 {
		log.Info("Hibernation schedules have been removed from shoot, stopping reconciliation")
		return reconcile.Result{}, nil
	}

	parsedSchedules, err := hibernationutils.Parse(schedules)
	if err != nil {
		log.Error(err, "Invalid hibernation schedules, stopping reconciliation")
		return reconcile.Result{}, nil
	}

	now := r.Clock.Now()
	if gardenerutils.IsShootFailedAndUpToDate(shoot) {
		requeueAfter := nextHibernationTimeDuration(parsedSchedules, now)
		log.Info("Shoot is in Failed state, requeuing shoot hibernation", "requeueAfter", requeueAfter)
		return reconcile.Result{RequeueAfter: requeueAfter}, nil
	}

	// Get the schedule which caused the current reconciliation and check whether the shoot should be hibernated or woken up.
	// If no such schedule is found, the hibernation schedules were changed mid-air and the shoot must be
	// hibernated or wakeup the at a later time.
	mostRecentSchedule := getScheduleWithMostRecentTime(parsedSchedules, r.Config.TriggerDeadlineDuration, shoot, now)
	if mostRecentSchedule != nil {
		if err := r.hibernateOrWakeUpShootBasedOnSchedule(ctx, shoot, mostRecentSchedule, now); err != nil {
			return reconcile.Result{}, err
		}
		log.Info("Successfully set hibernation.enabled", "enabled", *shoot.Spec.Hibernation.Enabled)
	}

	requeueAfter := nextHibernationTimeDuration(parsedSchedules, now)
	log.Info("Requeuing shoot hibernation", "requeueAfter", requeueAfter)
	return reconcile.Result{RequeueAfter: requeueAfter}, nil
}

func (r *Reconciler) hibernateOrWakeUpShootBasedOnSchedule(ctx context.Context, shoot *gardencorev1beta1.Shoot, schedule *hibernationutils.ParsedSchedule, now time.Time) error {
	patch := client.MergeFrom(shoot.DeepCopy())
	switch schedule.Operation {
	case hibernationutils.Hibernate:
		shoot.Spec.Hibernation.Enabled = new(true)
		r.Recorder.Eventf(shoot, nil, corev1.EventTypeNormal, gardencorev1beta1.ShootEventHibernationEnabled, gardencorev1beta1.EventActionReconcile, "Hibernating cluster due to schedule")
	case hibernationutils.WakeUp:
		shoot.Spec.Hibernation.Enabled = new(false)
		r.Recorder.Eventf(shoot, nil, corev1.EventTypeNormal, gardencorev1beta1.ShootEventHibernationDisabled, gardencorev1beta1.EventActionReconcile, "Waking up cluster due to schedule")
	}
	if err := r.Client.Patch(ctx, shoot, patch); err != nil {
		return err
	}

	patch = client.MergeFrom(shoot.DeepCopy())
	shoot.Status.LastHibernationTriggerTime = &metav1.Time{Time: now}
	return r.Client.Status().Patch(ctx, shoot, patch)
}

// nextHibernationTimeDuration returns the time duration after which to requeue the shoot based on the hibernation schedules and current time.
// It adds a 100ms padding to the next requeue to account for Network Time Protocol(NTP) time skews.
// If the time drifts are adjusted which in most realistic cases would be around 100ms, scheduled hibernation
// will still be executed without missing the schedule.
func nextHibernationTimeDuration(schedules []hibernationutils.ParsedSchedule, now time.Time) time.Duration {
	timeStamps := make([]time.Time, 0, len(schedules))
	for _, schedule := range schedules {
		timeStamps = append(timeStamps, schedule.Next(now))
	}

	slices.SortFunc(timeStamps, func(a, b time.Time) int {
		return a.Compare(b)
	})

	return timeStamps[0].Add(nextScheduleDelta).Sub(now)
}

// getScheduleWithMostRecentTime returns the ParsedSchedule that contains the schedule with the most recent (previous) execution time.
func getScheduleWithMostRecentTime(schedules []hibernationutils.ParsedSchedule, triggerDeadlineDuration *metav1.Duration, shoot *gardencorev1beta1.Shoot, now time.Time) *hibernationutils.ParsedSchedule {
	// If the shoot has just been created or has never been hibernated, use the creation timestamp.
	earliestTime := shoot.CreationTimestamp.Time
	if shoot.Status.LastHibernationTriggerTime != nil {
		earliestTime = shoot.Status.LastHibernationTriggerTime.Time
	}

	if triggerDeadlineDuration != nil {
		if triggerDeadline := now.Add(-triggerDeadlineDuration.Duration); triggerDeadline.After(earliestTime) {
			earliestTime = triggerDeadline
		}
	}

	// Cap earliestTime to 7 days ago. This is necessary if the shoot was created a long time ago and has never been hibernated,
	// so that a smaller time frame is used when looking for the schedule that has the most recent time entry.
	if sevenDaysAgo := now.Add(-sevenDays); earliestTime.Before(sevenDaysAgo) {
		earliestTime = sevenDaysAgo
	}

	// Iterate over all schedules that were parsed from the shoot specification until we find one that contains
	// a time entry between `earliestTime` and `now`` and that time entry is the latest one (most recent) with respect to `now`
	var scheduleWithMostRecentTime *hibernationutils.ParsedSchedule
	for i := range schedules {
		cur := schedules[i].Previous(earliestTime, now)
		if cur == nil {
			continue
		}
		if scheduleWithMostRecentTime == nil {
			scheduleWithMostRecentTime = &schedules[i]
			continue
		}
		mostRecentTime := scheduleWithMostRecentTime.Previous(earliestTime, now)
		if mostRecentTime == nil {
			continue
		}
		if cur.After(*mostRecentTime) {
			scheduleWithMostRecentTime = &schedules[i]
		}
	}

	return scheduleWithMostRecentTime
}

func getShootHibernationSchedules(hibernation *gardencorev1beta1.Hibernation) []gardencorev1beta1.HibernationSchedule {
	if hibernation == nil {
		return nil
	}
	return hibernation.Schedules
}

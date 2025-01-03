// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package hibernation

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/robfig/cron"
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
	controllermanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/controllermanager/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

const (
	sevenDays         = 7 * 24 * time.Hour
	nextScheduleDelta = 100 * time.Millisecond
)

type operation uint8

const (
	hibernate operation = iota
	wakeUp
)

// parsedHibernationSchedule holds the loaded location, parsed cron schedule and information whether
// the cluster should be hibernated or woken up.
type parsedHibernationSchedule struct {
	location  time.Location
	schedule  cron.Schedule
	operation operation
}

// next returns the time in UTC from the schedule, that is immediately after the input time 't'.
// The input 't' is converted in the schedule's location before any calculations are done.
func (s *parsedHibernationSchedule) next(t time.Time) time.Time {
	return s.schedule.Next(t.In(&s.location)).UTC()
}

// previous returns the time in UTC from the schedule that is immediately before 'to' and after 'from'.
// Nil is returned if no such time can be found.
// The input times - 'to' and 'from' are converted in the schedule's location before any calculation is done.
func (s *parsedHibernationSchedule) previous(from, to time.Time) *time.Time {
	// To get the time that is immediately before `to`, iterate over every activation time in the cron schedule
	// that is after "from" until the one that is immediately after `to` is reached.
	var previousActivationTime *time.Time
	for t := s.schedule.Next(from.In(&s.location)); !t.UTC().After(to.UTC()); t = s.schedule.Next(t) {
		inUTC := t.UTC()
		previousActivationTime = &inUTC
	}

	return previousActivationTime
}

// Reconciler reconciles Shoots and hibernates or wakes them up according to their hibernation schedules.
type Reconciler struct {
	Client   client.Client
	Config   controllermanagerconfigv1alpha1.ShootHibernationControllerConfiguration
	Clock    clock.Clock
	Recorder record.EventRecorder
}

// Reconcile reconciles Shoots and hibernates or wakes them up according to their hibernation schedules.
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

	if shoot.DeletionTimestamp != nil {
		log.Info("Shoot is currently being deleted, stopping reconciliation")
		return reconcile.Result{}, nil
	}

	schedules := getShootHibernationSchedules(shoot.Spec.Hibernation)
	if len(schedules) == 0 {
		log.Info("Hibernation schedules have been removed from shoot, stopping reconciliation")
		return reconcile.Result{}, nil
	}

	parsedSchedules, err := parseHibernationSchedules(schedules)
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

func (r *Reconciler) hibernateOrWakeUpShootBasedOnSchedule(ctx context.Context, shoot *gardencorev1beta1.Shoot, schedule *parsedHibernationSchedule, now time.Time) error {
	patch := client.MergeFrom(shoot.DeepCopy())
	switch schedule.operation {
	case hibernate:
		shoot.Spec.Hibernation.Enabled = ptr.To(true)
		r.Recorder.Event(shoot, corev1.EventTypeNormal, gardencorev1beta1.ShootEventHibernationEnabled, "Hibernating cluster due to schedule")
	case wakeUp:
		shoot.Spec.Hibernation.Enabled = ptr.To(false)
		r.Recorder.Event(shoot, corev1.EventTypeNormal, gardencorev1beta1.ShootEventHibernationDisabled, "Waking up cluster due to schedule")
	}
	if err := r.Client.Patch(ctx, shoot, patch); err != nil {
		return err
	}

	patch = client.MergeFrom(shoot.DeepCopy())
	shoot.Status.LastHibernationTriggerTime = &metav1.Time{Time: now}
	return r.Client.Status().Patch(ctx, shoot, patch)
}

// parseHibernationSchedules parses the given HibernationSchedules and returns an array of ParsedHibernationSchedules
// If the Location of a HibernationSchedule is `nil`, it is defaulted to UTC.
func parseHibernationSchedules(schedules []gardencorev1beta1.HibernationSchedule) ([]parsedHibernationSchedule, error) {
	var parsedHibernationSchedules []parsedHibernationSchedule

	for _, schedule := range schedules {
		locationID := time.UTC.String()
		if schedule.Location != nil {
			locationID = *schedule.Location
		}

		location, err := time.LoadLocation(locationID)
		if err != nil {
			return nil, err
		}

		if schedule.Start != nil {
			parsed, err := cron.ParseStandard(*schedule.Start)
			if err != nil {
				return nil, err
			}
			parsedHibernationSchedules = append(parsedHibernationSchedules,
				parsedHibernationSchedule{location: *location, schedule: parsed, operation: hibernate},
			)
		}

		if schedule.End != nil {
			parsed, err := cron.ParseStandard(*schedule.End)
			if err != nil {
				return nil, err
			}
			parsedHibernationSchedules = append(parsedHibernationSchedules,
				parsedHibernationSchedule{location: *location, schedule: parsed, operation: wakeUp},
			)
		}
	}

	return parsedHibernationSchedules, nil
}

// nextHibernationTimeDuration returns the time duration after which to requeue the shoot based on the hibernation schedules and current time.
// It adds a 100ms padding to the next requeue to account for Network Time Protocol(NTP) time skews.
// If the time drifts are adjusted which in most realistic cases would be around 100ms, scheduled hibernation
// will still be executed without missing the schedule.
func nextHibernationTimeDuration(schedules []parsedHibernationSchedule, now time.Time) time.Duration {
	timeStamps := make([]time.Time, 0, len(schedules))
	for _, schedule := range schedules {
		timeStamps = append(timeStamps, schedule.next(now))
	}

	slices.SortFunc(timeStamps, func(a, b time.Time) int {
		return a.Compare(b)
	})

	return timeStamps[0].Add(nextScheduleDelta).Sub(now)
}

// getScheduleWithMostRecentTime returns the ParsedHibernationSchedule that contains the schedule with the most recent (previous) execution time.
func getScheduleWithMostRecentTime(schedules []parsedHibernationSchedule, triggerDeadlineDuration *metav1.Duration, shoot *gardencorev1beta1.Shoot, now time.Time) *parsedHibernationSchedule {
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
	var scheduleWithMostRecentTime *parsedHibernationSchedule
	for i := range schedules {
		cur := schedules[i].previous(earliestTime, now)
		if cur == nil {
			continue
		}
		if scheduleWithMostRecentTime == nil {
			scheduleWithMostRecentTime = &schedules[i]
			continue
		}
		mostRecentTime := scheduleWithMostRecentTime.previous(earliestTime, now)
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

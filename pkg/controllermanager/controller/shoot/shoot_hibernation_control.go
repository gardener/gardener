// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// you may not use this file except in compliance with the License.
// Licensed under the Apache License, Version 2.0 (the "License");
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package shoot

import (
	"context"
	"reflect"
	"sort"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	gardenlogger "github.com/gardener/gardener/pkg/logger"
	"github.com/go-logr/logr"
	"github.com/robfig/cron"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

func (c *Controller) shootHibernationAdd(obj interface{}) {
	shoot, ok := obj.(*gardencorev1beta1.Shoot)
	if !ok {
		return
	}

	if shootHasHibernationSchedules(shoot) {
		key, err := cache.MetaNamespaceKeyFunc(obj)
		if err != nil {
			gardenlogger.Logger.Errorf("Couldn't get key for object %+v: %v", obj, err)
			return
		}
		c.shootHibernationQueue.Add(key)
	}
}

func (c *Controller) shootHibernationUpdate(oldObj, newObj interface{}) {
	var (
		oldShoot = oldObj.(*gardencorev1beta1.Shoot)
		newShoot = newObj.(*gardencorev1beta1.Shoot)

		oldSchedule = getShootHibernationSchedules(oldShoot)
		newSchedule = getShootHibernationSchedules(newShoot)
	)

	if !reflect.DeepEqual(oldSchedule, newSchedule) && len(newSchedule) > 0 {
		key, err := cache.MetaNamespaceKeyFunc(newObj)
		if err != nil {
			gardenlogger.Logger.Errorf("Couldn't get key for object %+v: %v", newObj, err)
			return
		}
		parsedSchedules, err := parseHibernationSchedules(newSchedule)
		if err != nil {
			gardenlogger.Logger.Infof("Could not parse hibernation schedules for shoot %s: %v", client.ObjectKeyFromObject(newShoot), err)
			return
		}
		requeueAfter := nextHibernationTimeDuration(parsedSchedules, TimeNow())
		c.shootHibernationQueue.AddAfter(key, requeueAfter)
	}
}

// ControllerName is the name of the controller.
const ControllerName = "hibernation"

// TimeNow returns the current time. Exposed for testing.
var TimeNow = time.Now

// ParsedHibernationSchedule holds the loaded location, parsed cron schedule and information whether
// the cluster should be hibernated or woken up.
type ParsedHibernationSchedule struct {
	Location *time.Location
	Schedule cron.Schedule
	Enabled  bool
}

// Next returns the next activation time in UTC, later than the given time.
// The given time is converted in the schedule's location.
func (s *ParsedHibernationSchedule) Next(t time.Time) time.Time {
	return s.Schedule.Next(t.In(s.Location)).UTC()
}

// Prev returns the previous activation time in UTC that is between the two given times.
// The times are converted in the schedule's location. It returns nil if no such activation time can be found.
func (s *ParsedHibernationSchedule) Prev(from, to time.Time) *time.Time {
	if from.After(to) {
		return nil
	}

	t1 := s.Schedule.Next(from.In(s.Location))
	if t1.After(to) {
		return nil
	}

	for {
		t2 := s.Schedule.Next(t1)
		if t2.After(to) {
			break
		}
		t1 = t2
	}
	inUTC := t1.UTC()
	return &inUTC
}

// NewShootHibernationReconciler creates a new instance of a reconciler which hibernates shoots or wakes them up.
func NewShootHibernationReconciler(
	gardenClient client.Client,
	config config.ShootHibernationControllerConfiguration,
	recorder record.EventRecorder,
) reconcile.Reconciler {
	return &shootHibernationReconciler{
		gardenClient: gardenClient,
		config:       config,
		recorder:     recorder,
	}
}

type shootHibernationReconciler struct {
	gardenClient client.Client
	config       config.ShootHibernationControllerConfiguration
	recorder     record.EventRecorder
}

func (r *shootHibernationReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx).WithName(ControllerName)

	shoot := &gardencorev1beta1.Shoot{}
	if err := r.gardenClient.Get(ctx, request.NamespacedName, shoot); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Shoot is gone, stopping reconciliation", "shoot", client.ObjectKeyFromObject(shoot))
			return reconcile.Result{}, nil
		}
		log.Error(err, "Unable to retrieve shoot from store", "shoot", client.ObjectKeyFromObject(shoot))
		return reconcile.Result{}, err
	}

	log = log.WithValues("shoot", client.ObjectKeyFromObject(shoot))

	if shoot.DeletionTimestamp != nil {
		log.Info("Shoot is currently being deleted, stopping reconciliation")
		return reconcile.Result{}, nil
	}
	return r.reconcile(ctx, shoot, log)
}

func (r *shootHibernationReconciler) reconcile(ctx context.Context, shoot *gardencorev1beta1.Shoot, log logr.Logger) (reconcile.Result, error) {
	schedules := getShootHibernationSchedules(shoot)
	if schedules == nil {
		log.Info("Hibernation schedules have been removed from shoot, stopping reconciliation")
		return reconcile.Result{}, nil
	}

	parsedSchedules, err := parseHibernationSchedules(schedules)
	if err != nil {
		log.Info("Invalid hibernation schedules, stopping reconciliation")
		return reconcile.Result{}, nil
	}

	// get the schedule which caused the current reconciliation, to check whether the shoot should be hibernated or woken up
	mostRecentSchedule := getScheduleWithMostRecentTime(parsedSchedules, TimeNow(), r.config.TriggerDeadlineDuration, shoot)
	if mostRecentSchedule != nil {
		patch := client.MergeFrom(shoot.DeepCopy())
		shoot.Spec.Hibernation.Enabled = &mostRecentSchedule.Enabled
		if err = r.gardenClient.Patch(ctx, shoot, patch); err != nil {
			return reconcile.Result{}, err
		}

		patch = client.MergeFrom(shoot.DeepCopy())
		hibernationTriggerTime := v1.NewTime(TimeNow())
		shoot.Status.LastHibernationTriggerTime = &hibernationTriggerTime
		if err = r.gardenClient.Status().Patch(ctx, shoot, patch); err != nil {
			return reconcile.Result{}, err
		}
		if mostRecentSchedule.Enabled {
			r.recorder.Event(shoot, corev1.EventTypeNormal, gardencorev1beta1.ShootEventHibernationEnabled, "Hibernating cluster due to schedule")
		} else {
			r.recorder.Event(shoot, corev1.EventTypeNormal, gardencorev1beta1.ShootEventHibernationDisabled, "Waking up cluster due to schedule")
		}
		log.Info("Successfully set shoot hibernation", "hibernation", mostRecentSchedule.Enabled)
	}

	requeueAfter := nextHibernationTimeDuration(parsedSchedules, TimeNow())
	log.Info("Requeuing hibernation", "requeueAfter", requeueAfter)
	return reconcile.Result{RequeueAfter: requeueAfter}, nil
}

// parseHibernationSchedules parses the given HibernationSchedules and returns an array of ParsedHibernationSchedules
// If the Location of a HibernationSchedule is `nil`, it is defaulted to UTC.
func parseHibernationSchedules(schedules []gardencorev1beta1.HibernationSchedule) ([]ParsedHibernationSchedule, error) {
	var parsedHibernationSchedules []ParsedHibernationSchedule

	for _, schedule := range schedules {
		var locationID string
		if schedule.Location != nil {
			locationID = *schedule.Location
		} else {
			locationID = time.UTC.String()
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
				ParsedHibernationSchedule{Location: location, Schedule: parsed, Enabled: true},
			)
		}
		if schedule.End != nil {
			parsed, err := cron.ParseStandard(*schedule.End)
			if err != nil {
				return nil, err
			}
			parsedHibernationSchedules = append(parsedHibernationSchedules,
				ParsedHibernationSchedule{Location: location, Schedule: parsed, Enabled: false},
			)
		}
	}

	return parsedHibernationSchedules, nil
}

// nextHibernationTimeDuration returns the time duration after which to requeue the shoot based on the hibernation schedules and current time.
func nextHibernationTimeDuration(schedules []ParsedHibernationSchedule, now time.Time) time.Duration {
	var timestamps []time.Time
	for _, schedule := range schedules {
		ts := schedule.Next(now)
		timestamps = append(timestamps, ts)
	}

	sort.Slice(timestamps, func(i, j int) bool {
		return timestamps[i].Before(timestamps[j])
	})

	duration := timestamps[0].Sub(now)
	return duration
}

// getScheduleWithMostRecentTime returns the ParsedHibernationSchedule that contains the schedule with the most recent previous execution time.
func getScheduleWithMostRecentTime(schedules []ParsedHibernationSchedule, now time.Time, triggerDeadlineDuration *metav1.Duration, shoot *gardencorev1beta1.Shoot) *ParsedHibernationSchedule {
	var startTime time.Time
	if shoot.Status.LastHibernationTriggerTime != nil {
		startTime = shoot.Status.LastHibernationTriggerTime.Time
	} else {
		// if the shoot has just been created or has never been hibernated, use the creation timestamp
		startTime = shoot.CreationTimestamp.Time
	}

	if triggerDeadlineDuration != nil {
		triggerDeadline := now.Add(-triggerDeadlineDuration.Duration)

		if triggerDeadline.After(startTime) {
			startTime = triggerDeadline
		}
	}

	if startTime.After(now) {
		return nil
	}

	var scheduleWithMostRecentTime *ParsedHibernationSchedule
	for i := 0; i < len(schedules); i++ {
		cur := schedules[i].Prev(startTime, now)
		if cur == nil {
			continue
		}
		if scheduleWithMostRecentTime == nil {
			scheduleWithMostRecentTime = &schedules[i]
			continue
		}
		mostRecentTime := scheduleWithMostRecentTime.Prev(startTime, now)
		if mostRecentTime == nil {
			continue
		}
		if cur.After(*mostRecentTime) {
			scheduleWithMostRecentTime = &schedules[i]
		}
	}

	return scheduleWithMostRecentTime
}

func shootHasHibernationSchedules(shoot *gardencorev1beta1.Shoot) bool {
	return getShootHibernationSchedules(shoot) != nil
}

func getShootHibernationSchedules(shoot *gardencorev1beta1.Shoot) []gardencorev1beta1.HibernationSchedule {
	hibernation := shoot.Spec.Hibernation
	if hibernation == nil {
		return nil
	}
	return hibernation.Schedules
}

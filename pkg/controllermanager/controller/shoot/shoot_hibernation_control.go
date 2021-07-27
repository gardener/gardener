// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
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
	"fmt"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	"github.com/go-logr/logr"

	"github.com/robfig/cron"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	// ShootHibernationControllerName is the name of the shoot-hibernation controller.
	ShootHibernationControllerName = "shoot-hibernation"
)

func addShootHibernationController(mgr manager.Manager, config *config.ShootHibernationControllerConfiguration) error {
	logger := mgr.GetLogger()
	gardenClient := mgr.GetClient()
	recorder := mgr.GetEventRecorderFor("controller-" + ShootHibernationControllerName)
	reconciler := NewShootHibernationReconciler(logger, gardenClient, NewHibernationScheduleRegistry(), recorder)

	ctrlOptions := controller.Options{
		Reconciler:              reconciler,
		MaxConcurrentReconciles: config.ConcurrentSyncs,
	}
	c, err := controller.New(ShootHibernationControllerName, mgr, ctrlOptions)
	if err != nil {
		return err
	}

	reconciler.logger = c.GetLogger()

	shoot := &gardencorev1beta1.Shoot{}
	if err := c.Watch(&source.Kind{Type: shoot}, &handler.EnqueueRequestForObject{}); err != nil {
		return fmt.Errorf("failed to create watcher for %T: %w", shoot, err)
	}

	return nil
}

func getShootHibernationSchedules(shoot *gardencorev1beta1.Shoot) []gardencorev1beta1.HibernationSchedule {
	hibernation := shoot.Spec.Hibernation
	if hibernation == nil {
		return nil
	}
	return hibernation.Schedules
}

var (
	// NewCronWithLocation creates a new cron with the given location. Exposed for testing.
	NewCronWithLocation = newCronWithLocation

	// TimeNow returns the current time. Exposed for testing.
	TimeNow = time.Now
)

func newCronWithLocation(location *time.Location) Cron {
	return cron.NewWithLocation(location)
}

// GroupHibernationSchedulesByLocation groups the given HibernationSchedules by their Location.
// If the Location of a HibernationSchedule is `nil`, it is defaulted to UTC.
func GroupHibernationSchedulesByLocation(schedules []gardencorev1beta1.HibernationSchedule) map[string][]gardencorev1beta1.HibernationSchedule {
	var (
		locationToSchedules = make(map[string][]gardencorev1beta1.HibernationSchedule)
	)

	for _, schedule := range schedules {
		var locationID string
		if schedule.Location != nil {
			locationID = *schedule.Location
		} else {
			locationID = time.UTC.String()
		}

		locationToSchedules[locationID] = append(locationToSchedules[locationID], schedule)
	}

	return locationToSchedules
}

// ComputeHibernationSchedule computes the HibernationSchedule for the given Shoot.
func ComputeHibernationSchedule(ctx context.Context, gardenClient client.Client, logger logr.Logger, recorder record.EventRecorder, shoot *gardencorev1beta1.Shoot) (HibernationSchedule, error) {
	var (
		schedules           = getShootHibernationSchedules(shoot)
		locationToSchedules = GroupHibernationSchedulesByLocation(schedules)
		schedule            = make(HibernationSchedule, len(locationToSchedules))
	)

	for locationID, schedules := range locationToSchedules {
		location, err := time.LoadLocation(locationID)
		if err != nil {
			return nil, err
		}

		cr := NewCronWithLocation(location)
		cronLogger := logger.WithValues("location", location)
		for _, schedule := range schedules {
			if schedule.Start != nil {
				start, err := cron.ParseStandard(*schedule.Start)
				if err != nil {
					return nil, err
				}

				cr.Schedule(start, NewHibernationJob(ctx, gardenClient, cronLogger, recorder, shoot, true))
				cronLogger.Info("Scheduled hibernation", "spec", *schedule.Start, "triggered", start.Next(TimeNow().UTC()))
			}

			if schedule.End != nil {
				end, err := cron.ParseStandard(*schedule.End)
				if err != nil {
					return nil, err
				}

				cr.Schedule(end, NewHibernationJob(ctx, gardenClient, cronLogger, recorder, shoot, false))
				cronLogger.Info("Scheduled wakeup", "spec", *schedule.End, "triggered", end.Next(TimeNow().UTC()))
			}
		}
		schedule[locationID] = cr
	}

	return schedule, nil
}

func shootHasHibernationSchedules(shoot *gardencorev1beta1.Shoot) bool {
	return getShootHibernationSchedules(shoot) != nil
}

// NewShootHibernationReconciler creates a new instance of a reconciler which hibernates shoots or wakes them up.
func NewShootHibernationReconciler(
	l logr.Logger,
	gardenClient client.Client,
	hibernationScheduleRegistry HibernationScheduleRegistry,
	recorder record.EventRecorder,
) *shootHibernationReconciler {
	return &shootHibernationReconciler{
		logger:                      l,
		gardenClient:                gardenClient,
		hibernationScheduleRegistry: hibernationScheduleRegistry,
		recorder:                    recorder,
	}
}

type shootHibernationReconciler struct {
	logger                      logr.Logger
	gardenClient                client.Client
	hibernationScheduleRegistry HibernationScheduleRegistry
	recorder                    record.EventRecorder
}

func (r *shootHibernationReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	key := fmt.Sprintf("%s/%s", request.Namespace, request.Name)
	logger := r.logger.WithValues("shoot", request)

	shoot := &gardencorev1beta1.Shoot{}
	if err := r.gardenClient.Get(ctx, request.NamespacedName, shoot); err != nil {
		if apierrors.IsNotFound(err) {
			r.deleteShootCron(logger, key)
			logger.Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}

		logger.Error(err, "Unable to retrieve object from store")
		return reconcile.Result{}, err
	}

	logger.Info("Reconciling")

	if shoot.DeletionTimestamp != nil {
		r.deleteShootCron(logger, key)
		return reconcile.Result{}, nil
	}

	return reconcile.Result{}, r.createOrUpdateShootCron(ctx, logger, key, shoot)
}

func (r *shootHibernationReconciler) deleteShootCron(logger logr.Logger, key string) {
	if sched, ok := r.hibernationScheduleRegistry.Load(key); ok {
		sched.Stop()
		logger.Info("Stopped cron")
	}

	r.hibernationScheduleRegistry.Delete(key)
	logger.Info("Deleted cron")
}

func (r *shootHibernationReconciler) createOrUpdateShootCron(ctx context.Context, logger logr.Logger, key string, shoot *gardencorev1beta1.Shoot) error {
	r.deleteShootCron(logger, key)
	if !shootHasHibernationSchedules(shoot) {
		return nil
	}

	schedule, err := ComputeHibernationSchedule(ctx, r.gardenClient, logger, r.recorder, shoot)
	if err != nil {
		return err
	}

	schedule.Start()
	r.hibernationScheduleRegistry.Store(key, schedule)
	logger.Info("Successfully started hibernation schedule")

	return nil
}

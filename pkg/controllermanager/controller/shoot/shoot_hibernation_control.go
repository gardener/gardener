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
	"reflect"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardenlogger "github.com/gardener/gardener/pkg/logger"
	"github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
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

	if !reflect.DeepEqual(oldSchedule, newSchedule) {
		key, err := cache.MetaNamespaceKeyFunc(newObj)
		if err != nil {
			gardenlogger.Logger.Errorf("Couldn't get key for object %+v: %v", newObj, err)
			return
		}
		c.shootHibernationQueue.Add(key)
	}
}

func (c *Controller) shootHibernationDelete(obj interface{}) {
	shoot, ok := obj.(*gardencorev1beta1.Shoot)
	if !ok {
		return
	}

	if shootHasHibernationSchedules(shoot) {
		key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
		if err != nil {
			gardenlogger.Logger.Errorf("Couldn't get key for object %+v: %v", obj, err)
			return
		}
		c.shootHibernationQueue.Add(key)
	}
}

var (
	// TimeNow returns the current time. Exposed for testing.
	TimeNow = time.Now
)

// NewShootHibernationReconciler creates a new instance of a reconciler which hibernates shoots or wakes them up.
func NewShootHibernationReconciler(
	l logrus.FieldLogger,
	gardenClient client.Client,
	recorder record.EventRecorder,
) reconcile.Reconciler {
	return &shootHibernationReconciler{
		logger:       l,
		gardenClient: gardenClient,
		recorder:     recorder,
	}
}

type shootHibernationReconciler struct {
	logger       logrus.FieldLogger
	gardenClient client.Client
	recorder     record.EventRecorder
}

func (r *shootHibernationReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	key := fmt.Sprintf("%s/%s", request.Namespace, request.Name)

	shoot := &gardencorev1beta1.Shoot{}
	if err := r.gardenClient.Get(ctx, request.NamespacedName, shoot); err != nil {
		if apierrors.IsNotFound(err) {
			r.logger.Infof("Object %q is gone, stop reconciling: %v", request.Name, err)
			return reconcile.Result{}, nil
		}
		r.logger.Infof("Unable to retrieve object %q from store: %v", request.Name, err)
		return reconcile.Result{}, err
	}

	logger := r.logger.WithField("shoot-hibernation", key)
	logger.Info("[SHOOT HIBERNATION]")

	if shoot.DeletionTimestamp != nil {
		return reconcile.Result{}, nil
	}

	return reconcile.Result{}, nil
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

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
	"fmt"
	garden "github.com/gardener/gardener/pkg/client/garden/clientset/versioned"
	"github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/client-go/util/retry"
	"reflect"
	"time"

	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	gardenlogger "github.com/gardener/gardener/pkg/logger"
	"github.com/robfig/cron"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/cache"
)

func hibernationLogger(key string) logrus.FieldLogger {
	return gardenlogger.Logger.WithFields(logrus.Fields{
		"controller": "shoot-hibernation",
		"key":        key,
	})
}

func getShootHibernationSchedules(shoot *gardenv1beta1.Shoot) []gardenv1beta1.HibernationSchedule {
	hibernation := shoot.Spec.Hibernation
	if hibernation == nil {
		return nil
	}
	return hibernation.Schedules
}

func shootHasHibernationSchedules(shoot *gardenv1beta1.Shoot) bool {
	return getShootHibernationSchedules(shoot) != nil
}

func (c *Controller) shootHibernationAdd(obj interface{}) {
	if shootHasHibernationSchedules(obj.(*gardenv1beta1.Shoot)) {
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
		oldShoot = oldObj.(*gardenv1beta1.Shoot)
		newShoot = newObj.(*gardenv1beta1.Shoot)

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
	if shootHasHibernationSchedules(obj.(*gardenv1beta1.Shoot)) {
		key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
		if err != nil {
			gardenlogger.Logger.Errorf("Couldn't get key for object %+v: %v", obj, err)
			return
		}
		c.shootHibernationQueue.Add(key)
	}
}

func (c *Controller) deleteShootCron(logger logrus.FieldLogger, key string) {
	if cr, ok := c.shootToHibernationCron[key]; ok {
		cr.Stop()
		logger.Debugf("Stopped cron")
	}
	delete(c.shootToHibernationCron, key)
	logger.Debugf("Deleted cron")
}

func (c *Controller) reconcileShootHibernationKey(key string) error {
	logger := hibernationLogger(key)
	logger.Info("Shoot Hibernation")

	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	shoot, err := c.shootLister.Shoots(namespace).Get(name)
	if apierrors.IsNotFound(err) {
		c.deleteShootCron(logger, key)
		logger.Debugf("Skipping because Shoot has been deleted", key)
		return nil
	}
	if err != nil {
		logger.Debugf("Unable to retrieve object from store: %v", key, err)
		return err
	}

	if shoot.DeletionTimestamp != nil {
		c.deleteShootCron(logger, key)
		return nil
	}
	return c.reconcileShootHibernation(logger, key, shoot.DeepCopy())
}

func (c *Controller) shootHibernationJob(logger logrus.FieldLogger, client garden.Interface, target *gardenv1beta1.Shoot, enabled bool) cron.Job {
	return cron.FuncJob(func() {
		_, err := kubernetes.TryUpdateShootHibernation(client, retry.DefaultBackoff, target.ObjectMeta,
			func(shoot *gardenv1beta1.Shoot) (*gardenv1beta1.Shoot, error) {
				if shoot.Spec.Hibernation == nil || !equality.Semantic.DeepEqual(target.Spec.Hibernation.Schedules, shoot.Spec.Hibernation.Schedules) {
					return nil, fmt.Errorf("shoot %s/%s hibernation schedule changed mid-air", shoot.Namespace, shoot.Name)
				}
				shoot.Spec.Hibernation.Enabled = enabled
				return shoot, nil
			})
		if err != nil {
			logger.Errorf("Could not set hibernation.enabled to %t: %+v", enabled, err)
			return
		}
		logger.Debugf("Successfully set hibernation.enabled to %t", enabled)
	})
}

func (c *Controller) reconcileShootHibernation(logger logrus.FieldLogger, key string, shoot *gardenv1beta1.Shoot) error {
	var (
		schedules = getShootHibernationSchedules(shoot)
		client    = c.k8sGardenClient.Garden()
	)

	c.deleteShootCron(logger, key)
	if len(schedules) == 0 {
		return nil
	}

	cr := cron.NewWithLocation(time.UTC)
	for _, schedule := range schedules {
		if schedule.Start != nil {
			start, err := cron.ParseStandard(*schedule.Start)
			if err != nil {
				return err
			}

			cr.Schedule(start, c.shootHibernationJob(logger, client, shoot, true))
		}

		if schedule.End != nil {
			end, err := cron.ParseStandard(*schedule.End)
			if err != nil {
				return err
			}

			cr.Schedule(end, c.shootHibernationJob(logger, client, shoot, false))
		}

	}
	c.shootToHibernationCron[key] = cr
	cr.Start()
	logger.Debugf("Successfully started hibernation schedule")

	return nil
}

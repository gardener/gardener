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
	"sync"

	"github.com/sirupsen/logrus"

	garden "github.com/gardener/gardener/pkg/client/garden/clientset/versioned"

	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/robfig/cron"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/client-go/util/retry"
)

// Cron is an interface that allows mocking cron.Cron.
type Cron interface {
	Schedule(schedule cron.Schedule, job cron.Job)
	Start()
	Stop()
}

// HibernationSchedule is a mapping from location to a Cron.
// It controls the hibernation process of a certain shoot.
type HibernationSchedule map[string]Cron

// Stop implements Cron.
func (h *HibernationSchedule) Stop() {
	for _, c := range *h {
		c.Stop()
	}
}

// Start implements Cron.
func (h *HibernationSchedule) Start() {
	for _, c := range *h {
		c.Start()
	}
}

type hibernationScheduleRegistry struct {
	data sync.Map
}

// HibernationScheduleRegistry is a goroutine-safe mapping of Shoot key to HibernationSchedule.
type HibernationScheduleRegistry interface {
	Load(key string) (schedule HibernationSchedule, ok bool)
	Store(key string, schedule HibernationSchedule)
	Delete(key string)
}

// Store implements HibernationScheduleRegistry.
func (h *hibernationScheduleRegistry) Store(key string, schedule HibernationSchedule) {
	h.data.Store(key, schedule)
}

// Delete implements HibernationScheduleRegistry.
func (h *hibernationScheduleRegistry) Delete(key string) {
	h.data.Delete(key)
}

// Load implements HibernationScheduleRegistry.
func (h *hibernationScheduleRegistry) Load(key string) (schedule HibernationSchedule, ok bool) {
	sched, ok := h.data.Load(key)
	if !ok {
		return nil, false
	}
	return sched.(HibernationSchedule), ok
}

// NewHibernationScheduleRegistry instantiates a new HibernationScheduleRegistry.
func NewHibernationScheduleRegistry() HibernationScheduleRegistry {
	return &hibernationScheduleRegistry{}
}

type hibernationJob struct {
	client  garden.Interface
	logger  logrus.FieldLogger
	target  *gardenv1beta1.Shoot
	enabled bool
}

// Run implements cron.Job.
func (h *hibernationJob) Run() {
	_, err := kubernetes.TryUpdateShootHibernation(h.client, retry.DefaultBackoff, h.target.ObjectMeta,
		func(shoot *gardenv1beta1.Shoot) (*gardenv1beta1.Shoot, error) {
			if shoot.Spec.Hibernation == nil || !equality.Semantic.DeepEqual(h.target.Spec.Hibernation.Schedules, shoot.Spec.Hibernation.Schedules) {
				return nil, fmt.Errorf("shoot %s/%s hibernation schedule changed mid-air", shoot.Namespace, shoot.Name)
			}
			shoot.Spec.Hibernation.Enabled = h.enabled
			return shoot, nil
		})
	if err != nil {
		h.logger.Errorf("Could not set hibernation.enabled to %t: %+v", h.enabled, err)
		return
	}
	h.logger.Debugf("Successfully set hibernation.enabled to %t", h.enabled)
}

// NewHibernationJob creates a new cron.Job that sets the hibernation of the given shoot to enabled when it triggers.
func NewHibernationJob(client garden.Interface, logger logrus.FieldLogger, target *gardenv1beta1.Shoot, enabled bool) cron.Job {
	return &hibernationJob{client, logger, target, enabled}
}

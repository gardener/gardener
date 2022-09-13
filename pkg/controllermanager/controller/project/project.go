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

package project

import (
	"context"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
)

// ControllerName is the name of this controller.
const ControllerName = "project"

// Controller controls Projects.
type Controller struct {
	cache client.Reader
	log   logr.Logger

	hasSyncedFuncs []cache.InformerSynced

	workerCh               chan int
	numberOfRunningWorkers int
}

// NewProjectController takes a Kubernetes client for the Garden clusters <k8sGardenClient>, a struct
// holding information about the acting Gardener, a <projectInformer>, and a <recorder> for
// event recording. It creates a new Gardener controller.
func NewProjectController(
	ctx context.Context,
	log logr.Logger,
	mgr manager.Manager,
	config *config.ControllerManagerConfiguration,
	clock clock.Clock,
) (
	*Controller,
	error,
) {
	log = log.WithName(ControllerName)

	projectController := &Controller{
		log:      log,
		workerCh: make(chan int),
	}

	return projectController, nil
}

// Run runs the Controller until the given stop channel can be read from.
func (c *Controller) Run(ctx context.Context, workers int) {
	var waitGroup sync.WaitGroup

	if !cache.WaitForCacheSync(ctx.Done(), c.hasSyncedFuncs...) {
		c.log.Error(wait.ErrWaitTimeout, "Timed out waiting for caches to sync")
		return
	}

	// Count number of running workers.
	go func() {
		for res := range c.workerCh {
			c.numberOfRunningWorkers += res
		}
	}()

	c.log.Info("Project controller initialized")

	for i := 0; i < workers; i++ {
	}

	// Shutdown handling
	<-ctx.Done()

	for {
		if c.numberOfRunningWorkers == 0 {
			c.log.V(1).Info("No running Project worker and no items left in the queues. Terminating Project controller")
			break
		}
		c.log.V(1).Info(
			"Waiting for Project workers to finish",
			"numberOfRunningWorkers", c.numberOfRunningWorkers,
		)
		time.Sleep(5 * time.Second)
	}

	waitGroup.Wait()
}

func updateStatus(ctx context.Context, c client.Client, project *gardencorev1beta1.Project, transform func()) error {
	patch := client.StrategicMergeFrom(project.DeepCopy())
	transform()
	return c.Status().Patch(ctx, project, patch)
}

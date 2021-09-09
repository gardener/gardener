// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package extensions

import (
	"context"
	"fmt"
	"sync"
	"time"

	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/gardenlet"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/sirupsen/logrus"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Controller watches the extension resources and has several control loops.
type Controller struct {
	log *logrus.Logger

	initialized            bool
	waitGroup              sync.WaitGroup
	workerCh               chan int
	numberOfRunningWorkers int

	controllerArtifacts           controllerArtifacts
	controllerInstallationControl *controllerInstallationControl
	shootStateControl             *ShootStateControl
}

// NewController creates new controller that syncs extensions states to ShootState
func NewController(gardenClient, seedClient kubernetes.Interface, seedName string, log *logrus.Logger, recorder record.EventRecorder) *Controller {
	controller := &Controller{
		log:      log,
		workerCh: make(chan int),

		controllerArtifacts: newControllerArtifacts(),
		controllerInstallationControl: &controllerInstallationControl{
			k8sGardenClient:             gardenClient,
			seedClient:                  seedClient,
			seedName:                    seedName,
			log:                         log,
			controllerInstallationQueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "controllerinstallation-extension-required"),
			lock:                        &sync.RWMutex{},
			kindToRequiredTypes:         make(map[string]sets.String),
		},
		shootStateControl: NewShootStateControl(gardenClient, seedClient, log, recorder),
	}

	return controller
}

// Initialize sets up all necessary dependencies to run this controller.
// This function must be called before Run is executed.
func (c *Controller) Initialize(ctx context.Context, seedClient kubernetes.Interface) error {
	if err := c.controllerArtifacts.initialize(ctx, seedClient); err != nil {
		return err
	}
	c.initialized = true
	return nil
}

// Run creates workers that reconciles extension resources.
// Initialize must be called before running the controller.
func (c *Controller) Run(ctx context.Context, controllerInstallationWorkers, shootStateWorkers int) {
	if !c.initialized {
		c.log.Fatal("controller is not initialized")
		return
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, time.Minute*2)
	defer cancel()

	if !cache.WaitForCacheSync(timeoutCtx.Done(), c.controllerArtifacts.hasSyncedFuncs...) {
		c.log.Fatal("timeout waiting for caches to sync")
		return
	}

	// Count number of running workers.
	go func() {
		for res := range c.workerCh {
			c.numberOfRunningWorkers += res
			c.log.Debugf("Current number of running extension controller workers is %d", c.numberOfRunningWorkers)
		}
	}()

	for i := 0; i < controllerInstallationWorkers; i++ {
		c.createControllerInstallationWorkers(ctx, c.controllerInstallationControl)
	}

	for i := 0; i < shootStateWorkers; i++ {
		c.createShootStateWorkers(ctx, c.shootStateControl)
	}

	c.log.Info("Extension controller initialized.")
}

func (c *Controller) createControllerInstallationWorkers(ctx context.Context, control *controllerInstallationControl) {
	controllerutils.CreateWorker(ctx, c.controllerInstallationControl.controllerInstallationQueue, "ControllerInstallation-Required", reconcile.Func(control.reconcileControllerInstallationRequired), &c.waitGroup, c.workerCh)

	for kind, artifact := range c.controllerArtifacts.controllerInstallationArtifacts {
		workerName := fmt.Sprintf("ControllerInstallation-Extension-%s", kind)
		controlFn := control.createExtensionRequiredReconcileFunc(kind, artifact.newListFunc)
		// Execute control function once outside of the worker to initialize the `kindToRequiredTypes` map once.
		// This is necessary for Kinds which are registered but no extension object exists in the seed yet (e.g. disabled backups).
		// In this case no event is triggered and the control function would never be executed.
		// Eventually, the Kind would never be part of the `kindToRequiredTypes` map and no decision if the ControllerInstallation is required could be taken.
		if _, err := controlFn(ctx, reconcile.Request{}); err != nil {
			c.log.Errorf("Error during initial run of extension reconciliation: %v", err)
		}
		controllerutils.CreateWorker(ctx, artifact.queue, workerName, controlFn, &c.waitGroup, c.workerCh)
	}
}

func (c *Controller) createShootStateWorkers(ctx context.Context, control *ShootStateControl) {
	for kind, artifact := range c.controllerArtifacts.stateArtifacts {
		workerName := fmt.Sprintf("ShootState-%s", kind)
		controllerutils.CreateWorker(ctx, artifact.queue, workerName, control.CreateShootStateSyncReconcileFunc(kind, artifact.newObjFunc), &c.waitGroup, c.workerCh)
	}
}

// Stop the controller
func (c *Controller) Stop() {
	c.controllerInstallationControl.controllerInstallationQueue.ShutDown()
	c.controllerArtifacts.shutdownQueues()
	c.waitGroup.Wait()
}

func createEnqueueFunc(queue workqueue.RateLimitingInterface) func(extensionObject interface{}) {
	return func(newObj interface{}) {
		enqueue(queue, newObj)
	}
}

// createEnqueueEmptyRequestFunc creates a func that enqueues an empty reconcile.Request into the given queue, which
// can serve as a trigger for a reconciler that actually doesn't care about which object was created/deleted, but
// only about that some object of a given kind was created/deleted.
func createEnqueueEmptyRequestFunc(queue workqueue.RateLimitingInterface) func(extensionObject interface{}) {
	return func(_ interface{}) {
		queue.Add(reconcile.Request{})
	}
}

func createEnqueueOnUpdateFunc(queue workqueue.RateLimitingInterface, predicateFunc func(new, old interface{}) bool) func(newExtensionObject, oldExtensionObject interface{}) {
	return func(newObj, oldObj interface{}) {
		if predicateFunc != nil && !predicateFunc(newObj, oldObj) {
			return
		}

		enqueue(queue, newObj)
	}
}

// createEnqueueEmptyRequestOnUpdateFunc is similar to createEnqueueEmptyRequestFunc in that it enqueues an empty
// reconcile.Request, but it only does it if an update matched the given predicateFunc.
func createEnqueueEmptyRequestOnUpdateFunc(queue workqueue.RateLimitingInterface, predicateFunc func(new, old interface{}) bool) func(newExtensionObject, oldExtensionObject interface{}) {
	return func(newObj, oldObj interface{}) {
		if predicateFunc != nil && !predicateFunc(newObj, oldObj) {
			return
		}

		queue.Add(reconcile.Request{})
	}
}

func enqueue(queue workqueue.RateLimitingInterface, obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		return
	}
	queue.Add(key)
}

func extensionStateOrResourcesChanged(newObj, oldObj interface{}) bool {
	return extensionPredicateFunc(
		func(new, old extensionsv1alpha1.Object) bool {
			return !apiequality.Semantic.DeepEqual(new.GetExtensionStatus().GetState(), old.GetExtensionStatus().GetState()) ||
				!apiequality.Semantic.DeepEqual(new.GetExtensionStatus().GetResources(), old.GetExtensionStatus().GetResources())
		},
	)(newObj, oldObj)
}

func extensionTypeChanged(newObj, oldObj interface{}) bool {
	return extensionPredicateFunc(
		func(new, old extensionsv1alpha1.Object) bool {
			return old.GetExtensionSpec().GetExtensionType() != new.GetExtensionSpec().GetExtensionType()
		},
	)(newObj, oldObj)
}

func extensionPredicateFunc(f func(extensionsv1alpha1.Object, extensionsv1alpha1.Object) bool) func(interface{}, interface{}) bool {
	return func(newObj, oldObj interface{}) bool {
		var (
			newExtensionObj, ok1 = newObj.(extensionsv1alpha1.Object)
			oldExtensionObj, ok2 = oldObj.(extensionsv1alpha1.Object)
		)
		return ok1 && ok2 && f(newExtensionObj, oldExtensionObj)
	}
}

// RunningWorkers returns the number of running workers.
func (c *Controller) RunningWorkers() int {
	return c.numberOfRunningWorkers
}

// CollectMetrics implements gardenmetrics.ControllerMetricsCollector interface
func (c *Controller) CollectMetrics(ch chan<- prometheus.Metric) {
	metric, err := prometheus.NewConstMetric(gardenlet.ControllerWorkerSum, prometheus.GaugeValue, float64(c.RunningWorkers()), "extensions")
	if err != nil {
		gardenlet.ScrapeFailures.With(prometheus.Labels{"kind": "extensions-controller"}).Inc()
		return
	}
	ch <- metric
}

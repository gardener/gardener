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

package seed

import (
	"context"
	"fmt"
	"sync"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	confighelper "github.com/gardener/gardener/pkg/gardenlet/apis/config/helper"
	"github.com/gardener/gardener/pkg/healthz"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// ControllerName is the name of this controller.
const ControllerName = "seed"

// Controller controls Seeds.
type Controller struct {
	log logr.Logger

	reconciler      reconcile.Reconciler
	leaseReconciler reconcile.Reconciler
	careReconciler  reconcile.Reconciler

	seedQueue      workqueue.RateLimitingInterface
	seedLeaseQueue workqueue.RateLimitingInterface
	seedCareQueue  workqueue.RateLimitingInterface

	hasSyncedFuncs         []cache.InformerSynced
	workerCh               chan int
	numberOfRunningWorkers int
}

// NewSeedController takes a Kubernetes client for the Garden clusters <k8sGardenClient>, a struct
// holding information about the acting Gardener, a <seedInformer>, and a <recorder> for
// event recording. It creates a new Gardener controller.
func NewSeedController(
	ctx context.Context,
	log logr.Logger,
	clientMap clientmap.ClientMap,
	healthManager healthz.Manager,
	imageVector imagevector.ImageVector,
	componentImageVectors imagevector.ComponentImageVectors,
	identity *gardencorev1beta1.Gardener,
	clientCertificateExpirationTimestamp *metav1.Time,
	config *config.GardenletConfiguration,
	recorder record.EventRecorder,
) (
	*Controller,
	error,
) {
	log = log.WithName(ControllerName)

	gardenClient, err := clientMap.GetClient(ctx, keys.ForGarden())
	if err != nil {
		return nil, err
	}

	seedInformer, err := gardenClient.Cache().GetInformer(ctx, &gardencorev1beta1.Seed{})
	if err != nil {
		return nil, fmt.Errorf("failed to get Seed Informer: %w", err)
	}

	gardenletClientCertificate, err := kutil.ClientCertificateFromRESTConfig(gardenCluster.GetConfig())
	if err != nil {
		return nil, fmt.Errorf("failed to get gardenlet client certificate: %w", err)
	}
	gardenletClientCertificateExpirationTime := &metav1.Time{Time: gardenletClientCertificate.Leaf.NotAfter}
	log.Info("The client certificate used to communicate with the garden cluster has expiration date", "expirationDate", gardenletClientCertificateExpirationTime)

	seedController := &Controller{
		log: log,

		reconciler:      newReconciler(clientMap, recorder, imageVector, componentImageVectors, identity, clientCertificateExpirationTimestamp, config),
		leaseReconciler: NewLeaseReconciler(clientMap, healthManager, metav1.Now, config),
		careReconciler:  NewCareReconciler(clientMap, *config.Controllers.SeedCare),

		seedQueue:      workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "seed"),
		seedLeaseQueue: workqueue.NewNamedRateLimitingQueue(workqueue.NewItemExponentialFailureRateLimiter(time.Millisecond, 2*time.Second), "seed-lease"),
		seedCareQueue:  workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "seed-care"),

		workerCh: make(chan int),
	}

	seedInformer.AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controllerutils.SeedFilterFunc(confighelper.SeedNameFromSeedConfig(config.SeedConfig)),
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    seedController.seedAdd,
			UpdateFunc: seedController.seedUpdate,
			DeleteFunc: seedController.seedDelete,
		},
	})

	seedInformer.AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controllerutils.SeedFilterFunc(confighelper.SeedNameFromSeedConfig(config.SeedConfig)),
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc: seedController.seedLeaseAdd,
		},
	})

	seedInformer.AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controllerutils.SeedFilterFunc(confighelper.SeedNameFromSeedConfig(config.SeedConfig)),
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    seedController.seedCareAdd,
			UpdateFunc: seedController.seedCareUpdate,
		},
	})

	seedController.hasSyncedFuncs = []cache.InformerSynced{
		seedInformer.HasSynced,
	}

	return seedController, nil
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

	c.log.Info("Seed controller initialized")

	for i := 0; i < workers; i++ {
		controllerutils.CreateWorker(ctx, c.seedQueue, "Seed", c.reconciler, &waitGroup, c.workerCh, controllerutils.WithLogger(c.log.WithName(reconcilerName)))
		controllerutils.CreateWorker(ctx, c.seedLeaseQueue, "Seed Lease", c.leaseReconciler, &waitGroup, c.workerCh, controllerutils.WithLogger(c.log.WithName(leaseReconcilerName)))
	}
	controllerutils.CreateWorker(ctx, c.seedCareQueue, "Seed Care", c.careReconciler, &waitGroup, c.workerCh, controllerutils.WithLogger(c.log.WithName(careReconcilerName)))

	// Shutdown handling
	<-ctx.Done()
	c.seedQueue.ShutDown()
	c.seedLeaseQueue.ShutDown()
	c.seedCareQueue.ShutDown()

	for {
		if c.seedQueue.Len() == 0 && c.seedLeaseQueue.Len() == 0 && c.seedCareQueue.Len() == 0 && c.numberOfRunningWorkers == 0 {
			c.log.V(1).Info("No running Seed worker and no items left in the queues. Terminated Seed controller")
			break
		}
		c.log.V(1).Info("Waiting for Seed workers to finish", "numberOfRunningWorkers", c.numberOfRunningWorkers, "queueLength", c.seedQueue.Len()+c.seedLeaseQueue.Len()+c.seedCareQueue.Len())
		time.Sleep(5 * time.Second)
	}

	waitGroup.Wait()
}

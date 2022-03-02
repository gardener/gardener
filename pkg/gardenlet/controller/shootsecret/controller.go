// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package shootsecret

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/logger"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	// ControllerName is the name of this controller.
	ControllerName = "shoot-secret-controller"
)

// Controller controls Secret in the seed cluster and persists them in the ShootState resource.
type Controller struct {
	reconciler reconcile.Reconciler

	hasSyncedFuncs []cache.InformerSynced
	secretQueue    workqueue.RateLimitingInterface

	workerCh               chan int
	numberOfRunningWorkers int
}

// NewController creates a new controller for secrets in the seed cluster which must be persisted to the ShootState in
// the garden cluster.
func NewController(ctx context.Context,
	gardenClient client.Client,
	seedClientSet kubernetes.Interface,
	log *logrus.Logger,
) (
	*Controller,
	error,
) {
	secretInformer, err := seedClientSet.Cache().GetInformer(ctx, &corev1.Secret{})
	if err != nil {
		return nil, fmt.Errorf("failed to get Secret Informer: %w", err)
	}

	controller := &Controller{
		reconciler:     NewReconciler(gardenClient, seedClientSet.Client(), log.WithField("controller", ControllerName)),
		secretQueue:    workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Secret"),
		workerCh:       make(chan int),
		hasSyncedFuncs: []cache.InformerSynced{secretInformer.HasSynced},
	}

	secretInformer.AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: func(obj interface{}) bool {
			secret, ok := obj.(*corev1.Secret)
			return ok && LabelsPredicate(secret.Labels)
		},
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    controller.secretAdd,
			UpdateFunc: controller.secretUpdate,
			DeleteFunc: controller.secretDelete,
		},
	})

	return controller, nil
}

// Run runs the Controller until the given stop channel can be read from.
func (c *Controller) Run(ctx context.Context, workers int) {
	var waitGroup sync.WaitGroup

	if !cache.WaitForCacheSync(ctx.Done(), c.hasSyncedFuncs...) {
		logger.Logger.Error("Timed out waiting for caches to sync")
		return
	}

	// Count number of running workers.
	go func() {
		for res := range c.workerCh {
			c.numberOfRunningWorkers += res
			logger.Logger.Debugf("Current number of running Secret workers is %d", c.numberOfRunningWorkers)
		}
	}()

	logger.Logger.Info("Secret controller initialized.")

	for i := 0; i < workers; i++ {
		controllerutils.CreateWorker(ctx, c.secretQueue, "secret", c.reconciler, &waitGroup, c.workerCh)
	}

	// Shutdown handling
	<-ctx.Done()
	c.secretQueue.ShutDown()

	for {
		if c.secretQueue.Len() == 0 && c.numberOfRunningWorkers == 0 {
			logger.Logger.Info("No running Secret worker and no items left in the queues. Terminated Secret controller...")
			break
		}
		logger.Logger.Infof("Waiting for %d Secret worker(s) to finish (%d item(s) left in the queues)...", c.numberOfRunningWorkers, c.secretQueue.Len())
		time.Sleep(5 * time.Second)
	}

	waitGroup.Wait()
}

func (c *Controller) secretAdd(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		logger.Logger.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}

	c.secretQueue.Add(key)
}

func (c *Controller) secretUpdate(_, newObj interface{}) {
	c.secretAdd(newObj)
}

func (c *Controller) secretDelete(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		logger.Logger.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}
	c.secretQueue.Add(key)
}

// LabelsPredicate is a function which returns true when the provided labels map suggests that the object is managed by
// the secrets manager and should be persisted.
func LabelsPredicate(labels map[string]string) bool {
	return labels[secretsmanager.LabelKeyManagedBy] == secretsmanager.LabelValueSecretsManager &&
		labels[secretsmanager.LabelKeyPersist] == secretsmanager.LabelValueTrue
}

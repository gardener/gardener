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

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/utils/clock"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	"github.com/gardener/gardener/pkg/controllerutils"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// ControllerName is the name of this controller.
const ControllerName = "seed"

// Controller controls Seeds.
type Controller struct {
	gardenClient client.Client
	config       *config.ControllerManagerConfiguration
	log          logr.Logger

	secretsReconciler                reconcile.Reconciler
	seedBackupBucketsCheckReconciler reconcile.Reconciler
	lifeCycleReconciler              reconcile.Reconciler
	extensionsCheckReconciler        reconcile.Reconciler

	secretsQueue                workqueue.RateLimitingInterface
	seedBackupBucketsCheckQueue workqueue.RateLimitingInterface
	seedLifecycleQueue          workqueue.RateLimitingInterface
	seedExtensionsCheckQueue    workqueue.RateLimitingInterface

	hasSyncedFuncs         []cache.InformerSynced
	workerCh               chan int
	numberOfRunningWorkers int
}

// NewSeedController takes a Kubernetes client for the Garden clusters <k8sGardenClient>, a struct
// holding information about the acting Gardener, a <gardenInformerFactory>, and a <recorder> for
// event recording. It creates a new Seed controller.
func NewSeedController(
	ctx context.Context,
	log logr.Logger,
	mgr manager.Manager,
	config *config.ControllerManagerConfiguration,
) (
	*Controller,
	error,
) {
	log = log.WithName(ControllerName)

	gardenClient := mgr.GetClient()
	gardenCache := mgr.GetCache()

	backupBucketInformer, err := gardenCache.GetInformer(ctx, &gardencorev1beta1.BackupBucket{})
	if err != nil {
		return nil, fmt.Errorf("failed to get BackupBucket Informer: %w", err)
	}
	secretInformer, err := gardenCache.GetInformer(ctx, &corev1.Secret{})
	if err != nil {
		return nil, fmt.Errorf("failed to get Secret Informer: %w", err)
	}
	seedInformer, err := gardenCache.GetInformer(ctx, &gardencorev1beta1.Seed{})
	if err != nil {
		return nil, fmt.Errorf("failed to get Seed Informer: %w", err)
	}
	controllerInstallationInformer, err := gardenCache.GetInformer(ctx, &gardencorev1beta1.ControllerInstallation{})
	if err != nil {
		return nil, fmt.Errorf("failed to get ControllerInstallation Informer: %w", err)
	}

	seedController := &Controller{
		gardenClient: gardenClient,
		config:       config,
		log:          log,

		secretsReconciler:                NewSecretsReconciler(gardenClient),
		lifeCycleReconciler:              NewLifecycleReconciler(gardenClient, config),
		seedBackupBucketsCheckReconciler: NewBackupBucketsCheckReconciler(gardenClient, *config.Controllers.SeedBackupBucketsCheck, clock.RealClock{}),
		extensionsCheckReconciler:        NewExtensionsCheckReconciler(gardenClient, *config.Controllers.SeedExtensionsCheck, clock.RealClock{}),

		secretsQueue:                workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Seed Secrets"),
		seedBackupBucketsCheckQueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Backup Buckets Check"),
		seedLifecycleQueue:          workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Seed Lifecycle"),
		seedExtensionsCheckQueue:    workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Extensions Check"),
		workerCh:                    make(chan int),
	}

	backupBucketInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    seedController.backupBucketAdd,
		UpdateFunc: seedController.backupBucketUpdate,
	})

	seedInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: seedController.seedLifecycleAdd,
	})

	seedInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: seedController.seedAdd,
	})

	controllerInstallationInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    seedController.controllerInstallationOfSeedAdd,
		UpdateFunc: seedController.controllerInstallationOfSeedUpdate,
		DeleteFunc: seedController.controllerInstallationOfSeedDelete,
	})

	secretInformer.AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: filterGardenSecret,
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc:    func(obj interface{}) { seedController.gardenSecretAdd(ctx, obj) },
			UpdateFunc: func(oldObj, newObj interface{}) { seedController.gardenSecretUpdate(ctx, oldObj, newObj) },
			DeleteFunc: func(obj interface{}) { seedController.gardenSecretDelete(ctx, obj) },
		},
	})

	seedController.hasSyncedFuncs = []cache.InformerSynced{
		backupBucketInformer.HasSynced,
		seedInformer.HasSynced,
		secretInformer.HasSynced,
	}

	return seedController, nil
}

// Run runs the Controller until the given stop channel can be read from.
func (c *Controller) Run(ctx context.Context, seedWorkers, seedBackupBucketsCheckWorkers, seedExtensionsCheckWorkers int) {
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

	var waitGroup sync.WaitGroup
	for i := 0; i < seedWorkers; i++ {
		controllerutils.CreateWorker(ctx, c.secretsQueue, "Seed Secrets", c.secretsReconciler, &waitGroup, c.workerCh, controllerutils.WithLogger(c.log.WithName(seedSecretsReconcilerName)))
		controllerutils.CreateWorker(ctx, c.seedLifecycleQueue, "Seed Lifecycle", c.lifeCycleReconciler, &waitGroup, c.workerCh, controllerutils.WithLogger(c.log.WithName(seedLifecycleReconcilerName)))
	}
	for i := 0; i < seedBackupBucketsCheckWorkers; i++ {
		controllerutils.CreateWorker(ctx, c.seedBackupBucketsCheckQueue, "Seed Backup Bucket Check", c.seedBackupBucketsCheckReconciler, &waitGroup, c.workerCh, controllerutils.WithLogger(c.log.WithName(seedBackupBucketsCheckReconcilerName)))
	}
	for i := 0; i < seedExtensionsCheckWorkers; i++ {
		controllerutils.CreateWorker(ctx, c.seedExtensionsCheckQueue, "Seed Extension Check", c.extensionsCheckReconciler, &waitGroup, c.workerCh, controllerutils.WithLogger(c.log.WithName(extensionCheckReconcilerName)))
	}

	// Shutdown handling
	<-ctx.Done()
	c.secretsQueue.ShutDown()
	c.seedBackupBucketsCheckQueue.ShutDown()
	c.seedLifecycleQueue.ShutDown()
	c.seedExtensionsCheckQueue.ShutDown()

	for {
		queueLength := c.secretsQueue.Len() + c.seedBackupBucketsCheckQueue.Len() + c.seedLifecycleQueue.Len() + c.seedExtensionsCheckQueue.Len()
		if queueLength == 0 && c.numberOfRunningWorkers == 0 {
			c.log.V(1).Info("No running Seed worker and no items left in the queues. Terminating Seed controller")
			break
		}
		c.log.V(1).Info("Waiting for Seed workers to finish", "numberOfRunningWorkers", c.numberOfRunningWorkers, "queueLength", queueLength)
		time.Sleep(5 * time.Second)
	}

	waitGroup.Wait()
}

func reconcileAfter(d time.Duration) (reconcile.Result, error) {
	return reconcile.Result{RequeueAfter: d}, nil
}

func reconcileResult(err error) (reconcile.Result, error) {
	return reconcile.Result{}, err
}

func setToProgressingOrUnknown(
	clock clock.Clock,
	conditionThreshold time.Duration,
	condition gardencorev1beta1.Condition,
	reason, message string,
	codes ...gardencorev1beta1.ErrorCode,
) gardencorev1beta1.Condition {
	return setToProgressingIfWithinThreshold(clock, conditionThreshold, condition, gardencorev1beta1.ConditionUnknown, reason, message, codes...)
}

func setToProgressingOrFalse(
	clock clock.Clock,
	conditionThreshold time.Duration,
	condition gardencorev1beta1.Condition,
	reason, message string,
	codes ...gardencorev1beta1.ErrorCode,
) gardencorev1beta1.Condition {
	return setToProgressingIfWithinThreshold(clock, conditionThreshold, condition, gardencorev1beta1.ConditionFalse, reason, message, codes...)
}

func setToProgressingIfWithinThreshold(
	clock clock.Clock,
	conditionThreshold time.Duration,
	condition gardencorev1beta1.Condition,
	eventualConditionStatus gardencorev1beta1.ConditionStatus,
	reason, message string,
	codes ...gardencorev1beta1.ErrorCode,
) gardencorev1beta1.Condition {
	switch condition.Status {
	case gardencorev1beta1.ConditionTrue:
		if conditionThreshold == 0 {
			return gardencorev1beta1helper.UpdatedCondition(condition, eventualConditionStatus, reason, message, codes...)
		}
		return gardencorev1beta1helper.UpdatedCondition(condition, gardencorev1beta1.ConditionProgressing, reason, message, codes...)

	case gardencorev1beta1.ConditionProgressing:
		if conditionThreshold == 0 {
			return gardencorev1beta1helper.UpdatedCondition(condition, eventualConditionStatus, reason, message, codes...)
		}

		if delta := clock.Now().UTC().Sub(condition.LastTransitionTime.Time.UTC()); delta <= conditionThreshold {
			return gardencorev1beta1helper.UpdatedCondition(condition, gardencorev1beta1.ConditionProgressing, reason, message, codes...)
		}
		return gardencorev1beta1helper.UpdatedCondition(condition, eventualConditionStatus, reason, message, codes...)
	}

	return gardencorev1beta1helper.UpdatedCondition(condition, eventualConditionStatus, reason, message, codes...)
}

func getThresholdForCondition(conditions []config.ConditionThreshold, conditionType gardencorev1beta1.ConditionType) time.Duration {
	for _, threshold := range conditions {
		if threshold.Type == string(conditionType) {
			return threshold.Duration.Duration
		}
	}
	return 0
}

func patchSeedCondition(ctx context.Context, c client.StatusClient, seed *gardencorev1beta1.Seed, condition gardencorev1beta1.Condition) error {
	patch := client.StrategicMergeFrom(seed.DeepCopy())

	conditions := gardencorev1beta1helper.MergeConditions(seed.Status.Conditions, condition)
	if !gardencorev1beta1helper.ConditionsNeedUpdate(seed.Status.Conditions, conditions) {
		return nil
	}

	seed.Status.Conditions = conditions
	return c.Status().Patch(ctx, seed, patch)
}

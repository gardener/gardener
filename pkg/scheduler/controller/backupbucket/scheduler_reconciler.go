// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package backupbucket

import (
	"context"
	"fmt"

	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/scheduler/controller/common"
	"github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardencorev1alpha1helper "github.com/gardener/gardener/pkg/apis/core/v1alpha1/helper"
	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	gardenhelper "github.com/gardener/gardener/pkg/apis/garden/v1beta1/helper"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// MsgUnschedulable is the Message for the Event on a BackupBucket that the Scheduler creates in case it cannot schedule the BackupBucket to any Seed
const MsgUnschedulable = "Failed to schedule backupbucket"

func (c *SchedulerController) backupBucketAdd(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		logger.Logger.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}

	newBackupBucket := obj.(*gardencorev1alpha1.BackupBucket)

	if newBackupBucket.DeletionTimestamp != nil {
		logger.Logger.Infof("Ignoring backupBucket '%s' because it has been marked for deletion", newBackupBucket.Name)
		c.backupBucketQueue.Forget(key)
		return
	}

	c.backupBucketQueue.Add(key)
}

func (c *SchedulerController) backupBucketUpdate(oldObj, newObj interface{}) {
	c.backupBucketAdd(newObj)
}

// reconciler implements the reconcile.Reconcile interface for backupBucket scheduler.
type reconciler struct {
	ctx      context.Context
	client   client.Client
	recorder record.EventRecorder
	logger   *logrus.Entry
}

// newReconciler returns the new backupBucker reconciler.
func newReconciler(ctx context.Context, gardenClient client.Client, recorder record.EventRecorder) reconcile.Reconciler {
	return &reconciler{
		ctx:      ctx,
		client:   gardenClient,
		recorder: recorder,
		logger:   logger.NewFieldLogger(logger.Logger, "scheduler", "backupbucket"),
	}
}
func (r *reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	bb := &gardencorev1alpha1.BackupBucket{}
	if err := r.client.Get(r.ctx, request.NamespacedName, bb); err != nil {
		if apierrors.IsNotFound(err) {
			r.logger.Debugf("[SCHEDULER BACKUPBUCKET RECONCILE] %s - skipping because BackupBucket has been deleted", request.NamespacedName)
			return reconcile.Result{}, nil
		}
		r.logger.Infof("[SCHEDULER BACKUPBUCKET RECONCILE] %s - unable to retrieve object from store: %v", request.NamespacedName, err)
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, r.scheduleBackupBucket(bb)
}

type executeSchedulingRequest = func(context.Context, *gardencorev1alpha1.BackupBucket) error

func (r *reconciler) scheduleBackupBucket(obj *gardencorev1alpha1.BackupBucket) error {
	var (
		backupBucket    = obj.DeepCopy()
		schedulerLogger = r.logger.WithField("backupbucket", backupBucket.Name)
	)

	if backupBucket.Spec.Seed != nil {
		// If the BackupBucket manifest already specifies a desired Seed cluster,
		// we should check its availability. If its not available we will try to reschedule it again.
		schedulerLogger.Infof("BackupBucket is already scheduled on seed %s", *backupBucket.Spec.Seed)
		seed := &gardenv1beta1.Seed{}
		if err := r.client.Get(r.ctx, kutil.Key(*backupBucket.Spec.Seed), seed); err != nil {
			return err
		}

		if cond := gardencorev1alpha1helper.GetCondition(seed.Status.Conditions, gardenv1beta1.SeedAvailable); cond != nil && cond.Status != gardencorev1alpha1.ConditionUnknown {
			schedulerLogger.Infof("Seed %s is available, ignoring further reconciliation.", *backupBucket.Spec.Seed)
			return nil
		}
		schedulerLogger.Infof("Seed %s is not available, we will schedule it on another seed", *backupBucket.Spec.Seed)
	}
	// If no Seed is referenced, we try to determine an adequate one.
	seed, err := r.determineSeed(backupBucket)
	if err != nil {
		r.reportFailedScheduling(backupBucket, err)
		return err
	}

	if err := r.updateBackupBucketToBeScheduledOntoSeed(backupBucket, seed.Name); err != nil {
		if _, ok := err.(*common.AlreadyScheduledError); ok {
			return nil
		}
		r.reportFailedScheduling(backupBucket, err)
		return err
	}

	schedulerLogger.Infof("BackupBucket '%s' (Cloud Provider '%s', Region '%s') successfully scheduled to seed '%s' ", backupBucket.Name, backupBucket.Spec.Provider.Type, backupBucket.Spec.Provider.Region, seed.Name)
	r.reportSuccessfulScheduling(backupBucket, seed.Name)
	return nil
}

// determineSeed finds the appropriate seed for backupBucket.
// It finds the seed by filtering out list as per policy mentioned below:
// 1. Filter out seeds marked for deletion
// 2. Filter out seeds which are not available and ready currently.
// 3. Select a seed if both, it's cloud provider and region matches with backupBucket.
// 4. If failed find seed in step 3, then select a seed with matching cloud provider.
// 5. If still not found then, select any of remaining seed.
// 6. Return error if none of the above step found seed.
func (r *reconciler) determineSeed(backupBucket *gardencorev1alpha1.BackupBucket) (*gardenv1beta1.Seed, error) {
	var (
		candidatesWithMathingProvider    = make([]*gardenv1beta1.Seed, 0)
		candidatesWithoutMathingProvider = make([]*gardenv1beta1.Seed, 0)
		cloudProfileCache                = make(map[string]gardenv1beta1.CloudProvider) //cloudProfile name verses cloudProvider map
	)

	seeds := &gardenv1beta1.SeedList{}
	if err := r.client.List(r.ctx, seeds); err != nil {
		return nil, err
	}

	if len(seeds.Items) == 0 {
		return nil, fmt.Errorf("no seed found for scheduling")
	}
	for _, seed := range seeds.Items {
		if seed.DeletionTimestamp != nil || !verifySeedAvailability(&seed) {
			continue
		}

		// Post GEP-4 following logic will be simplified as commented.
		// provider := seed.Spec.Provider.Type
		provider, ok := cloudProfileCache[seed.Spec.Cloud.Profile]
		if !ok {
			cloudProfile := &gardenv1beta1.CloudProfile{}
			if err := r.client.Get(r.ctx, kutil.Key(seed.Spec.Cloud.Profile), cloudProfile); err != nil {
				return nil, err
			}

			provider, err := gardenhelper.DetermineCloudProviderInProfile(cloudProfile.Spec)
			if err != nil {
				return nil, err
			}
			cloudProfileCache[cloudProfile.Name] = provider
		}
		if backupBucket.Spec.Provider.Type == string(provider) {
			if seed.Spec.Cloud.Region == backupBucket.Spec.Provider.Region {
				return &seed, nil
			}
			candidatesWithMathingProvider = append(candidatesWithMathingProvider, &seed)
		}
		candidatesWithoutMathingProvider = append(candidatesWithoutMathingProvider, &seed)
	}

	if len(candidatesWithMathingProvider) != 0 {
		return candidatesWithMathingProvider[0], nil
	}

	if len(candidatesWithoutMathingProvider) != 0 {
		return candidatesWithoutMathingProvider[0], nil
	}
	return nil, fmt.Errorf("failed to find valid seed for scheduling")
}

func verifySeedAvailability(seed *gardenv1beta1.Seed) bool {
	if cond := gardencorev1alpha1helper.GetCondition(seed.Status.Conditions, gardenv1beta1.SeedAvailable); cond != nil {
		return cond.Status == gardencorev1alpha1.ConditionTrue
	}
	return false
}

// updateBackupBucketToBeScheduledOntoSeed sets the seed name where the backupBucket should be scheduled on. Then it executes the actual update call to the API server. The call is capsuled to allow for easier testing.
func (r *reconciler) updateBackupBucketToBeScheduledOntoSeed(backupBucket *gardencorev1alpha1.BackupBucket, seedName string) error {
	return kutil.TryUpdate(r.ctx, retry.DefaultBackoff, r.client, backupBucket, func() error {
		if backupBucket.Spec.Seed != nil {
			alreadyScheduledErr := common.NewAlreadyScheduledError(fmt.Sprintf("backupBucket has already a seed assigned when trying to schedule the backupBucket to %s", *backupBucket.Spec.Seed))
			return &alreadyScheduledErr
		}
		backupBucket.Spec.Seed = &seedName
		return nil
	})
}

func (r *reconciler) reportFailedScheduling(backupBucket *gardencorev1alpha1.BackupBucket, err error) {
	r.reportEvent(backupBucket, corev1.EventTypeWarning, gardencorev1alpha1.EventSchedulingFailed, MsgUnschedulable+" '%s' : %+v", backupBucket.Name, err)
}

func (r *reconciler) reportSuccessfulScheduling(backupBucket *gardencorev1alpha1.BackupBucket, seedName string) {
	r.reportEvent(backupBucket, corev1.EventTypeNormal, gardencorev1alpha1.EventSchedulingSuccessful, "Scheduled to seed '%s'", seedName)
}

func (r *reconciler) reportEvent(obj *gardencorev1alpha1.BackupBucket, eventType, eventReason, messageFmt string, args ...interface{}) {
	r.recorder.Eventf(obj, eventType, eventReason, messageFmt, args...)
}

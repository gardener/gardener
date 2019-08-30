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

package controllerregistration

import (
	"context"
	"fmt"
	"time"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardencorev1alpha1helper "github.com/gardener/gardener/pkg/apis/core/v1alpha1/helper"
	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	gardencorelisters "github.com/gardener/gardener/pkg/client/core/listers/core/v1alpha1"
	gardeninformers "github.com/gardener/gardener/pkg/client/garden/informers/externalversions"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	controllerutils "github.com/gardener/gardener/pkg/controllermanager/controller/utils"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/operation/common"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/retry"
	"github.com/sirupsen/logrus"

	multierror "github.com/hashicorp/go-multierror"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (c *Controller) seedAdd(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		logger.Logger.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}
	c.seedQueue.Add(key)
}

func (c *Controller) seedUpdate(oldObj, newObj interface{}) {
	c.seedAdd(newObj)
}

func (c *Controller) seedDelete(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		logger.Logger.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}
	c.seedQueue.Add(key)
}

func (c *Controller) reconcileSeedKey(key string) error {
	_, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	seed, err := c.seedLister.Get(name)
	if apierrors.IsNotFound(err) {
		logger.Logger.Debugf("[CONTROLLERREGISTRATION SEED RECONCILE] %s - skipping because Seed has been deleted", key)
		return nil
	}
	if err != nil {
		logger.Logger.Infof("[CONTROLLERREGISTRATION SEED RECONCILE] %s - unable to retrieve object from store: %v", key, err)
		return err
	}

	if err := c.seedControl.Reconcile(seed); err != nil {
		return err
	}

	c.seedQueue.AddAfter(key, 30*time.Second)
	return nil
}

// SeedControlInterface implements the control logic for updating Seeds. It is implemented as an interface to allow
// for extensions that provide different semantics. Currently, there is only one implementation.
type SeedControlInterface interface {
	Reconcile(*gardenv1beta1.Seed) error
}

// NewDefaultSeedControl returns a new instance of the default implementation ControlInterface that
// implements the documented semantics for Seeds. You should use an instance returned from NewDefaultSeedControl()
// for any scenario other than testing.
func NewDefaultSeedControl(k8sGardenClient kubernetes.Interface, k8sGardenInformers gardeninformers.SharedInformerFactory, k8sGardenCoreInformers gardencoreinformers.SharedInformerFactory, recorder record.EventRecorder, config *config.ControllerManagerConfiguration, controllerRegistrationLister gardencorelisters.ControllerRegistrationLister, controllerInstallationLister gardencorelisters.ControllerInstallationLister, controllerRegistrationQueue workqueue.RateLimitingInterface) SeedControlInterface {
	return &defaultSeedControl{k8sGardenClient, k8sGardenInformers, k8sGardenCoreInformers, recorder, config, controllerRegistrationLister, controllerInstallationLister, controllerRegistrationQueue}
}

type defaultSeedControl struct {
	k8sGardenClient              kubernetes.Interface
	k8sGardenInformers           gardeninformers.SharedInformerFactory
	k8sGardenCoreInformers       gardencoreinformers.SharedInformerFactory
	recorder                     record.EventRecorder
	config                       *config.ControllerManagerConfiguration
	controllerRegistrationLister gardencorelisters.ControllerRegistrationLister
	controllerInstallationLister gardencorelisters.ControllerInstallationLister
	controllerRegistrationQueue  workqueue.RateLimitingInterface
}

func (c *defaultSeedControl) Reconcile(obj *gardenv1beta1.Seed) error {
	var (
		ctx    = context.TODO()
		seed   = obj.DeepCopy()
		logger = logger.NewFieldLogger(logger.Logger, "controllerregistration-seed", seed.Name)
		result error
	)

	controllerRegistrationList, err := c.controllerRegistrationLister.List(labels.Everything())
	if err != nil {
		return err
	}

	for _, controllerRegistration := range controllerRegistrationList {
		key, err := cache.MetaNamespaceKeyFunc(controllerRegistration)
		if err != nil {
			result = multierror.Append(result, err)
			continue
		}
		c.controllerRegistrationQueue.Add(key)
	}

	if result != nil {
		return result
	}

	if seed.DeletionTimestamp != nil {
		if seed.Spec.Backup != nil {
			if err := waitUntilBackupBucketDeleted(ctx, c.k8sGardenClient.Client(), seed, logger); err != nil {
				return err
			}
		}

		controllerInstallationList, err := c.controllerInstallationLister.List(labels.Everything())
		if err != nil {
			return err
		}

		for _, controllerInstallation := range controllerInstallationList {
			if controllerInstallation.Spec.SeedRef.Name == seed.Name {
				return fmt.Errorf("ControllerInstallations for seed %q still pending, cannot release seed", seed.Name)
			}
		}

		if err := controllerutils.RemoveFinalizer(ctx, c.k8sGardenClient.Client(), seed, FinalizerName); err != nil {
			logger.Errorf("Could not update the Seed specification: %s", err.Error())
			return err
		}
	}

	return nil
}

// waitUntilBackupBucketDeleted waits until backup bucket extension resource is deleted in gardener cluster.
func waitUntilBackupBucketDeleted(ctx context.Context, gardenClient client.Client, seed *gardenv1beta1.Seed, logger *logrus.Entry) error {
	var lastError *gardencorev1alpha1.LastError

	if err := retry.UntilTimeout(ctx, time.Second, 30*time.Second, func(ctx context.Context) (bool, error) {
		backupBucketName := string(seed.UID)
		bb := &gardencorev1alpha1.BackupBucket{}

		if err := gardenClient.Get(ctx, kutil.Key(backupBucketName), bb); err != nil {
			if apierrors.IsNotFound(err) {
				return retry.Ok()
			}
			return retry.SevereError(err)
		}

		if lastErr := bb.Status.LastError; lastErr != nil {
			logger.Errorf("BackupBucket did not get deleted yet, lastError is: %s", lastErr.Description)
			lastError = lastErr
		}

		logger.Infof("Waiting for backupBucket to be deleted...")
		return retry.MinorError(common.WrapWithLastError(fmt.Errorf("worker is still present"), lastError))
	}); err != nil {
		message := fmt.Sprintf("Error while waiting for backupBucket object to be deleted")
		if lastError != nil {
			return gardencorev1alpha1helper.DetermineError(fmt.Sprintf("%s: %s", message, lastError.Description))
		}
		return gardencorev1alpha1helper.DetermineError(fmt.Sprintf("%s: %s", message, err.Error()))
	}

	return nil
}

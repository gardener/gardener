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
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	"github.com/gardener/gardener/pkg/logger"
	seedpkg "github.com/gardener/gardener/pkg/operation/seed"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"
)

func (c *Controller) seedHeartbeatAdd(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		return
	}
	c.seedHeartbeatQueue.Add(key)
}

func (c *Controller) reconcileSeedHeartbeatKey(key string) error {
	_, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	seed, err := c.seedLister.Get(name)
	if apierrors.IsNotFound(err) {
		logger.Logger.Infof("[SEED HEARTBEAT] Stopping heart beat operations for Seed %s since it has been deleted", key)
		c.seedHeartbeatQueue.Done(key)
		return nil
	}
	if err != nil {
		logger.Logger.Infof("[SEED HEARTBEAT] %s - unable to retrieve object from store: %v", key, err)
		return err
	}

	if err := c.heartbeatControl.Beat(seed, key); err != nil {
		return err
	}

	c.seedHeartbeatQueue.AddAfter(key, 20*time.Second)
	return nil
}

// HeartbeatControlInterface implements the control logic for heart beats for Seeds. It is implemented as an interface to allow
// for extensions that provide different semantics. Currently, there is only one implementation.
type HeartbeatControlInterface interface {
	Beat(seed *gardencorev1beta1.Seed, key string) error
}

// NewDefaultHeartbeatControl returns a new instance of the default implementation HeartbeatControlInterface that
// implements the documented semantics for heartbeating for Seeds. You should use an instance returned from NewDefaultHeartbeatControl()
// for any scenario other than testing.
func NewDefaultHeartbeatControl(k8sGardenClient kubernetes.Interface, k8sGardenCoreInformers gardencoreinformers.Interface, identity *gardencorev1beta1.Gardener, config *config.GardenletConfiguration) HeartbeatControlInterface {
	return &defaultHeartbeatControl{k8sGardenClient, k8sGardenCoreInformers, identity, config}
}

type defaultHeartbeatControl struct {
	k8sGardenClient        kubernetes.Interface
	k8sGardenCoreInformers gardencoreinformers.Interface
	identity               *gardencorev1beta1.Gardener
	config                 *config.GardenletConfiguration
}

func (c *defaultHeartbeatControl) Beat(seedObj *gardencorev1beta1.Seed, key string) error {
	var (
		seed       = seedObj.DeepCopy()
		seedLogger = logger.NewFieldLogger(logger.Logger, "seed", seed.Name)
	)

	seedLogger.Debugf("[SEED HEARTBEAT] %s", key)

	// Initialize conditions based on the current status.
	var conditionGardenletReady = gardencorev1beta1helper.GetOrInitCondition(seed.Status.Conditions, gardencorev1beta1.SeedGardenletReady)

	if _, err := seedpkg.GetSeedClient(context.TODO(), c.k8sGardenClient.Client(), c.config.SeedClientConnection.ClientConnectionConfiguration, c.config.SeedSelector == nil, seed.Name); err != nil {
		// If this returns an error then there is a problem with the connectivity to the seed's api server.
		conditionGardenletReady = gardencorev1beta1helper.UpdatedCondition(conditionGardenletReady, gardencorev1beta1.ConditionFalse, "GardenletNotReady", fmt.Sprintf("error talking to the seed apiserver: %+v", err.Error()))
	} else {
		conditionGardenletReady = gardencorev1beta1helper.UpdatedCondition(conditionGardenletReady, gardencorev1beta1.ConditionTrue, "GardenletReady", "Gardenlet is posting ready status.")
	}

	// Update Seed status
	_, err := kutil.TryUpdateSeedConditions(c.k8sGardenClient.GardenCore(), retry.DefaultBackoff, seed.ObjectMeta,
		func(seed *gardencorev1beta1.Seed) (*gardencorev1beta1.Seed, error) {
			seed.Status.Conditions = gardencorev1beta1helper.MergeConditions(seed.Status.Conditions, conditionGardenletReady)
			return seed, nil
		},
	)
	return err
}

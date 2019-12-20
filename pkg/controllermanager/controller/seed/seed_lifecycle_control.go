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
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	"github.com/gardener/gardener/pkg/logger"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"
)

func (c *Controller) seedAdd(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
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
		logger.Logger.Infof("[SEED LIFECYCLE] Stopping lifecycle operations for Seed %s since it has been deleted", key)
		c.seedQueue.Done(key)
		return nil
	}
	if err != nil {
		logger.Logger.Infof("[SEED LIFECYCLE] %s - unable to retrieve object from store: %v", key, err)
		return err
	}

	if err := c.control.Reconcile(seed, key); err != nil {
		return err
	}

	c.seedQueue.AddAfter(key, 10*time.Second)
	return nil
}

// ControlInterface implements the control logic for managing the lifecycle for Seeds. It is implemented as an interface to allow
// for extensions that provide different semantics. Currently, there is only one implementation.
type ControlInterface interface {
	Reconcile(seed *gardencorev1beta1.Seed, key string) error
}

// NewDefaultControl returns a new instance of the default implementation ControlInterface that
// implements the documented semantics for checking the lifecycle for Seeds. You should use an instance returned from NewDefaultControl()
// for any scenario other than testing.
func NewDefaultControl(k8sGardenClient kubernetes.Interface, k8sGardenCoreInformers gardencoreinformers.Interface, config *config.ControllerManagerConfiguration) ControlInterface {
	return &defaultControl{k8sGardenClient, k8sGardenCoreInformers, config}
}

type defaultControl struct {
	k8sGardenClient        kubernetes.Interface
	k8sGardenCoreInformers gardencoreinformers.Interface
	config                 *config.ControllerManagerConfiguration
}

func (c *defaultControl) Reconcile(seedObj *gardencorev1beta1.Seed, key string) error {
	var (
		seed       = seedObj.DeepCopy()
		seedLogger = logger.NewFieldLogger(logger.Logger, "seed", seed.Name)
	)

	// New seeds don't have conditions, i.e., gardenlet never reported anything yet. Let's wait until it sends a heart beat.
	if len(seed.Status.Conditions) == 0 {
		return nil
	}

	for _, condition := range seed.Status.Conditions {
		if condition.Type == gardencorev1beta1.SeedGardenletReady && (condition.Status == gardencorev1beta1.ConditionUnknown || !condition.LastUpdateTime.UTC().Before(time.Now().UTC().Add(-c.config.Controllers.Seed.MonitorPeriod.Duration))) {
			return nil
		}
	}

	seedLogger.Infof("Setting status for seed %q to 'unknown' as gardenlet stopped reporting seed status.", seed.Name)

	conditionGardenletReady := gardencorev1beta1helper.GetOrInitCondition(seed.Status.Conditions, gardencorev1beta1.SeedGardenletReady)
	conditionGardenletReady = gardencorev1beta1helper.UpdatedCondition(conditionGardenletReady, gardencorev1beta1.ConditionUnknown, "SeedStatusUnknown", "Gardenlet stopped posting seed status.")

	// Update Seed status
	// We don't handle the error here as we don't want to run into the exponential backoff for the hearbeat.
	_, err := kutil.TryUpdateSeedConditions(c.k8sGardenClient.GardenCore(), retry.DefaultBackoff, seed.ObjectMeta,
		func(seed *gardencorev1beta1.Seed) (*gardencorev1beta1.Seed, error) {
			seed.Status.Conditions = gardencorev1beta1helper.MergeConditions(seed.Status.Conditions, conditionGardenletReady)
			return seed, nil
		},
	)
	return err
}

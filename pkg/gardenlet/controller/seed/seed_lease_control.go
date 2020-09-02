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

package seed

import (
	"context"
	"fmt"
	"net/http"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/logger"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

func (c *Controller) seedLeaseAdd(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		return
	}
	c.seedLeaseQueue.Add(key)
}

const (
	// LeaseResyncSeconds defines how often (in seconds) the seed lease is renewed.
	LeaseResyncSeconds = 2
	// LeaseResyncGracePeriodSeconds is the grace period for how long the lease may not be resynced before the health status
	// is changed to false.
	LeaseResyncGracePeriodSeconds = LeaseResyncSeconds * 10
)

func (c *Controller) reconcileSeedLeaseKey(key string) error {
	ctx := context.TODO()

	_, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	seed, err := c.seedLister.Get(name)
	if apierrors.IsNotFound(err) {
		logger.Logger.Infof("[SEED LEASE] Stopping lease operations for Seed %s since it has been deleted", key)

		if err := c.clientMap.InvalidateClient(keys.ForSeedWithName(name)); err != nil {
			return fmt.Errorf("failed to invalidate seed client: %w", err)
		}

		c.seedLeaseQueue.Done(key)
		return nil
	}
	if err != nil {
		logger.Logger.Infof("[SEED LEASE] unable to retrieve Seed object %s from store: %v", key, err)
		return err
	}

	if err := c.checkSeedConnection(ctx, seed); err != nil {
		c.lock.Lock()
		c.leaseMap[seed.Name] = false
		c.lock.Unlock()
		return fmt.Errorf("[SEED LEASE] cannot establish connection with Seed %s: %v", key, err)
	}

	var (
		seedCopy           = seed.DeepCopy()
		seedOwnerReference = buildSeedOwnerReference(seedCopy)
	)

	if err := c.seedLeaseControl.Sync(seedCopy.Name, *seedOwnerReference); err != nil {
		c.lock.Lock()
		c.leaseMap[seed.Name] = false
		c.lock.Unlock()
		return err
	}

	gardenClient, err := c.clientMap.GetClient(ctx, keys.ForGarden())
	if err != nil {
		return fmt.Errorf("failed to get garden client: %w", err)
	}

	c.lock.Lock()
	c.leaseMap[seed.Name] = true
	c.lock.Unlock()

	bldr, err := helper.NewConditionBuilder(gardencorev1beta1.SeedGardenletReady)
	if err != nil {
		return err
	}

	condition := helper.GetCondition(seedCopy.Status.Conditions, gardencorev1beta1.SeedGardenletReady)
	if condition != nil {
		bldr.WithOldCondition(*condition)
	}

	bldr.WithStatus(gardencorev1beta1.ConditionTrue)
	bldr.WithReason("GardenletReady")
	bldr.WithMessage("Gardenlet is posting ready status.")

	if newCondition, update := bldr.WithNowFunc(metav1.Now).Build(); update {
		seed.Status.Conditions = helper.MergeConditions(seedCopy.Status.Conditions, newCondition)
		if err := gardenClient.Client().Status().Update(context.TODO(), seed); err != nil {
			return err
		}
	}

	c.seedLeaseQueue.AddAfter(key, LeaseResyncSeconds*time.Second)
	return nil
}

func buildSeedOwnerReference(seed *gardencorev1beta1.Seed) *metav1.OwnerReference {
	return &metav1.OwnerReference{
		APIVersion: gardencorev1beta1.SchemeGroupVersion.WithKind("Seed").Version,
		Kind:       gardencorev1beta1.SchemeGroupVersion.WithKind("Seed").Kind,
		Name:       seed.GetName(),
		UID:        seed.GetUID(),
	}
}

func (c *Controller) checkSeedConnection(ctx context.Context, seed *gardencorev1beta1.Seed) error {
	client, err := c.clientMap.GetClient(ctx, keys.ForSeed(seed))
	if err != nil {
		return fmt.Errorf("failed to get seed client: %w", err)
	}

	result := client.RESTClient().Get().AbsPath("/healthz").Do(ctx)
	if result.Error() != nil {
		return fmt.Errorf("failed to execute call to Kubernetes API Server: %v", result.Error())
	}

	var statusCode int
	result.StatusCode(&statusCode)
	if statusCode != http.StatusOK {
		return fmt.Errorf("API Server returned unexpected status code: %d", statusCode)
	}

	return nil
}

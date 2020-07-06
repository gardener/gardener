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

package controllerregistration

import (
	"context"
	"fmt"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorelisters "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/logger"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

func (c *Controller) seedAdd(obj interface{}, addToControllerRegistrationQueue bool) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		return
	}

	c.seedQueue.Add(key)
	if addToControllerRegistrationQueue {
		c.controllerRegistrationSeedQueue.Add(key)
	}
}

func (c *Controller) seedUpdate(oldObj, newObj interface{}) {
	oldObject, ok := oldObj.(*gardencorev1beta1.Seed)
	if !ok {
		return
	}

	newObject, ok := newObj.(*gardencorev1beta1.Seed)
	if !ok {
		return
	}

	c.seedAdd(newObj, !apiequality.Semantic.DeepEqual(oldObject.Spec.DNS.Provider, newObject.Spec.DNS.Provider))
}

func (c *Controller) seedDelete(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		return
	}
	c.seedQueue.Add(key)
	c.controllerRegistrationSeedQueue.Add(key)
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

	return c.seedControl.Reconcile(seed)
}

// SeedControlInterface implements the control logic for updating Seeds. It is implemented as an interface to allow
// for extensions that provide different semantics. Currently, there is only one implementation.
type SeedControlInterface interface {
	Reconcile(*gardencorev1beta1.Seed) error
}

// NewDefaultSeedControl returns a new instance of the default implementation ControlInterface that
// implements the documented semantics for Seeds. You should use an instance returned from NewDefaultSeedControl()
// for any scenario other than testing.
func NewDefaultSeedControl(clientMap clientmap.ClientMap, controllerInstallationLister gardencorelisters.ControllerInstallationLister) SeedControlInterface {
	return &defaultSeedControl{clientMap, controllerInstallationLister}
}

type defaultSeedControl struct {
	clientMap                    clientmap.ClientMap
	controllerInstallationLister gardencorelisters.ControllerInstallationLister
}

func (c *defaultSeedControl) Reconcile(obj *gardencorev1beta1.Seed) error {
	var (
		ctx  = context.TODO()
		seed = obj.DeepCopy()
	)

	gardenClient, err := c.clientMap.GetClient(ctx, keys.ForGarden())
	if err != nil {
		return fmt.Errorf("failed to get garden client: %w", err)
	}

	if seed.DeletionTimestamp != nil {
		if !controllerutils.HasFinalizer(seed, FinalizerName) {
			return nil
		}

		controllerInstallationList, err := c.controllerInstallationLister.List(labels.Everything())
		if err != nil {
			return err
		}

		for _, controllerInstallation := range controllerInstallationList {
			if controllerInstallation.Spec.SeedRef.Name == seed.Name {
				return fmt.Errorf("cannot remove finalizer of Seed %q because still found at least one ControllerInstallation", seed.Name)
			}
		}

		return controllerutils.RemoveFinalizer(ctx, gardenClient.DirectClient(), seed, FinalizerName)
	}

	return controllerutils.EnsureFinalizer(ctx, gardenClient.Client(), seed, FinalizerName)
}

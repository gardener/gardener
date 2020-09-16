// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

func (c *Controller) seedAdd(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
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

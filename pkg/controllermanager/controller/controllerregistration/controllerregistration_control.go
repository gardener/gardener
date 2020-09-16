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

func (c *Controller) controllerRegistrationAdd(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		logger.Logger.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}
	c.controllerRegistrationQueue.Add(key)

	seedList, err := c.seedLister.List(labels.Everything())
	if err != nil {
		logger.Logger.Errorf("error listing seeds: %+v", err)
		return
	}

	for _, seed := range seedList {
		c.controllerRegistrationSeedQueue.Add(seed.Name)
	}
}

func (c *Controller) controllerRegistrationUpdate(oldObj, newObj interface{}) {
	c.controllerRegistrationAdd(newObj)
}

func (c *Controller) controllerRegistrationDelete(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		logger.Logger.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}
	c.controllerRegistrationQueue.Add(key)
}

func (c *Controller) reconcileControllerRegistrationKey(key string) error {
	_, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	controllerRegistration, err := c.controllerRegistrationLister.Get(name)
	if apierrors.IsNotFound(err) {
		logger.Logger.Debugf("[CONTROLLERREGISTRATION RECONCILE] %s - skipping because ControllerRegistration has been deleted", key)
		return nil
	}
	if err != nil {
		logger.Logger.Infof("[CONTROLLERREGISTRATION RECONCILE] %s - unable to retrieve object from store: %v", key, err)
		return err
	}

	return c.controllerRegistrationControl.Reconcile(controllerRegistration)
}

// ControlInterface implements the control logic for updating ControllerRegistrations. It is implemented as an interface to allow
// for extensions that provide different semantics. Currently, there is only one implementation.
type ControlInterface interface {
	Reconcile(*gardencorev1beta1.ControllerRegistration) error
}

// NewDefaultControllerRegistrationControl returns a new instance of the default implementation ControlInterface that
// implements the documented semantics for ControllerRegistrations. You should use an instance returned from NewDefaultControllerRegistrationControl()
// for any scenario other than testing.
func NewDefaultControllerRegistrationControl(clientMap clientmap.ClientMap, controllerInstallationLister gardencorelisters.ControllerInstallationLister) ControlInterface {
	return &defaultControllerRegistrationControl{clientMap, controllerInstallationLister}
}

type defaultControllerRegistrationControl struct {
	clientMap                    clientmap.ClientMap
	controllerInstallationLister gardencorelisters.ControllerInstallationLister
}

func (c *defaultControllerRegistrationControl) Reconcile(obj *gardencorev1beta1.ControllerRegistration) error {
	var (
		ctx                    = context.TODO()
		controllerRegistration = obj.DeepCopy()
	)

	gardenClient, err := c.clientMap.GetClient(ctx, keys.ForGarden())
	if err != nil {
		return fmt.Errorf("failed to get garden client: %w", err)
	}

	if controllerRegistration.DeletionTimestamp != nil {
		if !controllerutils.HasFinalizer(controllerRegistration, FinalizerName) {
			return nil
		}

		controllerInstallationList, err := c.controllerInstallationLister.List(labels.Everything())
		if err != nil {
			return err
		}

		for _, controllerInstallation := range controllerInstallationList {
			if controllerInstallation.Spec.RegistrationRef.Name == controllerRegistration.Name {
				return fmt.Errorf("cannot remove finalizer of ControllerRegistration %q because still found at least one ControllerInstallation", controllerRegistration.Name)
			}
		}

		return controllerutils.RemoveFinalizer(ctx, gardenClient.DirectClient(), controllerRegistration, FinalizerName)
	}

	return controllerutils.EnsureFinalizer(ctx, gardenClient.Client(), controllerRegistration, FinalizerName)
}

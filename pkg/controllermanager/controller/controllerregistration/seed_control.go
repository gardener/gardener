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
	"fmt"
	"time"

	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	gardencorelisters "github.com/gardener/gardener/pkg/client/core/listers/core/v1alpha1"
	gardeninformers "github.com/gardener/gardener/pkg/client/garden/informers/externalversions"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	"github.com/gardener/gardener/pkg/logger"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	multierror "github.com/hashicorp/go-multierror"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"k8s.io/client-go/util/workqueue"
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
// implements the documented semantics for Seeds. updater is the UpdaterInterface used
// to update the status of Seeds. You should use an instance returned from NewDefaultSeedControl() for any
// scenario other than testing.
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
		controllerInstallationList, err := c.controllerInstallationLister.List(labels.Everything())
		if err != nil {
			return err
		}

		for _, controllerInstallation := range controllerInstallationList {
			if controllerInstallation.Spec.SeedRef.Name == seed.Name {
				return fmt.Errorf("ControllerInstallations for seed %q still pending, cannot release seed", seed.Name)
			}
		}

		_, err = kutil.TryUpdateSeedWithEqualFunc(c.k8sGardenClient.Garden(), retry.DefaultBackoff, seed.ObjectMeta, func(s *gardenv1beta1.Seed) (*gardenv1beta1.Seed, error) {
			finalizers := sets.NewString(s.Finalizers...)
			finalizers.Delete(FinalizerName)
			s.Finalizers = finalizers.UnsortedList()
			return s, nil
		}, func(cur, updated *gardenv1beta1.Seed) bool {
			finalizers := sets.NewString(cur.Finalizers...)
			return !finalizers.Has(FinalizerName)
		})
		if err != nil {
			logger.Errorf("Could not update the Seed specification: %s", err.Error())
			return err
		}
	}

	return nil
}

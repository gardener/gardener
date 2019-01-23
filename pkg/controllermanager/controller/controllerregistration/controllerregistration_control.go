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

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	gardencorelisters "github.com/gardener/gardener/pkg/client/core/listers/core/v1alpha1"
	gardeninformers "github.com/gardener/gardener/pkg/client/garden/informers/externalversions"
	gardenlisters "github.com/gardener/gardener/pkg/client/garden/listers/garden/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	"github.com/gardener/gardener/pkg/logger"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	multierror "github.com/hashicorp/go-multierror"

	"github.com/sirupsen/logrus"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
)

func (c *Controller) controllerRegistrationAdd(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		logger.Logger.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}
	c.controllerRegistrationQueue.Add(key)
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

	if err := c.controllerRegistrationControl.Reconcile(controllerRegistration); err != nil {
		return err
	}

	c.controllerRegistrationQueue.AddAfter(key, 30*time.Second)
	return nil
}

// ControlInterface implements the control logic for updating ControllerRegistrations. It is implemented as an interface to allow
// for extensions that provide different semantics. Currently, there is only one implementation.
type ControlInterface interface {
	Reconcile(*gardencorev1alpha1.ControllerRegistration) error
}

// NewDefaultControllerRegistrationControl returns a new instance of the default implementation ControlInterface that
// implements the documented semantics for ControllerRegistrations. updater is the UpdaterInterface used
// to update the status of ControllerRegistrations. You should use an instance returned from NewDefaultControllerRegistrationControl() for any
// scenario other than testing.
func NewDefaultControllerRegistrationControl(k8sGardenClient kubernetes.Interface, k8sGardenInformers gardeninformers.SharedInformerFactory, k8sGardenCoreInformers gardencoreinformers.SharedInformerFactory, recorder record.EventRecorder, config *config.ControllerManagerConfiguration, seedLister gardenlisters.SeedLister, controllerRegistrationLister gardencorelisters.ControllerRegistrationLister, controllerInstallationLister gardencorelisters.ControllerInstallationLister) ControlInterface {
	return &defaultControllerRegistrationControl{k8sGardenClient, k8sGardenInformers, k8sGardenCoreInformers, recorder, config, seedLister, controllerRegistrationLister, controllerInstallationLister}
}

type defaultControllerRegistrationControl struct {
	k8sGardenClient              kubernetes.Interface
	k8sGardenInformers           gardeninformers.SharedInformerFactory
	k8sGardenCoreInformers       gardencoreinformers.SharedInformerFactory
	recorder                     record.EventRecorder
	config                       *config.ControllerManagerConfiguration
	seedLister                   gardenlisters.SeedLister
	controllerRegistrationLister gardencorelisters.ControllerRegistrationLister
	controllerInstallationLister gardencorelisters.ControllerInstallationLister
}

func (c *defaultControllerRegistrationControl) Reconcile(obj *gardencorev1alpha1.ControllerRegistration) error {
	var (
		controllerRegistration = obj.DeepCopy()
		logger                 = logger.NewFieldLogger(logger.Logger, "controllerregistration", controllerRegistration.Name)
	)

	if controllerRegistration.DeletionTimestamp != nil {
		return c.delete(controllerRegistration, logger)
	}

	return c.reconcile(controllerRegistration, logger)
}

func (c *defaultControllerRegistrationControl) reconcile(controllerRegistration *gardencorev1alpha1.ControllerRegistration, logger logrus.FieldLogger) error {
	var (
		err              error
		result           error
		installationsMap = map[string]string{}

		mustWriteFinalizer = false
	)

	seedList, err := c.seedLister.List(labels.Everything())
	if err != nil {
		return err
	}

	for _, seed := range seedList {
		if seed.DeletionTimestamp == nil {
			mustWriteFinalizer = true
		}
	}

	if mustWriteFinalizer {
		controllerRegistration, err = kutil.TryUpdateControllerRegistrationWithEqualFunc(c.k8sGardenClient.GardenCore(), retry.DefaultBackoff, controllerRegistration.ObjectMeta, func(c *gardencorev1alpha1.ControllerRegistration) (*gardencorev1alpha1.ControllerRegistration, error) {
			if finalizers := sets.NewString(c.Finalizers...); !finalizers.Has(FinalizerName) {
				finalizers.Insert(FinalizerName)
				c.Finalizers = finalizers.UnsortedList()
			}
			return c, nil
		}, func(cur, updated *gardencorev1alpha1.ControllerRegistration) bool {
			return sets.NewString(cur.Finalizers...).Has(FinalizerName)
		})
		if err != nil {
			return err
		}
	}

	// Live lookup to prevent working on a stale cache and trying to create multiple installations for the same
	// registration/seed combination.
	controllerInstallationList, err := c.k8sGardenClient.GardenCore().CoreV1alpha1().ControllerInstallations().List(metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, controllerInstallation := range controllerInstallationList.Items {
		if controllerInstallation.Spec.RegistrationRef.Name == controllerRegistration.Name {
			installationsMap[controllerInstallation.Spec.SeedRef.Name] = controllerInstallation.Name
		}
	}

	for _, seed := range seedList {
		if err := c.reconcileSeedInstallations(controllerRegistration, seed, installationsMap); err != nil {
			result = multierror.Append(result, err)
		}
	}

	return result
}

func (c *defaultControllerRegistrationControl) reconcileSeedInstallations(controllerRegistration *gardencorev1alpha1.ControllerRegistration, seed *gardenv1beta1.Seed, installationsMap map[string]string) error {
	if seed.DeletionTimestamp != nil {
		if installation, ok := installationsMap[seed.Name]; ok {
			if err := c.k8sGardenClient.GardenCore().CoreV1alpha1().ControllerInstallations().Delete(installation, &metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
				return err
			}
		}
		return nil
	}

	seed, err := kutil.TryUpdateSeedWithEqualFunc(c.k8sGardenClient.Garden(), retry.DefaultBackoff, seed.ObjectMeta, func(s *gardenv1beta1.Seed) (*gardenv1beta1.Seed, error) {
		if finalizers := sets.NewString(s.Finalizers...); !finalizers.Has(FinalizerName) {
			finalizers.Insert(FinalizerName)
			s.Finalizers = finalizers.UnsortedList()
		}
		return s, nil
	}, func(cur, updated *gardenv1beta1.Seed) bool {
		return sets.NewString(cur.Finalizers...).Has(FinalizerName)
	})
	if err != nil {
		return err
	}

	installationSpec := gardencorev1alpha1.ControllerInstallationSpec{
		SeedRef: corev1.ObjectReference{
			Name:            seed.Name,
			ResourceVersion: seed.ResourceVersion,
		},
		RegistrationRef: corev1.ObjectReference{
			Name:            controllerRegistration.Name,
			ResourceVersion: controllerRegistration.ResourceVersion,
		},
	}

	if name, ok := installationsMap[seed.Name]; ok {
		if _, err := kutil.CreateOrPatchControllerInstallation(c.k8sGardenClient.GardenCore(), metav1.ObjectMeta{Name: name}, func(controllerInstallation *gardencorev1alpha1.ControllerInstallation) *gardencorev1alpha1.ControllerInstallation {
			controllerInstallation.Spec = installationSpec
			return controllerInstallation
		}); err != nil {
			return err
		}

		return nil
	}

	controllerInstallation := &gardencorev1alpha1.ControllerInstallation{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", controllerRegistration.Name),
		},
		Spec: installationSpec,
	}

	_, err = c.k8sGardenClient.GardenCore().CoreV1alpha1().ControllerInstallations().Create(controllerInstallation)
	return err
}

func (c *defaultControllerRegistrationControl) delete(controllerRegistration *gardencorev1alpha1.ControllerRegistration, logger logrus.FieldLogger) error {
	var (
		result error
		count  int
	)

	controllerInstallationList, err := c.controllerInstallationLister.List(labels.Everything())
	if err != nil {
		return err
	}

	for _, controllerInstallation := range controllerInstallationList {
		if controllerInstallation.Spec.RegistrationRef.Name == controllerRegistration.Name {
			count++

			if err := c.k8sGardenClient.GardenCore().CoreV1alpha1().ControllerInstallations().Delete(controllerInstallation.Name, &metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
				result = multierror.Append(result, err)
			}
		}
	}

	if result != nil {
		return result
	}
	if count > 0 {
		return fmt.Errorf("deletion of installations is still pending")
	}

	_, err = kutil.TryUpdateControllerRegistrationWithEqualFunc(c.k8sGardenClient.GardenCore(), retry.DefaultBackoff, controllerRegistration.ObjectMeta, func(c *gardencorev1alpha1.ControllerRegistration) (*gardencorev1alpha1.ControllerRegistration, error) {
		finalizers := sets.NewString(c.Finalizers...)
		finalizers.Delete(FinalizerName)
		c.Finalizers = finalizers.UnsortedList()
		return c, nil
	}, func(cur, updated *gardencorev1alpha1.ControllerRegistration) bool {
		return !sets.NewString(cur.Finalizers...).Has(FinalizerName)
	})
	return err
}

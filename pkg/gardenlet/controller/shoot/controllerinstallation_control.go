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

package shoot

import (
	"strings"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/operation/common"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
)

func (c *Controller) controllerInstallationEnqueue(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		return
	}
	c.controllerInstallationQueue.Add(key)
}

func (c *Controller) controllerInstallationAdd(obj interface{}) {
	controllerInstallation, ok := obj.(*gardencorev1beta1.ControllerInstallation)
	if !ok {
		return
	}

	// We only want to add ControllerInstallations that we got an ADD event for because they have been newly created.
	// (We also get ADD events on controller restarts - here, we want to avoid adding previously existing ControllerInstallations).
	if controllerInstallation.Generation != 1 {
		return
	}

	c.controllerInstallationEnqueue(obj)
}

func (c *Controller) controllerInstallationUpdate(oldObj, newObj interface{}) {
	old, ok1 := oldObj.(*gardencorev1beta1.ControllerInstallation)
	new, ok2 := newObj.(*gardencorev1beta1.ControllerInstallation)

	if !ok1 || !ok2 {
		return
	}

	if specHashesChanged(old, new) {
		c.controllerInstallationEnqueue(newObj)
	}
}

func (c *Controller) reconcileControllerInstallationKey(key string) error {
	_, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}
	controllerInstallation, err := c.controllerInstallationLister.Get(name)
	if apierrors.IsNotFound(err) {
		logger.Logger.Debugf("[SHOOT CONTROLLERINSTALLATION] %s - skipping because ControllerInstallation has been deleted", key)
		return nil
	}
	if err != nil {
		logger.Logger.Errorf("[SHOOT CONTROLLERINSTALLATION] %s - unable to retrieve object from store: %v", key, err)
		return err
	}

	shootsRequiringEnqueueing, err := c.controllerInstallationControl.Reconcile(controllerInstallation)
	if err != nil {
		return err
	}

	for _, shoot := range shootsRequiringEnqueueing {
		key, err := cache.MetaNamespaceKeyFunc(shoot)
		if err != nil {
			return err
		}

		c.getShootQueue(shoot).Add(key)
		shootLogger := logger.NewShootLogger(logger.Logger, shoot.Name, shoot.Namespace)
		shootLogger.Infof("[SHOOT CONTROLLERINSTALLATION] Marked shoot for enqueueing because dependent extension installation %q was updated", controllerInstallation.Name)
	}

	return nil
}

// ControllerInstallationControlInterface implements the control logic for requeuing Shoots after extensions have been updated.
// It is implemented as an interface to allow for extensions that provide different semantics. Currently, there is only one
// implementation.
type ControllerInstallationControlInterface interface {
	Reconcile(controllerInstallationObj *gardencorev1beta1.ControllerInstallation) ([]*gardencorev1beta1.Shoot, error)
}

// NewDefaultControllerInstallationControl returns a new instance of the default implementation ControllerInstallationControlInterface that
// implements the documented semantics for maintaining Shoots. You should use an instance returned from
// NewDefaultControllerInstallationControl() for any scenario other than testing.
func NewDefaultControllerInstallationControl(clientMap clientmap.ClientMap, k8sGardenCoreInformers gardencoreinformers.Interface, recorder record.EventRecorder) ControllerInstallationControlInterface {
	return &defaultControllerInstallationControl{clientMap, k8sGardenCoreInformers, recorder}
}

type defaultControllerInstallationControl struct {
	clientMap              clientmap.ClientMap
	k8sGardenCoreInformers gardencoreinformers.Interface
	recorder               record.EventRecorder
}

func (c *defaultControllerInstallationControl) Reconcile(controllerInstallationObj *gardencorev1beta1.ControllerInstallation) ([]*gardencorev1beta1.Shoot, error) {
	controllerInstallation := controllerInstallationObj.DeepCopy()

	controllerRegistration, err := c.k8sGardenCoreInformers.ControllerRegistrations().Lister().Get(controllerInstallation.Spec.RegistrationRef.Name)
	if err != nil {
		return nil, err
	}

	resources := make(map[string]string, len(controllerRegistration.Spec.Resources))
	for _, resource := range controllerRegistration.Spec.Resources {
		resources[resource.Kind] = resource.Type
	}

	shootList, err := c.k8sGardenCoreInformers.Shoots().Lister().Shoots(corev1.NamespaceAll).List(labels.Everything())
	if err != nil {
		return nil, err
	}

	var shootsRequiringEnqueueing []*gardencorev1beta1.Shoot
	for _, shoot := range shootList {
		if seed := shoot.Spec.SeedName; seed == nil || *seed != controllerInstallation.Spec.SeedRef.Name {
			continue
		}
		if !c.isDependentOnResource(resources, shoot) {
			continue
		}

		shootsRequiringEnqueueing = append(shootsRequiringEnqueueing, shoot)
	}

	return shootsRequiringEnqueueing, nil
}

func (c *defaultControllerInstallationControl) isDependentOnResource(resources map[string]string, shoot *gardencorev1beta1.Shoot) bool {
	var machineImages []*gardencorev1beta1.ShootMachineImage
	for _, worker := range shoot.Spec.Provider.Workers {
		if worker.Machine.Image != nil {
			machineImages = append(machineImages, worker.Machine.Image)
		}
	}

	for resourceKind, resourceType := range resources {
		for _, machineImage := range machineImages {
			if machineImage == nil {
				continue
			}
			if resourceKind == extensionsv1alpha1.OperatingSystemConfigResource && strings.EqualFold(resourceType, string(machineImage.Name)) {
				return true
			}
		}
	}
	return false
}

func specHashesChanged(new, old *gardencorev1beta1.ControllerInstallation) bool {
	var (
		oldSeedHash, newSeedHash                 string
		oldRegistrationHash, newRegistrationHash string
	)

	if old.Labels != nil {
		oldSeedHash = old.Labels[common.SeedSpecHash]
		oldRegistrationHash = old.Labels[common.RegistrationSpecHash]
	}
	if new.Labels != nil {
		newSeedHash = new.Labels[common.SeedSpecHash]
		newRegistrationHash = new.Labels[common.RegistrationSpecHash]
	}

	return oldSeedHash != newSeedHash || oldRegistrationHash != newRegistrationHash
}

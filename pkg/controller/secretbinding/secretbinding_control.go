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

package secretbinding

import (
	"errors"
	"fmt"
	"time"

	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	gardeninformers "github.com/gardener/gardener/pkg/client/garden/informers/externalversions"
	gardenlisters "github.com/gardener/gardener/pkg/client/garden/listers/garden/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	controllerutils "github.com/gardener/gardener/pkg/controller/utils"
	"github.com/gardener/gardener/pkg/logger"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	kubecorev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
)

func (c *Controller) secretBindingAdd(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		logger.Logger.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}
	c.secretBindingQueue.Add(key)
}

func (c *Controller) secretBindingUpdate(oldObj, newObj interface{}) {
	c.secretBindingAdd(newObj)
}

func (c *Controller) secretBindingDelete(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		logger.Logger.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}
	c.secretBindingQueue.Add(key)
}

func (c *Controller) reconcileSecretBindingKey(key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	secretBinding, err := c.secretBindingLister.SecretBindings(namespace).Get(name)
	if apierrors.IsNotFound(err) {
		logger.Logger.Debugf("[SECRETBINDING RECONCILE] %s - skipping because SecretBinding has been deleted", key)
		return nil
	}
	if err != nil {
		logger.Logger.Infof("[SECRETBINDING RECONCILE] %s - unable to retrieve object from store: %v", key, err)
		return err
	}

	err = c.control.ReconcileSecretBinding(secretBinding, key)
	if err != nil {
		c.secretBindingQueue.AddAfter(key, time.Minute)
	}
	return nil
}

// ControlInterface implements the control logic for updating SecretBindings. It is implemented as an interface to allow
// for extensions that provide different semantics. Currently, there is only one implementation.
type ControlInterface interface {
	// ReconcileSecretBinding implements the control logic for SecretBinding creation, update, and deletion.
	// If an implementation returns a non-nil error, the invocation will be retried using a rate-limited strategy.
	// Implementors should sink any errors that they do not wish to trigger a retry, and they may feel free to
	// exit exceptionally at any point provided they wish the update to be re-run at a later point in time.
	ReconcileSecretBinding(secretBinding *gardenv1beta1.SecretBinding, key string) error
}

// NewDefaultControl returns a new instance of the default implementation ControlInterface that
// implements the documented semantics for SecretBindings. updater is the UpdaterInterface used
// to update the status of SecretBindings. You should use an instance returned from NewDefaultControl() for any
// scenario other than testing.
func NewDefaultControl(k8sGardenClient kubernetes.Client, k8sGardenInformers gardeninformers.SharedInformerFactory, recorder record.EventRecorder, secretLister kubecorev1listers.SecretLister, shootLister gardenlisters.ShootLister) ControlInterface {
	return &defaultControl{k8sGardenClient, k8sGardenInformers, recorder, secretLister, shootLister}
}

type defaultControl struct {
	k8sGardenClient    kubernetes.Client
	k8sGardenInformers gardeninformers.SharedInformerFactory
	recorder           record.EventRecorder
	secretLister       kubecorev1listers.SecretLister
	shootLister        gardenlisters.ShootLister
}

func (c *defaultControl) ReconcileSecretBinding(obj *gardenv1beta1.SecretBinding, key string) error {
	_, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		return err
	}

	var (
		secretBinding       = obj.DeepCopy()
		secretBindingLogger = logger.NewFieldLogger(logger.Logger, "secretbinding", fmt.Sprintf("%s/%s", secretBinding.Namespace, secretBinding.Name))
	)

	// The deletionTimestamp labels a SecretBinding as intended to get deleted. Before deletion,
	// it has to be ensured that no Shoots are depending on the SecretBinding anymore.
	// When this happens the controller will remove the finalizers from the SecretBinding so that it can be garbage collected.
	if secretBinding.DeletionTimestamp != nil {
		if !sets.NewString(secretBinding.Finalizers...).Has(gardenv1beta1.GardenerName) {
			return nil
		}

		associatedShoots, err := controllerutils.DetermineShootAssociations(secretBinding, c.shootLister)
		if err != nil {
			secretBindingLogger.Error(err.Error())
			return err
		}

		if len(associatedShoots) == 0 {
			secretBindingLogger.Info("No Shoots are referencing the SecretBinding. Deletion accepted.")

			// Remove finalizer from referenced secret
			secret, err := c.secretLister.Secrets(secretBinding.SecretRef.Namespace).Get(secretBinding.SecretRef.Name)
			if err == nil {
				secretFinalizers := sets.NewString(secret.Finalizers...)
				secretFinalizers.Delete(gardenv1beta1.ExternalGardenerName)
				secret.Finalizers = secretFinalizers.UnsortedList()
				if _, err := c.k8sGardenClient.UpdateSecretObject(secret); err != nil && !apierrors.IsNotFound(err) {
					secretBindingLogger.Error(err.Error())
					return err
				}
			} else if !apierrors.IsNotFound(err) {
				secretBindingLogger.Error(err.Error())
				return err
			}

			// Remove finalizer from SecretBinding
			secretBindingFinalizers := sets.NewString(secretBinding.Finalizers...)
			secretBindingFinalizers.Delete(gardenv1beta1.GardenerName)
			secretBinding.Finalizers = secretBindingFinalizers.UnsortedList()
			if _, err := c.k8sGardenClient.GardenClientset().GardenV1beta1().SecretBindings(secretBinding.Namespace).Update(secretBinding); err != nil && !apierrors.IsNotFound(err) {
				secretBindingLogger.Error(err.Error())
				return err
			}
			return nil
		}
		secretBindingLogger.Infof("Can't delete SecretBinding, because the following Shoots are still referencing it: %v", associatedShoots)
		return errors.New("SecretBinding still has references")
	}

	// Add the Gardener finalizer to the referenced SecretBinding secret to protect it from deletion as long as
	// the SecretBinding resource does exist.
	secret, err := c.secretLister.Secrets(secretBinding.SecretRef.Namespace).Get(secretBinding.SecretRef.Name)
	if err != nil {
		secretBindingLogger.Error(err.Error())
		return err
	}
	secretFinalizers := sets.NewString(secret.Finalizers...)
	if !secretFinalizers.Has(gardenv1beta1.ExternalGardenerName) {
		secretFinalizers.Insert(gardenv1beta1.ExternalGardenerName)
	}
	secret.Finalizers = secretFinalizers.UnsortedList()
	if _, err := c.k8sGardenClient.UpdateSecretObject(secret); err != nil {
		secretBindingLogger.Error(err.Error())
		return err
	}

	return nil
}

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
	"context"
	"errors"
	"fmt"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	gardencorelisters "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/logger"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
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

	if err := c.control.ReconcileSecretBinding(secretBinding, key); err != nil {
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
	ReconcileSecretBinding(secretBinding *gardencorev1beta1.SecretBinding, key string) error
}

// NewDefaultControl returns a new instance of the default implementation ControlInterface that
// implements the documented semantics for SecretBindings. You should use an instance returned from NewDefaultControl()
// for any scenario other than testing.
func NewDefaultControl(clientMap clientmap.ClientMap, k8sGardenCoreInformers gardencoreinformers.SharedInformerFactory, recorder record.EventRecorder, secretBindingLister gardencorelisters.SecretBindingLister, secretLister kubecorev1listers.SecretLister, shootLister gardencorelisters.ShootLister) ControlInterface {
	return &defaultControl{clientMap, k8sGardenCoreInformers, recorder, secretBindingLister, secretLister, shootLister}
}

type defaultControl struct {
	clientMap              clientmap.ClientMap
	k8sGardenCoreInformers gardencoreinformers.SharedInformerFactory
	recorder               record.EventRecorder
	secretBindingLister    gardencorelisters.SecretBindingLister
	secretLister           kubecorev1listers.SecretLister
	shootLister            gardencorelisters.ShootLister
}

func (c *defaultControl) ReconcileSecretBinding(obj *gardencorev1beta1.SecretBinding, key string) error {
	_, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		return err
	}

	var (
		secretBinding       = obj.DeepCopy()
		secretBindingLogger = logger.NewFieldLogger(logger.Logger, "secretbinding", fmt.Sprintf("%s/%s", secretBinding.Namespace, secretBinding.Name))
		ctx                 = context.TODO()
	)

	gardenClient, err := c.clientMap.GetClient(ctx, keys.ForGarden())
	if err != nil {
		return fmt.Errorf("failed to get garden client: %w", err)
	}

	// The deletionTimestamp labels a SecretBinding as intended to get deleted. Before deletion,
	// it has to be ensured that no Shoots are depending on the SecretBinding anymore.
	// When this happens the controller will remove the finalizers from the SecretBinding so that it can be garbage collected.
	if secretBinding.DeletionTimestamp != nil {
		if !controllerutils.HasFinalizer(secretBinding, gardencorev1beta1.GardenerName) {
			return nil
		}

		associatedShoots, err := controllerutils.DetermineShootsAssociatedTo(secretBinding, c.shootLister)
		if err != nil {
			secretBindingLogger.Error(err.Error())
			return err
		}

		if len(associatedShoots) == 0 {
			secretBindingLogger.Info("No Shoots are referencing the SecretBinding. Deletion accepted.")

			mayReleaseSecret, err := c.mayReleaseSecret(secretBinding.Namespace, secretBinding.Name, secretBinding.SecretRef.Namespace, secretBinding.SecretRef.Name)
			if err != nil {
				secretBindingLogger.Error(err.Error())
				return err
			}

			if mayReleaseSecret {
				// Remove finalizer from referenced secret
				secret, err := c.secretLister.Secrets(secretBinding.SecretRef.Namespace).Get(secretBinding.SecretRef.Name)
				if err == nil {
					if err2 := controllerutils.RemoveFinalizer(ctx, gardenClient.Client(), secret.DeepCopy(), gardencorev1beta1.ExternalGardenerName); err2 != nil {
						secretBindingLogger.Error(err2.Error())
						return err2
					}
				} else if !apierrors.IsNotFound(err) {
					return err
				}
			}

			// Remove finalizer from SecretBinding
			if err := controllerutils.RemoveFinalizer(ctx, gardenClient.Client(), secretBinding, gardencorev1beta1.GardenerName); err != nil {
				secretBindingLogger.Error(err.Error())
				return err
			}

			return nil
		}

		message := fmt.Sprintf("Can't delete SecretBinding, because the following Shoots are still referencing it: %v", associatedShoots)
		secretBindingLogger.Infof(message)
		c.recorder.Event(secretBinding, corev1.EventTypeNormal, v1beta1constants.EventResourceReferenced, message)

		return errors.New("SecretBinding still has references")
	}

	if err := controllerutils.EnsureFinalizer(ctx, gardenClient.Client(), secretBinding, gardencorev1beta1.GardenerName); err != nil {
		secretBindingLogger.Errorf("Could not add finalizer to SecretBinding: %s", err.Error())
		return err
	}

	// Add the Gardener finalizer to the referenced SecretBinding secret to protect it from deletion as long as
	// the SecretBinding resource does exist.
	secret, err := c.secretLister.Secrets(secretBinding.SecretRef.Namespace).Get(secretBinding.SecretRef.Name)
	if err != nil {
		secretBindingLogger.Error(err.Error())
		return err
	}

	if err := controllerutils.EnsureFinalizer(ctx, gardenClient.Client(), secret.DeepCopy(), gardencorev1beta1.ExternalGardenerName); err != nil {
		secretBindingLogger.Errorf("Could not add finalizer to Secret referenced in SecretBinding: %s", err.Error())
		return err
	}

	return nil
}

// We may only release a secret if there is no other secretbinding that references it (maybe in a different namespace).
func (c *defaultControl) mayReleaseSecret(secretBindingNamespace, secretBindingName, secretNamespace, secretName string) (bool, error) {
	secretBindingList, err := c.secretBindingLister.List(labels.Everything())
	if err != nil {
		return false, err
	}

	for _, secretBinding := range secretBindingList {
		if secretBinding.Namespace == secretBindingNamespace && secretBinding.Name == secretBindingName {
			continue
		}
		if secretBinding.SecretRef.Namespace == secretNamespace && secretBinding.SecretRef.Name == secretName {
			return false, nil
		}
	}

	return true, nil
}

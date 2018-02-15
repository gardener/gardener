// Copyright 2018 The Gardener Authors.
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

package privatesecretbinding

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

func (c *Controller) privateSecretBindingAdd(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		logger.Logger.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}
	c.privateSecretBindingQueue.Add(key)
}

func (c *Controller) privateSecretBindingUpdate(oldObj, newObj interface{}) {
	c.privateSecretBindingAdd(newObj)
}

func (c *Controller) privateSecretBindingDelete(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		logger.Logger.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}
	c.privateSecretBindingQueue.Add(key)
}

func (c *Controller) reconcilePrivateSecretBindingKey(key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	privateSecretBinding, err := c.privateSecretBindingLister.PrivateSecretBindings(namespace).Get(name)
	if apierrors.IsNotFound(err) {
		logger.Logger.Debugf("[PRIVATESECRETBINDING RECONCILE] %s - skipping because PrivateSecretBinding has been deleted", key)
		return nil
	}
	if err != nil {
		logger.Logger.Infof("[PRIVATESECRETBINDING RECONCILE] %s - unable to retrieve object from store: %v", key, err)
		return err
	}

	err = c.control.ReconcilePrivateSecretBinding(privateSecretBinding, key)
	if err != nil {
		c.privateSecretBindingQueue.AddAfter(key, time.Minute)
	}
	return nil
}

// ControlInterface implements the control logic for updating PrivateSecretBindings. It is implemented as an interface to allow
// for extensions that provide different semantics. Currently, there is only one implementation.
type ControlInterface interface {
	// ReconcilePrivateSecretBinding implements the control logic for PrivateSecretBinding creation, update, and deletion.
	// If an implementation returns a non-nil error, the invocation will be retried using a rate-limited strategy.
	// Implementors should sink any errors that they do not wish to trigger a retry, and they may feel free to
	// exit exceptionally at any point provided they wish the update to be re-run at a later point in time.
	ReconcilePrivateSecretBinding(privateSecretBinding *gardenv1beta1.PrivateSecretBinding, key string) error
}

// NewDefaultControl returns a new instance of the default implementation ControlInterface that
// implements the documented semantics for PrivateSecretBindings. updater is the UpdaterInterface used
// to update the status of PrivateSecretBindings. You should use an instance returned from NewDefaultControl() for any
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

func (c *defaultControl) ReconcilePrivateSecretBinding(obj *gardenv1beta1.PrivateSecretBinding, key string) error {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		return err
	}

	var (
		privateSecretBinding       = obj.DeepCopy()
		privateSecretBindingLogger = logger.NewFieldLogger(logger.Logger, "privatesecretbinding", fmt.Sprintf("%s/%s", privateSecretBinding.Namespace, privateSecretBinding.Name))
	)

	// The deletionTimestamp labels a PrivateSecretBinding as intented to get deleted. Before deletion,
	// it has to be ensured that no Shoots are depending on the PrivateSecretBinding anymore.
	// When this happens the controller will remove the finalizers from the PrivateSecretBinding so that it can be garbage collected.
	if privateSecretBinding.DeletionTimestamp != nil {
		associatedShoots, err := controllerutils.DetermineShootAssociations(privateSecretBinding, c.shootLister)
		if err != nil {
			privateSecretBindingLogger.Error(err.Error())
			return err
		}

		if len(associatedShoots) == 0 {
			privateSecretBindingLogger.Info("No Shoots are referencing the PrivateSecretBinding. Deletion accepted.")

			// Remove finalizer from referenced secret
			secret, err := c.secretLister.Secrets(privateSecretBinding.Namespace).Get(privateSecretBinding.SecretRef.Name)
			if err != nil {
				privateSecretBindingLogger.Error(err.Error())
				return err
			}
			secretFinalizers := sets.NewString(secret.Finalizers...)
			secretFinalizers.Delete(gardenv1beta1.ExternalGardenerName)
			secret.Finalizers = secretFinalizers.UnsortedList()
			if _, err := c.k8sGardenClient.UpdateSecretObject(secret); err != nil {
				privateSecretBindingLogger.Error(err.Error())
				return err
			}

			// Remove finalizer from PrivateSecretBinding
			privateSecretBindingFinalizers := sets.NewString(privateSecretBinding.Finalizers...)
			privateSecretBindingFinalizers.Delete(gardenv1beta1.GardenerName)
			privateSecretBinding.Finalizers = privateSecretBindingFinalizers.UnsortedList()
			if _, err := c.k8sGardenClient.GardenClientset().GardenV1beta1().PrivateSecretBindings(privateSecretBinding.Namespace).Update(privateSecretBinding); err != nil {
				privateSecretBindingLogger.Error(err.Error())
				return err
			}
			return nil
		}
		privateSecretBindingLogger.Infof("Can't delete PrivateSecretBinding, because the following Shoots are still referencing it: %v", associatedShoots)
		return errors.New("PrivateSecretBinding still has references")
	}

	// Add the Gardener finalizer to the referenced PrivateSecretBinding secret to protect it from deletion as long as
	// the PrivateSecretBinding resource does exist.
	secret, err := c.secretLister.Secrets(privateSecretBinding.Namespace).Get(privateSecretBinding.SecretRef.Name)
	if err != nil {
		privateSecretBindingLogger.Error(err.Error())
		return err
	}
	secretFinalizers := sets.NewString(secret.Finalizers...)
	if !secretFinalizers.Has(gardenv1beta1.ExternalGardenerName) {
		secretFinalizers.Insert(gardenv1beta1.ExternalGardenerName)
	}
	secret.Finalizers = secretFinalizers.UnsortedList()
	if _, err := c.k8sGardenClient.UpdateSecretObject(secret); err != nil {
		privateSecretBindingLogger.Error(err.Error())
		return err
	}

	return nil
}

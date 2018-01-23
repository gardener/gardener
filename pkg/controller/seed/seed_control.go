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

package seed

import (
	"encoding/json"
	"fmt"
	"time"

	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/apis/garden/v1beta1/helper"
	gardeninformers "github.com/gardener/gardener/pkg/client/garden/informers/externalversions"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/logger"
	seedpkg "github.com/gardener/gardener/pkg/operation/seed"
	"github.com/gardener/gardener/pkg/utils/imagevector"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
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
	var (
		oldSeed       = oldObj.(*gardenv1beta1.Seed)
		newSeed       = newObj.(*gardenv1beta1.Seed)
		specChanged   = !apiequality.Semantic.DeepEqual(oldSeed.Spec, newSeed.Spec)
		statusChanged = !apiequality.Semantic.DeepEqual(oldSeed.Status, newSeed.Status)
	)

	if !specChanged && statusChanged {
		return
	}
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
		logger.Logger.Debugf("[SEED RECONCILE] %s - skipping because Seed has been deleted", key)
		return nil
	}
	if err != nil {
		logger.Logger.Infof("[SEED RECONCILE] %s - unable to retrieve object from store: %v", key, err)
		return err
	}

	err = c.control.ReconcileSeed(seed, key)
	if err != nil {
		c.seedQueue.AddAfter(key, 15*time.Second)
	}
	return nil
}

// ControlInterface implements the control logic for updating Seeds. It is implemented as an interface to allow
// for extensions that provide different semantics. Currently, there is only one implementation.
type ControlInterface interface {
	// ReconcileSeed implements the control logic for Seed creation, update, and deletion.
	// If an implementation returns a non-nil error, the invocation will be retried using a rate-limited strategy.
	// Implementors should sink any errors that they do not wish to trigger a retry, and they may feel free to
	// exit exceptionally at any point provided they wish the update to be re-run at a later point in time.
	ReconcileSeed(seed *gardenv1beta1.Seed, key string) error
}

// NewDefaultControl returns a new instance of the default implementation ControlInterface that
// implements the documented semantics for Seeds. updater is the UpdaterInterface used
// to update the status of Seeds. You should use an instance returned from NewDefaultControl() for any
// scenario other than testing.
func NewDefaultControl(k8sGardenClient kubernetes.Client, k8sGardenInformers gardeninformers.SharedInformerFactory, secrets map[string]*corev1.Secret, imageVector imagevector.ImageVector, recorder record.EventRecorder, updater UpdaterInterface) ControlInterface {
	return &defaultControl{k8sGardenClient, k8sGardenInformers, secrets, imageVector, recorder, updater}
}

type defaultControl struct {
	k8sGardenClient    kubernetes.Client
	k8sGardenInformers gardeninformers.SharedInformerFactory
	secrets            map[string]*corev1.Secret
	imageVector        imagevector.ImageVector
	recorder           record.EventRecorder
	updater            UpdaterInterface
}

func (c *defaultControl) ReconcileSeed(obj *gardenv1beta1.Seed, key string) error {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		return err
	}
	var (
		seed        = obj.DeepCopy()
		seedJSON, _ = json.Marshal(seed)
		seedLogger  = logger.NewSeedLogger(logger.Logger, seed.Name)
	)

	seedLogger.Infof("[SEED RECONCILE] %s", key)
	seedLogger.Debugf(string(seedJSON))

	// Initialize conditions based on the current status.
	newConditions := helper.NewConditions(seed.Status.Conditions, gardenv1beta1.SeedAvailable)
	conditionSeedAvailable := newConditions[0]

	seedObj, err := seedpkg.New(c.k8sGardenClient, c.k8sGardenInformers.Garden().V1beta1(), seed)
	if err != nil {
		message := fmt.Sprintf("Failed to create a Seed object (%s).", err.Error())
		conditionSeedAvailable = helper.ModifyCondition(conditionSeedAvailable, corev1.ConditionUnknown, gardenv1beta1.ConditionCheckError, message)
		seedLogger.Error(message)
		c.updateSeedStatus(seed, *conditionSeedAvailable)
		return nil
	}

	// Bootstrap the Seed cluster.
	if err := seedpkg.BootstrapCluster(seedObj, c.k8sGardenClient, c.secrets, c.imageVector); err != nil {
		conditionSeedAvailable = helper.ModifyCondition(conditionSeedAvailable, corev1.ConditionFalse, "BootstrappingFailed", err.Error())
		c.updateSeedStatus(seed, *conditionSeedAvailable)
		seedLogger.Error(err.Error())
		return nil
	}

	// Check whether the Kubernetes version of the Seed cluster fulfills the minimal requirements.
	if err := seedObj.CheckMinimumK8SVersion(); err != nil {
		conditionSeedAvailable = helper.ModifyCondition(conditionSeedAvailable, corev1.ConditionFalse, "K8SVersionTooOld", err.Error())
		c.updateSeedStatus(seed, *conditionSeedAvailable)
		seedLogger.Error(err.Error())
		return nil
	}
	conditionSeedAvailable = helper.ModifyCondition(conditionSeedAvailable, corev1.ConditionTrue, "Passed", "all checks passed")
	c.updateSeedStatus(seed, *conditionSeedAvailable)

	return nil
}

func (c *defaultControl) updateSeedStatus(seed *gardenv1beta1.Seed, conditions ...gardenv1beta1.Condition) error {
	if !helper.ConditionsNeedUpdate(seed.Status.Conditions, conditions) {
		return nil
	}

	seed.Status.Conditions = conditions

	_, err := c.updater.UpdateSeedStatus(seed)
	if err != nil {
		logger.Logger.Errorf("Could not update the Seed status: %+v", err)
	}

	return err
}

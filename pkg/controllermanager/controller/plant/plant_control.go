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

package plant

import (
	"context"
	"fmt"
	"sync"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/logger"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	kubecorev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// reconcilePlantForMatchingSecret checks if there is a plant resource that references this secret and then reconciles the plant again
func (c *Controller) reconcilePlantForMatchingSecret(obj interface{}) {
	secret, ok := obj.(*corev1.Secret)
	if !ok {
		logger.Logger.Errorf("Could not convert object %v into Secret", obj)
		return
	}

	plants, err := c.plantLister.List(labels.Everything())
	if err != nil {
		logger.Logger.Errorf("Couldn't list plants for updated secret %+v: %v", obj, err)
		return
	}

	for _, plant := range plants {
		if isPlantSecret(plant, kutil.Key(secret.Namespace, secret.Name)) {
			key, err := cache.MetaNamespaceKeyFunc(plant)
			if err != nil {
				logger.Logger.Errorf("Couldn't get key for plant %+v: %v", plant, err)
				return
			}
			logger.Logger.Infof("[PLANT RECONCILE] Reconciling Plant after secret change")
			c.plantQueue.Add(key)
			return
		}
	}
}

// plantSecretUpdate calls reconcilePlantForMatchingSecret with the updated secret
func (c *Controller) plantSecretUpdate(oldObj, newObj interface{}) {
	old, ok1 := oldObj.(*corev1.Secret)
	new, ok2 := newObj.(*corev1.Secret)
	if !ok1 || !ok2 {
		return
	}
	if old.ResourceVersion != new.ResourceVersion {
		c.reconcilePlantForMatchingSecret(newObj)
	}
}

// plantAdd adds the plant resource
func (c *Controller) plantAdd(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		logger.Logger.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}
	c.plantQueue.Add(key)
}

// plantUpdate updates the plant resource
func (c *Controller) plantUpdate(oldObj, newObj interface{}) {
	old, ok1 := oldObj.(*gardencorev1beta1.Plant)
	new, ok2 := newObj.(*gardencorev1beta1.Plant)
	if !ok1 || !ok2 {
		return
	}

	if new.ObjectMeta.Generation == old.ObjectMeta.Generation {
		return
	}

	c.plantAdd(newObj)
}

func (c *Controller) plantDelete(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		logger.Logger.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}
	c.plantQueue.Add(key)
}

func (c *Controller) reconcilePlantKey(ctx context.Context, key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	plant, err := c.plantLister.Plants(namespace).Get(name)
	if apierrors.IsNotFound(err) {
		logger.Logger.Debugf("[PLANT RECONCILE] %s - skipping because Plant has been deleted", key)
		return nil
	}
	if err != nil {
		logger.Logger.Errorf("[PLANT RECONCILE] %s - unable to retrieve object from store: %v", key, err)
		return err

	}

	if err := c.plantControl.Reconcile(ctx, plant); err != nil {
		return err
	}

	c.plantQueue.AddAfter(key, c.config.Controllers.Plant.SyncPeriod.Duration)
	return nil
}

// ControlInterface implements the control logic for updating Plants. It is implemented as an interface to allow
// for extensions that provide different semantics. Currently, there is only one implementation.
type ControlInterface interface {
	Reconcile(context.Context, *gardencorev1beta1.Plant) error
}

// NewDefaultPlantControl returns a new instance of the default implementation ControlInterface that
// implements the documented semantics for Plants.
func NewDefaultPlantControl(clientMap clientmap.ClientMap, recorder record.EventRecorder, config *config.ControllerManagerConfiguration, secretLister kubecorev1listers.SecretLister) ControlInterface {
	return &defaultPlantControl{
		clientMap:     clientMap,
		secretsLister: secretLister,
		recorder:      recorder,
		config:        config,
	}
}

func (c *defaultPlantControl) Reconcile(ctx context.Context, obj *gardencorev1beta1.Plant) error {
	var (
		plant  = obj.DeepCopy()
		logger = logger.NewFieldLogger(logger.Logger, "plant", plant.Name)
	)

	if plant.DeletionTimestamp != nil {
		return c.delete(ctx, plant)
	}

	return c.reconcile(ctx, plant, logger)
}

func (c *defaultPlantControl) reconcile(ctx context.Context, plant *gardencorev1beta1.Plant, logger logrus.FieldLogger) error {
	logger.Infof("[PLANT RECONCILE] %s", plant.Name)

	gardenClient, err := c.clientMap.GetClient(ctx, keys.ForGarden())
	if err != nil {
		return fmt.Errorf("failed to get garden client: %w", err)
	}

	// Add Finalizers to Plant
	if err := controllerutils.EnsureFinalizer(ctx, gardenClient.Client(), plant, FinalizerName); err != nil {
		return fmt.Errorf("failed to ensure finalizer on plant: %w", err)
	}

	var (
		conditionAPIServerAvailable = gardencorev1beta1helper.GetOrInitCondition(plant.Status.Conditions, gardencorev1beta1.PlantAPIServerAvailable)
		conditionEveryNodeReady     = gardencorev1beta1helper.GetOrInitCondition(plant.Status.Conditions, gardencorev1beta1.PlantEveryNodeReady)
	)

	kubeconfigSecret, err := c.secretsLister.Secrets(plant.Namespace).Get(plant.Spec.SecretRef.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return updateStatusToUnknown(ctx, gardenClient.DirectClient(), plant, "Referenced Plant secret could not be found.", conditionAPIServerAvailable, conditionEveryNodeReady)
		}
		return fmt.Errorf("failed to get plant secret '%s/%s': %w", plant.Namespace, plant.Spec.SecretRef.Name, err)
	}

	if err := controllerutils.EnsureFinalizer(ctx, gardenClient.Client(), kubeconfigSecret, FinalizerName); err != nil {
		return fmt.Errorf("failed to ensure finalizer on plant secret '%s/%s': %w", plant.Namespace, plant.Spec.SecretRef.Name, err)
	}

	plantClient, err := c.clientMap.GetClient(ctx, keys.ForPlant(plant))
	if err != nil {
		msg := fmt.Sprintf("failed to get plant client: %v", err)
		logger.Error(msg)
		return updateStatusToUnknown(ctx, gardenClient.DirectClient(), plant, msg, conditionAPIServerAvailable, conditionEveryNodeReady)
	}

	healthChecker := NewHealthChecker(plantClient.Client(), plantClient.Kubernetes().Discovery())

	// Trigger health check
	conditionAPIServerAvailable, conditionEveryNodeReady = c.healthChecks(ctx, healthChecker, conditionAPIServerAvailable, conditionEveryNodeReady)

	cloudInfo, err := FetchCloudInfo(ctx, plantClient.Client(), plantClient.Kubernetes().Discovery(), logger)
	if err != nil {
		return fmt.Errorf("failed to fetch cloud info for plant: %w", err)
	}

	return updateStatus(ctx, gardenClient.DirectClient(), plant, cloudInfo, conditionAPIServerAvailable, conditionEveryNodeReady)
}

func updateStatusToUnknown(ctx context.Context, c client.Client, plant *gardencorev1beta1.Plant, message string, conditionAPIServerAvailable, conditionEveryNodeReady gardencorev1beta1.Condition) error {
	conditionAPIServerAvailable = gardencorev1beta1helper.UpdatedCondition(conditionAPIServerAvailable, gardencorev1beta1.ConditionFalse, "APIServerDown", message)
	conditionEveryNodeReady = gardencorev1beta1helper.UpdatedCondition(conditionEveryNodeReady, gardencorev1beta1.ConditionFalse, "Nodes not reachable", message)
	return updateStatus(ctx, c, plant, &StatusCloudInfo{}, conditionAPIServerAvailable, conditionEveryNodeReady)
}

func updateStatus(ctx context.Context, c client.Client, plant *gardencorev1beta1.Plant, cloudInfo *StatusCloudInfo, conditions ...gardencorev1beta1.Condition) error {
	return kutil.TryUpdateStatus(ctx, retry.DefaultBackoff, c, plant, func() error {
		if plant.Status.ClusterInfo == nil {
			plant.Status.ClusterInfo = &gardencorev1beta1.ClusterInfo{}
		}
		plant.Status.ClusterInfo.Cloud.Type = cloudInfo.CloudType
		plant.Status.ClusterInfo.Cloud.Region = cloudInfo.Region
		plant.Status.ClusterInfo.Kubernetes.Version = cloudInfo.K8sVersion
		plant.Status.Conditions = conditions

		return nil
	})
}

func (c *defaultPlantControl) delete(ctx context.Context, plant *gardencorev1beta1.Plant) error {
	gardenClient, err := c.clientMap.GetClient(ctx, keys.ForGarden())
	if err != nil {
		return fmt.Errorf("failed to get garden client: %w", err)
	}

	secret := &corev1.Secret{}
	if err := gardenClient.Client().Get(ctx, kutil.Key(plant.Namespace, plant.Spec.SecretRef.Name), secret); client.IgnoreNotFound(err) != nil {
		return fmt.Errorf("failed to get plant secret '%s/%s': %w", plant.Namespace, plant.Spec.SecretRef.Name, err)
	}

	if err := controllerutils.RemoveFinalizer(ctx, gardenClient.DirectClient(), secret, FinalizerName); err != nil {
		return fmt.Errorf("failed to remove finalizer from plant secret '%s/%s': %w", plant.Namespace, plant.Spec.SecretRef.Name, err)
	}

	if err := controllerutils.RemoveFinalizer(ctx, gardenClient.DirectClient(), plant, FinalizerName); err != nil {
		return fmt.Errorf("failed to remove finalizer from plant: %w", err)
	}

	if err := c.clientMap.InvalidateClient(keys.ForPlant(plant)); err != nil {
		return fmt.Errorf("failed to invalidate plant client: %w", err)
	}

	return nil
}

func (c *defaultPlantControl) healthChecks(ctx context.Context, healthChecker *HealthChecker, apiserverAvailability, everyNodeReady gardencorev1beta1.Condition) (gardencorev1beta1.Condition, gardencorev1beta1.Condition) {
	var wg sync.WaitGroup

	wg.Add(2)
	go func() {
		defer wg.Done()
		apiserverAvailability = healthChecker.CheckAPIServerAvailability(ctx, apiserverAvailability)
	}()
	go func() {
		defer wg.Done()
		everyNodeReady = healthChecker.CheckPlantClusterNodes(ctx, everyNodeReady)
	}()

	wg.Wait()

	return apiserverAvailability, everyNodeReady
}

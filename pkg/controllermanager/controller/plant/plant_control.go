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
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// reconcilePlantForMatchingSecret checks if there is a plant resource that references this secret and then reconciles the plant again
func (c *Controller) reconcilePlantForMatchingSecret(ctx context.Context, obj interface{}) {
	secret, ok := obj.(*corev1.Secret)
	if !ok {
		logger.Logger.Errorf("Could not convert object %v into Secret", obj)
		return
	}

	plantList := &gardencorev1beta1.PlantList{}
	if err := c.gardenClient.List(ctx, plantList); err != nil {
		logger.Logger.Errorf("Couldn't list plants for updated secret %+v: %v", obj, err)
		return
	}

	for _, plant := range plantList.Items {
		if isPlantSecret(plant, kutil.Key(secret.Namespace, secret.Name)) {
			key, err := cache.MetaNamespaceKeyFunc(&plant)
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
func (c *Controller) plantSecretUpdate(ctx context.Context, oldObj, newObj interface{}) {
	old, ok1 := oldObj.(*corev1.Secret)
	new, ok2 := newObj.(*corev1.Secret)
	if !ok1 || !ok2 {
		return
	}
	if old.ResourceVersion != new.ResourceVersion {
		c.reconcilePlantForMatchingSecret(ctx, newObj)
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

// NewPlantReconciler creates a new instance of a reconciler which reconciles Plants.
func NewPlantReconciler(l logrus.FieldLogger, clientMap clientmap.ClientMap, gardenClient client.Client, config *config.PlantControllerConfiguration) reconcile.Reconciler {
	return &plantReconciler{
		logger:       l,
		clientMap:    clientMap,
		gardenClient: gardenClient,
		config:       config,
	}
}

type plantReconciler struct {
	logger       logrus.FieldLogger
	clientMap    clientmap.ClientMap
	gardenClient client.Client
	config       *config.PlantControllerConfiguration
}

func (r *plantReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	plant := &gardencorev1beta1.Plant{}
	if err := r.gardenClient.Get(ctx, request.NamespacedName, plant); err != nil {
		if apierrors.IsNotFound(err) {
			r.logger.Infof("Object %q is gone, stop reconciling: %v", request.Name, err)
			return reconcile.Result{}, nil
		}
		r.logger.Infof("Unable to retrieve object %q from store: %v", request.Name, err)
		return reconcile.Result{}, err
	}

	if plant.DeletionTimestamp != nil {
		if err := r.delete(ctx, plant, r.gardenClient); err != nil {
			return reconcile.Result{}, err
		}
	}

	if err := r.reconcile(ctx, plant, r.gardenClient); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{RequeueAfter: r.config.SyncPeriod.Duration}, nil
}

func (r *plantReconciler) reconcile(ctx context.Context, plant *gardencorev1beta1.Plant, gardenClient client.Client) error {
	logger := logger.NewFieldLogger(r.logger, "plant", plant.Name)
	logger.Infof("[PLANT RECONCILE] %s", plant.Name)

	// Add Finalizers to Plant
	if !controllerutil.ContainsFinalizer(plant, FinalizerName) {
		if err := controllerutils.StrategicMergePatchAddFinalizers(ctx, gardenClient, plant, FinalizerName); err != nil {
			return fmt.Errorf("failed to ensure finalizer on plant: %w", err)
		}
	}

	var (
		conditionAPIServerAvailable = gardencorev1beta1helper.GetOrInitCondition(plant.Status.Conditions, gardencorev1beta1.PlantAPIServerAvailable)
		conditionEveryNodeReady     = gardencorev1beta1helper.GetOrInitCondition(plant.Status.Conditions, gardencorev1beta1.PlantEveryNodeReady)
	)

	kubeconfigSecret := &corev1.Secret{}
	if err := r.gardenClient.Get(ctx, kutil.Key(plant.Namespace, plant.Spec.SecretRef.Name), kubeconfigSecret); err != nil {
		if apierrors.IsNotFound(err) {
			return updateStatusToUnknown(ctx, gardenClient, plant, "Referenced Plant secret could not be found.", conditionAPIServerAvailable, conditionEveryNodeReady)
		}
		return fmt.Errorf("failed to get plant secret '%s/%s': %w", plant.Namespace, plant.Spec.SecretRef.Name, err)
	}

	if !controllerutil.ContainsFinalizer(kubeconfigSecret, FinalizerName) {
		if err := controllerutils.StrategicMergePatchAddFinalizers(ctx, gardenClient, kubeconfigSecret, FinalizerName); err != nil {
			return fmt.Errorf("failed to ensure finalizer on plant secret '%s/%s': %w", plant.Namespace, plant.Spec.SecretRef.Name, err)
		}
	}

	plantClient, err := r.clientMap.GetClient(ctx, keys.ForPlant(plant))
	if err != nil {
		msg := fmt.Sprintf("failed to get plant client: %v", err)
		logger.Error(msg)
		return updateStatusToUnknown(ctx, gardenClient, plant, msg, conditionAPIServerAvailable, conditionEveryNodeReady)
	}

	healthChecker := NewHealthChecker(plantClient.Client(), plantClient.Kubernetes().Discovery())

	// Trigger health check
	conditionAPIServerAvailable, conditionEveryNodeReady = healthChecks(ctx, healthChecker, conditionAPIServerAvailable, conditionEveryNodeReady)

	cloudInfo, err := FetchCloudInfo(ctx, plantClient.Client(), plantClient.Kubernetes().Discovery(), logger)
	if err != nil {
		return fmt.Errorf("failed to fetch cloud info for plant: %w", err)
	}

	return updateStatus(ctx, gardenClient, plant, cloudInfo, conditionAPIServerAvailable, conditionEveryNodeReady)
}

func updateStatusToUnknown(ctx context.Context, c client.Client, plant *gardencorev1beta1.Plant, message string, conditionAPIServerAvailable, conditionEveryNodeReady gardencorev1beta1.Condition) error {
	conditionAPIServerAvailable = gardencorev1beta1helper.UpdatedCondition(conditionAPIServerAvailable, gardencorev1beta1.ConditionFalse, "APIServerDown", message)
	conditionEveryNodeReady = gardencorev1beta1helper.UpdatedCondition(conditionEveryNodeReady, gardencorev1beta1.ConditionFalse, "Nodes not reachable", message)
	return updateStatus(ctx, c, plant, &StatusCloudInfo{}, conditionAPIServerAvailable, conditionEveryNodeReady)
}

func updateStatus(ctx context.Context, c client.Client, plant *gardencorev1beta1.Plant, cloudInfo *StatusCloudInfo, conditions ...gardencorev1beta1.Condition) error {
	patch := client.StrategicMergeFrom(plant.DeepCopy())

	if plant.Status.ClusterInfo == nil {
		plant.Status.ClusterInfo = &gardencorev1beta1.ClusterInfo{}
	}
	plant.Status.ClusterInfo.Cloud.Type = cloudInfo.CloudType
	plant.Status.ClusterInfo.Cloud.Region = cloudInfo.Region
	plant.Status.ClusterInfo.Kubernetes.Version = cloudInfo.K8sVersion
	plant.Status.Conditions = conditions

	return c.Status().Patch(ctx, plant, patch)
}

func (r *plantReconciler) delete(ctx context.Context, plant *gardencorev1beta1.Plant, gardenClient client.Client) error {
	secret := &corev1.Secret{}
	err := gardenClient.Get(ctx, kutil.Key(plant.Namespace, plant.Spec.SecretRef.Name), secret)
	if err == nil {
		if err2 := controllerutils.PatchRemoveFinalizers(ctx, gardenClient, secret, FinalizerName); err2 != nil {
			return fmt.Errorf("failed to remove finalizer from plant secret '%s/%s': %w", plant.Namespace, plant.Spec.SecretRef.Name, err2)
		}
	} else if !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to get plant secret '%s/%s': %w", plant.Namespace, plant.Spec.SecretRef.Name, err)
	}

	if err := controllerutils.PatchRemoveFinalizers(ctx, gardenClient, plant, FinalizerName); client.IgnoreNotFound(err) != nil {
		return fmt.Errorf("failed to remove finalizer from plant: %w", err)
	}

	if err := r.clientMap.InvalidateClient(keys.ForPlant(plant)); err != nil {
		return fmt.Errorf("failed to invalidate plant client: %w", err)
	}

	return nil
}

func healthChecks(ctx context.Context, healthChecker *HealthChecker, apiserverAvailability, everyNodeReady gardencorev1beta1.Condition) (gardencorev1beta1.Condition, gardencorev1beta1.Condition) {
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

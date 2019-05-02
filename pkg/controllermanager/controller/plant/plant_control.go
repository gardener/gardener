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

	"k8s.io/apimachinery/pkg/labels"

	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kubernetesclientset "k8s.io/client-go/kubernetes"
	kubecorev1listers "k8s.io/client-go/listers/core/v1"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	"github.com/gardener/gardener/pkg/apis/core/v1alpha1/helper"
	gardencorelisters "github.com/gardener/gardener/pkg/client/core/listers/core/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	"github.com/gardener/gardener/pkg/logger"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
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
	old, ok1 := oldObj.(*gardencorev1alpha1.Plant)
	new, ok2 := newObj.(*gardencorev1alpha1.Plant)
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

	if err := c.plantControl.Reconcile(ctx, plant, key); err != nil {
		return err
	}

	c.plantQueue.AddAfter(key, c.config.Controllers.Plant.SyncPeriod.Duration)
	return nil
}

// ControlInterface implements the control logic for updating Plants. It is implemented as an interface to allow
// for extensions that provide different semantics. Currently, there is only one implementation.
type ControlInterface interface {
	Reconcile(context.Context, *gardencorev1alpha1.Plant, string) error
}

// NewDefaultPlantControl returns a new instance of the default implementation ControlInterface that
// implements the documented semantics for Plants. updater is the UpdaterInterface used
// to update the status of Plants.
func NewDefaultPlantControl(k8sGardenClient kubernetes.Interface, recorder record.EventRecorder, config *config.ControllerManagerConfiguration, plantsLister gardencorelisters.PlantLister, secretLister kubecorev1listers.SecretLister) ControlInterface {
	return &defaultPlantControl{
		k8sGardenClient: k8sGardenClient,
		plantLister:     plantsLister,
		secretsLister:   secretLister,
		recorder:        recorder,
		config:          config,
	}
}

func (c *defaultPlantControl) Reconcile(ctx context.Context, obj *gardencorev1alpha1.Plant, key string) error {
	var (
		plant  = obj.DeepCopy()
		logger = logger.NewFieldLogger(logger.Logger, "plant", plant.Name)
	)

	if plant.DeletionTimestamp != nil {
		return c.delete(ctx, plant, logger)
	}

	return c.reconcile(ctx, plant, key, logger)
}

func (c *defaultPlantControl) reconcile(ctx context.Context, plant *gardencorev1alpha1.Plant, key string, logger logrus.FieldLogger) error {
	logger.Infof("[PLANT RECONCILE] %s", plant.Name)

	// Add Finalizers to Plant
	if finalizers := sets.NewString(plant.Finalizers...); !finalizers.Has(FinalizerName) {
		updatePlant := plant.DeepCopy()

		finalizers.Insert(FinalizerName)
		updatePlant.Finalizers = finalizers.UnsortedList()
		if err := c.k8sGardenClient.Client().Status().Update(ctx, updatePlant); err != nil {
			return err
		}
	}

	var (
		conditionAPIServerAvailable = helper.GetOrInitCondition(plant.Status.Conditions, gardencorev1alpha1.PlantAPIServerAvailable)
		conditionEveryNodeReady     = helper.GetOrInitCondition(plant.Status.Conditions, gardencorev1alpha1.PlantEveryNodeReady)
	)

	kubeconfigSecret, err := c.secretsLister.Secrets(plant.Namespace).Get(plant.Spec.SecretRef.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return c.updateStatusToUnknown(ctx, plant, "Referenced Plant secret could not be found.", conditionAPIServerAvailable, conditionEveryNodeReady)
		}
		return err
	}

	secretFinalizers := sets.NewString(kubeconfigSecret.Finalizers...)
	if !secretFinalizers.Has(FinalizerName) {
		secretFinalizers.Insert(FinalizerName)
	}
	kubeconfigSecret.Finalizers = secretFinalizers.UnsortedList()
	if err := c.k8sGardenClient.Client().Update(ctx, kubeconfigSecret); err != nil {
		return err
	}

	kubeconfig, ok := kubeconfigSecret.Data["kubeconfig"]
	if !ok {
		message := "Plant secret needs to contain a kubeconfig key."
		return c.updateStatusToUnknown(ctx, plant, message, conditionAPIServerAvailable, conditionEveryNodeReady)
	}

	plantClusterClient, discoveryClient, err := c.initializePlantClients(plant, key, kubeconfig)
	if err != nil {
		message := fmt.Sprintf("Could not initialize Plant clients: %+v", err)
		return c.updateStatusToUnknown(ctx, plant, message, conditionAPIServerAvailable, conditionEveryNodeReady)
	}

	healthChecker := NewHealthChecker(plantClusterClient, discoveryClient)

	// Trigger health check
	conditionAPIServerAvailable, conditionEveryNodeReady = c.healthChecks(ctx, healthChecker, logger, conditionAPIServerAvailable, conditionEveryNodeReady)

	cloudInfo, err := FetchCloudInfo(ctx, plantClusterClient, discoveryClient, logger)
	if err != nil {
		return err
	}

	return c.updateStatus(ctx, plant, cloudInfo, conditionAPIServerAvailable, conditionEveryNodeReady)
}

func (c *defaultPlantControl) updateStatusToUnknown(ctx context.Context, plant *gardencorev1alpha1.Plant, message string, conditionAPIServerAvailable, conditionEveryNodeReady gardencorev1alpha1.Condition) error {
	conditionAPIServerAvailable = helper.UpdatedCondition(conditionAPIServerAvailable, gardencorev1alpha1.ConditionFalse, "APIServerDown", message)
	conditionEveryNodeReady = helper.UpdatedCondition(conditionEveryNodeReady, gardencorev1alpha1.ConditionFalse, "Nodes not reachable", message)
	return c.updateStatus(ctx, plant, &StatusCloudInfo{}, conditionAPIServerAvailable, conditionEveryNodeReady)
}

func (c *defaultPlantControl) updateStatus(ctx context.Context, plant *gardencorev1alpha1.Plant, cloudInfo *StatusCloudInfo, conditions ...gardencorev1alpha1.Condition) error {
	updatePlant := plant.DeepCopy()
	if updatePlant.Status.ClusterInfo == nil {
		updatePlant.Status.ClusterInfo = &gardencorev1alpha1.ClusterInfo{}
	}
	updatePlant.Status.ClusterInfo.Cloud.Type = cloudInfo.CloudType
	updatePlant.Status.ClusterInfo.Cloud.Region = cloudInfo.Region
	updatePlant.Status.ClusterInfo.Kubernetes.Version = cloudInfo.K8sVersion
	updatePlant.Status.Conditions = conditions

	if !equality.Semantic.DeepEqual(plant, updatePlant) {
		return c.k8sGardenClient.Client().Status().Update(ctx, updatePlant)
	}
	return nil
}

func (c *defaultPlantControl) delete(ctx context.Context, plant *gardencorev1alpha1.Plant, logger logrus.FieldLogger) error {
	if err := c.removeFinalizerFromSecret(ctx, plant); err != nil {
		return err
	}

	return c.removeFinalizersFromPlant(ctx, plant)
}

func (c *defaultPlantControl) removeFinalizerFromSecret(ctx context.Context, plant *gardencorev1alpha1.Plant) error {
	secret, err := c.secretsLister.Secrets(plant.Namespace).Get(plant.Spec.SecretRef.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	secretFinalizers := sets.NewString(secret.Finalizers...)
	secretFinalizers.Delete(FinalizerName)
	secret.Finalizers = secretFinalizers.UnsortedList()

	return c.k8sGardenClient.Client().Update(ctx, secret)

}

func (c *defaultPlantControl) removeFinalizersFromPlant(ctx context.Context, plant *gardencorev1alpha1.Plant) error {
	if sets.NewString(plant.Finalizers...).Has(FinalizerName) {
		finalizers := sets.NewString(plant.Finalizers...)
		finalizers.Delete(FinalizerName)
		plant.Finalizers = finalizers.UnsortedList()
		if err := c.k8sGardenClient.Client().Update(ctx, plant); err != nil && !apierrors.IsConflict(err) {
			return err
		}
	}
	return nil
}

func (c *defaultPlantControl) initializePlantClients(plant *gardencorev1alpha1.Plant, key string, kubeconfigSecretValue []byte) (client.Client, *kubernetesclientset.Clientset, error) {
	config, err := clientcmd.RESTConfigFromKubeConfig(kubeconfigSecretValue)
	if err != nil {
		return nil, nil, fmt.Errorf("%v:%v", "invalid kubconfig supplied resulted in: ", err)
	}

	plantClusterClient, err := kubernetes.NewRuntimeClientForConfig(config, client.Options{
		Scheme: kubernetes.PlantScheme,
	})
	if err != nil {
		return nil, nil, err
	}

	discoveryClient, err := kubernetesclientset.NewForConfig(config)
	if err != nil {
		return nil, nil, err
	}

	return plantClusterClient, discoveryClient, nil
}

func (c *defaultPlantControl) healthChecks(ctx context.Context, healthChecker *HealthChecker, logger logrus.FieldLogger, apiserverAvailability, everyNodeReady gardencorev1alpha1.Condition) (gardencorev1alpha1.Condition, gardencorev1alpha1.Condition) {
	var wg sync.WaitGroup

	wg.Add(2)
	go func() {
		defer wg.Done()
		apiserverAvailability = healthChecker.CheckAPIServerAvailability(apiserverAvailability)
	}()
	go func() {
		defer wg.Done()
		everyNodeReady = healthChecker.CheckPlantClusterNodes(ctx, everyNodeReady)
	}()

	wg.Wait()

	return apiserverAvailability, everyNodeReady
}

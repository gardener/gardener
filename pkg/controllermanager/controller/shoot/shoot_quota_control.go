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

package shoot

import (
	"context"
	"fmt"
	"time"

	"github.com/gardener/gardener/pkg/client/kubernetes"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/operation/common"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
)

func (c *Controller) shootQuotaAdd(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		return
	}
	c.shootQuotaQueue.Add(key)
}

func (c *Controller) shootQuotaDelete(obj interface{}) {
	shoot, ok := obj.(*gardencorev1beta1.Shoot)
	if shoot == nil || !ok {
		return
	}
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		return
	}
	c.shootQuotaQueue.Done(key)
}

func (c *Controller) reconcileShootQuotaKey(key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	shoot, err := c.shootLister.Shoots(namespace).Get(name)
	if apierrors.IsNotFound(err) {
		logger.Logger.Debugf("[SHOOT QUOTA] %s - skipping because Shoot has been deleted", key)
		return nil
	}
	if err != nil {
		logger.Logger.Infof("[SHOOT QUOTA] %s - unable to retrieve object from store: %v", key, err)
		return err
	}

	if err := c.quotaControl.CheckQuota(shoot, key); err != nil {
		c.shootQuotaQueue.AddAfter(key, 2*time.Minute)
		return nil
	}
	c.shootQuotaQueue.AddAfter(key, c.config.Controllers.ShootQuota.SyncPeriod.Duration)
	return nil
}

// QuotaControlInterface implements the control logic for quota management of Shoots. It is implemented as an interface to allow
// for extensions that provide different semantics. Currently, there is only one implementation.
type QuotaControlInterface interface {
	CheckQuota(shoot *gardencorev1beta1.Shoot, key string) error
}

// NewDefaultQuotaControl returns a new instance of the default implementation of QuotaControlInterface
// which implements the semantics for controlling the quota handling of Shoot resources.
func NewDefaultQuotaControl(clientMap clientmap.ClientMap, k8sGardenCoreInformers gardencoreinformers.Interface) QuotaControlInterface {
	return &defaultQuotaControl{clientMap, k8sGardenCoreInformers}
}

type defaultQuotaControl struct {
	clientMap              clientmap.ClientMap
	k8sGardenCoreInformers gardencoreinformers.Interface
}

func (c *defaultQuotaControl) CheckQuota(shootObj *gardencorev1beta1.Shoot, key string) error {
	var (
		ctx             = context.TODO()
		clusterLifeTime *int32
		shoot           = shootObj.DeepCopy()
		shootLogger     = logger.NewShootLogger(logger.Logger, shoot.Name, shoot.Namespace)
	)

	secretBinding, err := c.k8sGardenCoreInformers.SecretBindings().Lister().SecretBindings(shoot.Namespace).Get(shoot.Spec.SecretBindingName)
	if err != nil {
		return err
	}
	for _, quotaRef := range secretBinding.Quotas {
		quota, err := c.k8sGardenCoreInformers.Quotas().Lister().Quotas(quotaRef.Namespace).Get(quotaRef.Name)
		if err != nil {
			return err
		}

		if quota.Spec.ClusterLifetimeDays == nil {
			continue
		}
		if clusterLifeTime == nil || *quota.Spec.ClusterLifetimeDays < *clusterLifeTime {
			clusterLifeTime = quota.Spec.ClusterLifetimeDays
		}
	}

	// If the Shoot has no Quotas referenced (anymore) or if the referenced Quotas does not have a clusterLifetime,
	// then we will not check for cluster lifetime expiration, even if the Shoot has a clusterLifetime timestamp already annotated.
	if clusterLifeTime == nil {
		return nil
	}

	gardenClient, err := c.clientMap.GetClient(ctx, keys.ForGarden())
	if err != nil {
		return fmt.Errorf("failed to get garden client: %w", err)
	}

	expirationTime, exits := shoot.Annotations[common.ShootExpirationTimestamp]
	if !exits {
		expirationTime = shoot.CreationTimestamp.Add(time.Duration(*clusterLifeTime*24) * time.Hour).Format(time.RFC3339)
		metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, common.ShootExpirationTimestamp, expirationTime)

		shootUpdated, err := gardenClient.GardenCore().CoreV1beta1().Shoots(shoot.Namespace).Update(ctx, shoot, kubernetes.DefaultUpdateOptions())
		if err != nil {
			return err
		}
		shoot = shootUpdated
	}

	expirationTimeParsed, err := time.Parse(time.RFC3339, expirationTime)
	if err != nil {
		return err
	}

	if time.Now().UTC().After(expirationTimeParsed.UTC()) {
		shootLogger.Info("[SHOOT QUOTA] Shoot cluster lifetime expired. Shoot will be deleted.")

		// We have to annotate the Shoot to confirm the deletion.
		if err := common.ConfirmDeletion(ctx, gardenClient.Client(), shoot); err != nil {
			return err
		}

		// Now we are allowed to delete the Shoot (to set the deletionTimestamp).
		if err := gardenClient.GardenCore().CoreV1beta1().Shoots(shoot.Namespace).Delete(ctx, shoot.Name, metav1.DeleteOptions{}); err != nil {
			return err
		}
	}
	return nil
}

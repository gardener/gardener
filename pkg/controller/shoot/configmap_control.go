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
	"github.com/gardener/gardener/pkg/logger"

	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

func (c *Controller) configMapAdd(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		logger.Logger.Errorf("[ConfigMap controller] Couldn't get key for object %+v: %v", obj, err)
		return
	}
	c.configMapQueue.Add(key)
}

func (c *Controller) configMapUpdate(oldObj, newObj interface{}) {
	var (
		oldConfigMap = oldObj.(*corev1.ConfigMap)
		newConfigMap = newObj.(*corev1.ConfigMap)
	)

	if apiequality.Semantic.Equalities.DeepEqual(oldConfigMap.Data, newConfigMap.Data) {
		logger.Logger.Debugf("[ConfigMap controller] No update of the `.data` field of cm %v/%v. Do not requeue the ConfigMap", oldConfigMap.Namespace, oldConfigMap.Name)
		return
	}
	c.configMapAdd(newObj)
}

func (c *Controller) reconcileConfigMapKey(key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}
	return c.reconcileShootsReferringConfigMap(name, namespace)
}

func (c *Controller) reconcileShootsReferringConfigMap(configMapName string, configMapNamespace string) error {
	shoots, err := c.shootLister.Shoots(configMapNamespace).List(labels.Everything())
	if err != nil {
		return err
	}
	for _, shoot := range shoots {
		if shoot.Spec.Kubernetes.KubeAPIServer != nil &&
			shoot.Spec.Kubernetes.KubeAPIServer.AuditConfig != nil &&
			shoot.Spec.Kubernetes.KubeAPIServer.AuditConfig.AuditPolicy != nil &&
			shoot.Spec.Kubernetes.KubeAPIServer.AuditConfig.AuditPolicy.ConfigMapRef != nil &&
			shoot.Spec.Kubernetes.KubeAPIServer.AuditConfig.AuditPolicy.ConfigMapRef.Name == configMapName {
			if shootKey, err := cache.MetaNamespaceKeyFunc(shoot); err == nil {
				logger.Logger.Infof("[ConfigMap controller] schedule for reconciliation shoot %v ", shootKey)
				c.shootQueue.Add(shootKey)
			} else {
				logger.Logger.Errorf("[ConfigMap controller] failed to get key for shoot. err=%+v", err)
			}
		}
	}
	return nil
}

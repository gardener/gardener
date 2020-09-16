// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"context"
	"fmt"

	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/utils/kubernetes"

	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
		logger.Logger.Debugf("[SHOOT CONFIGMAP controller] No update of the `.data` field of cm %v/%v. Do not requeue the ConfigMap", oldConfigMap.Namespace, oldConfigMap.Name)
		return
	}
	c.configMapAdd(newObj)
}

func (c *Controller) reconcileConfigMapKey(key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	configMap, err := c.configMapLister.ConfigMaps(namespace).Get(name)
	if apierrors.IsNotFound(err) {
		logger.Logger.Debugf("[SHOOT CONFIGMAP] %s - skipping because ConfigMap has been deleted", key)
		return nil
	}
	if err != nil {
		logger.Logger.Errorf("[SHOOT CONFIGMAP] %s - unable to retrieve object from store: %v", key, err)
		return err
	}

	return c.reconcileShootsReferringConfigMap(configMap)
}

func (c *Controller) reconcileShootsReferringConfigMap(configMap *corev1.ConfigMap) error {
	ctx := context.TODO()

	gardenClient, err := c.clientMap.GetClient(ctx, keys.ForGarden())
	if err != nil {
		return fmt.Errorf("failed to get garden client: %w", err)
	}

	shoots, err := c.shootLister.Shoots(configMap.Namespace).List(labels.Everything())
	if err != nil {
		return err
	}

	for _, shoot := range shoots {
		if shoot.Spec.Kubernetes.KubeAPIServer != nil &&
			shoot.Spec.Kubernetes.KubeAPIServer.AuditConfig != nil &&
			shoot.Spec.Kubernetes.KubeAPIServer.AuditConfig.AuditPolicy != nil &&
			shoot.Spec.Kubernetes.KubeAPIServer.AuditConfig.AuditPolicy.ConfigMapRef != nil &&
			shoot.Spec.Kubernetes.KubeAPIServer.AuditConfig.AuditPolicy.ConfigMapRef.Name == configMap.Name {

			shootKey, err := cache.MetaNamespaceKeyFunc(shoot)
			if err != nil {
				logger.Logger.Errorf("[SHOOT CONFIGMAP controller] failed to get key for shoot. err=%+v", err)
				continue
			}

			if shoot.Spec.Kubernetes.KubeAPIServer.AuditConfig.AuditPolicy.ConfigMapRef.ResourceVersion != configMap.ResourceVersion {
				logger.Logger.Infof("[SHOOT CONFIGMAP controller] schedule for reconciliation shoot %v ", shootKey)
				// send empty patch to let the admission plugin add the config map resource version
				if err := kubernetes.SubmitEmptyPatch(context.TODO(), gardenClient.Client(), shoot); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

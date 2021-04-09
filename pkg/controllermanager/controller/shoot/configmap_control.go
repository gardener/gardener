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

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/logger"
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

// NewConfigMapReconciler creates a new instance of a reconciler which reconciles ConfigMaps.
func NewConfigMapReconciler(l logrus.FieldLogger, gardenClient client.Client) reconcile.Reconciler {
	return &configMapReconciler{
		logger:       l,
		gardenClient: gardenClient,
	}
}

type configMapReconciler struct {
	logger       logrus.FieldLogger
	gardenClient client.Client
}

func (r *configMapReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	configMap := &corev1.ConfigMap{}
	if err := r.gardenClient.Get(ctx, request.NamespacedName, configMap); err != nil {
		if apierrors.IsNotFound(err) {
			r.logger.Infof("Object %q is gone, stop reconciling: %v", request.Name, err)
			return reconcile.Result{}, nil
		}
		r.logger.Infof("Unable to retrieve object %q from store: %v", request.Name, err)
		return reconcile.Result{}, err
	}

	shootList := &gardencorev1beta1.ShootList{}
	if err := r.gardenClient.List(ctx, shootList, client.InNamespace(configMap.Namespace)); err != nil {
		return reconcile.Result{}, err
	}

	for _, shoot := range shootList.Items {
		if shoot.DeletionTimestamp != nil {
			// spec of shoot that is marked for deletion cannot be updated
			continue
		}

		if shoot.Spec.Kubernetes.KubeAPIServer != nil &&
			shoot.Spec.Kubernetes.KubeAPIServer.AuditConfig != nil &&
			shoot.Spec.Kubernetes.KubeAPIServer.AuditConfig.AuditPolicy != nil &&
			shoot.Spec.Kubernetes.KubeAPIServer.AuditConfig.AuditPolicy.ConfigMapRef != nil &&
			shoot.Spec.Kubernetes.KubeAPIServer.AuditConfig.AuditPolicy.ConfigMapRef.Name == configMap.Name {

			shootKey, err := cache.MetaNamespaceKeyFunc(&shoot)
			if err != nil {
				logger.Logger.Errorf("[SHOOT CONFIGMAP controller] failed to get key for shoot. err=%+v", err)
				continue
			}

			if shoot.Spec.Kubernetes.KubeAPIServer.AuditConfig.AuditPolicy.ConfigMapRef.ResourceVersion != configMap.ResourceVersion {
				logger.Logger.Infof("[SHOOT CONFIGMAP controller] schedule for reconciliation shoot %v", shootKey)

				patch := client.MergeFrom(shoot.DeepCopy())
				shoot.Spec.Kubernetes.KubeAPIServer.AuditConfig.AuditPolicy.ConfigMapRef.ResourceVersion = configMap.ResourceVersion
				if err := r.gardenClient.Patch(ctx, &shoot, patch); err != nil {
					return reconcile.Result{}, err
				}
			}
		}
	}

	return reconcile.Result{}, nil
}

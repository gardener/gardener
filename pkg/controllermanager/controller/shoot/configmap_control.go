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

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	"github.com/go-logr/logr"
)

const (
	// ConfigMapControllerName is the name of the shoot-configmap controller.
	ConfigMapControllerName = "shoot-configmap"
)

func addConfigMapController(mgr manager.Manager, config *config.ShootMaintenanceControllerConfiguration) error {
	reconciler := NewConfigMapReconciler(mgr.GetLogger(), mgr.GetClient())

	ctrlOptions := controller.Options{
		Reconciler:              reconciler,
		MaxConcurrentReconciles: config.ConcurrentSyncs,
	}
	c, err := controller.New(ConfigMapControllerName, mgr, ctrlOptions)
	if err != nil {
		return err
	}

	reconciler.logger = c.GetLogger()

	configMap := &corev1.ConfigMap{}
	if err := c.Watch(&source.Kind{Type: configMap}, &handler.EnqueueRequestForObject{}); err != nil {
		return fmt.Errorf("failed to create watcher for %T: %w", configMap, err)
	}

	return nil
}

// NewConfigMapReconciler creates a new instance of a reconciler which reconciles ConfigMaps.
func NewConfigMapReconciler(l logr.Logger, gardenClient client.Client) *configMapReconciler {
	return &configMapReconciler{
		logger:       l,
		gardenClient: gardenClient,
	}
}

type configMapReconciler struct {
	logger       logr.Logger
	gardenClient client.Client
}

func (r *configMapReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	logger := r.logger.WithValues("shoot", request)

	configMap := &corev1.ConfigMap{}
	if err := r.gardenClient.Get(ctx, request.NamespacedName, configMap); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}

		logger.Error(err, "Unable to retrieve object from store")
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

			if shoot.Spec.Kubernetes.KubeAPIServer.AuditConfig.AuditPolicy.ConfigMapRef.ResourceVersion != configMap.ResourceVersion {
				logger.Info("schedule for reconciliation shoot")

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

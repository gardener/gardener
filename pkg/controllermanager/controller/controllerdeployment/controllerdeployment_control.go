// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package controllerdeployment

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap"
	"github.com/gardener/gardener/pkg/client/kubernetes/clientmap/keys"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/logger"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
)

func (c *Controller) controllerDeploymentAdd(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		logger.Logger.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}
	c.controllerDeploymentQueue.Add(key)
}

func (c *Controller) controllerDeploymentUpdate(_, newObj interface{}) {
	c.controllerDeploymentAdd(newObj)
}

// NewReconciler creates a new instance of a reconciler which reconciles ControllerDeployments.
func NewReconciler(clientMap clientmap.ClientMap, l logrus.FieldLogger) reconcile.Reconciler {
	return &controllerDeploymentReconciler{
		logger:    l,
		clientMap: clientMap,
	}
}

type controllerDeploymentReconciler struct {
	logger    logrus.FieldLogger
	clientMap clientmap.ClientMap
}

func (c *controllerDeploymentReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	gardenClient, err := c.clientMap.GetClient(ctx, keys.ForGarden())
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get garden client: %w", err)
	}

	controllerDeployment := &gardencorev1beta1.ControllerDeployment{}
	if err := gardenClient.Client().Get(ctx, kutil.Key(req.Name), controllerDeployment); err != nil {
		if apierrors.IsNotFound(err) {
			c.logger.Debugf("[CONTROLLERDEPLOYMENT RECONCILE] %s - skipping because ControllerDeployment has been deleted", req.Name)
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	if controllerDeployment.DeletionTimestamp != nil {
		if !controllerutil.ContainsFinalizer(controllerDeployment, FinalizerName) {
			return reconcile.Result{}, nil
		}

		controllerRegistrationList := &gardencorev1beta1.ControllerRegistrationList{}
		if err := gardenClient.Client().List(ctx, controllerRegistrationList); err != nil {
			return reconcile.Result{}, err
		}

		for _, controllerRegistration := range controllerRegistrationList.Items {
			deployment := controllerRegistration.Spec.Deployment
			if deployment == nil {
				continue
			}
			if len(deployment.DeploymentRefs) < 1 {
				continue
			}
			for _, deploymentRef := range deployment.DeploymentRefs {
				if deploymentRef.Name == controllerDeployment.Name {
					return reconcile.Result{}, fmt.Errorf("cannot remove finalizer of ControllerDeployment %q because still found one ControllerRegistration", controllerRegistration.Name)
				}
			}
		}

		return reconcile.Result{}, controllerutils.PatchRemoveFinalizers(ctx, gardenClient.Client(), controllerDeployment, FinalizerName)
	}

	return reconcile.Result{}, controllerutils.PatchAddFinalizers(ctx, gardenClient.Client(), controllerDeployment, FinalizerName)
}

// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package controllerregistration

import (
	"context"
	"fmt"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/controllerutils"

	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func (c *Controller) controllerRegistrationAdd(ctx context.Context, obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		return
	}
	c.controllerRegistrationQueue.Add(key)
	c.enqueueAllSeeds(ctx)
}

func (c *Controller) controllerRegistrationUpdate(ctx context.Context, _, newObj interface{}) {
	c.controllerRegistrationAdd(ctx, newObj)
}

func (c *Controller) controllerRegistrationDelete(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		return
	}
	c.controllerRegistrationQueue.Add(key)
}

// NewControllerRegistrationReconciler creates a new instance of a reconciler which reconciles ControllerRegistrations.
func NewControllerRegistrationReconciler(logger logrus.FieldLogger, gardenClient client.Client) reconcile.Reconciler {
	return &controllerRegistrationReconciler{
		logger:       logger,
		gardenClient: gardenClient,
	}
}

type controllerRegistrationReconciler struct {
	logger       logrus.FieldLogger
	gardenClient client.Client
}

func (r *controllerRegistrationReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	controllerRegistration := &gardencorev1beta1.ControllerRegistration{}
	if err := r.gardenClient.Get(ctx, request.NamespacedName, controllerRegistration); err != nil {
		if apierrors.IsNotFound(err) {
			r.logger.Infof("Object %q is gone, stop reconciling: %v", request.Name, err)
			return reconcile.Result{}, nil
		}
		r.logger.Infof("Unable to retrieve object %q from store: %v", request.Name, err)
		return reconcile.Result{}, err
	}

	if controllerRegistration.DeletionTimestamp != nil {
		if !controllerutil.ContainsFinalizer(controllerRegistration, FinalizerName) {
			return reconcile.Result{}, nil
		}

		controllerInstallationList := &gardencorev1beta1.ControllerInstallationList{}
		if err := r.gardenClient.List(ctx, controllerInstallationList); err != nil {
			return reconcile.Result{}, err
		}

		for _, controllerInstallation := range controllerInstallationList.Items {
			if controllerInstallation.Spec.RegistrationRef.Name == controllerRegistration.Name {
				return reconcile.Result{}, fmt.Errorf("cannot remove finalizer of ControllerRegistration %q because still found at least one ControllerInstallation", controllerRegistration.Name)
			}
		}

		return reconcile.Result{}, controllerutils.PatchRemoveFinalizers(ctx, r.gardenClient, controllerRegistration, FinalizerName)
	}

	if !controllerutil.ContainsFinalizer(controllerRegistration, FinalizerName) {
		return reconcile.Result{}, controllerutils.StrategicMergePatchAddFinalizers(ctx, r.gardenClient, controllerRegistration, FinalizerName)
	}

	return reconcile.Result{}, nil
}

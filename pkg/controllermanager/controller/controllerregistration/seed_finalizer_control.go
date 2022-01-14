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

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const seedFinalizerReconcilerName = "seed-finalizer"

func (c *Controller) seedAdd(obj interface{}, addToControllerRegistrationQueue bool) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		c.log.Error(err, "Couldn't get key for object", "object", obj)
		return
	}

	c.seedFinalizerQueue.Add(key)
	if addToControllerRegistrationQueue {
		c.seedQueue.Add(key)
	}
}

func (c *Controller) seedUpdate(oldObj, newObj interface{}) {
	oldObject, ok := oldObj.(*gardencorev1beta1.Seed)
	if !ok {
		return
	}

	newObject, ok := newObj.(*gardencorev1beta1.Seed)
	if !ok {
		return
	}

	enqueue := !apiequality.Semantic.DeepEqual(oldObject.Spec.DNS.Provider, newObject.Spec.DNS.Provider) ||
		newObject.DeletionTimestamp != nil

	c.seedAdd(newObj, enqueue)
}

func (c *Controller) seedDelete(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		c.log.Error(err, "Couldn't get key for object", "object", obj)
		return
	}
	c.seedFinalizerQueue.Add(key)
	c.seedQueue.Add(key)
}

// NewSeedFinalizerReconciler creates a new reconciler that manages the finalizer on Seed objects depending on whether
// ControllerInstallation objects exist in the system.
// It basically protects Seeds from being deleted, if there are still ControllerInstallations referencing it, to make
// sure we are able to cleanup ControllerInstallation objects of terminating Seeds.
func NewSeedFinalizerReconciler(gardenClient client.Client) reconcile.Reconciler {
	return &seedReconciler{
		gardenClient: gardenClient,
	}
}

type seedReconciler struct {
	gardenClient client.Client
}

func (r *seedReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	seed := &gardencorev1beta1.Seed{}
	if err := r.gardenClient.Get(ctx, request.NamespacedName, seed); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	if seed.DeletionTimestamp != nil {
		if !controllerutil.ContainsFinalizer(seed, FinalizerName) {
			return reconcile.Result{}, nil
		}

		controllerInstallationList := &gardencorev1beta1.ControllerInstallationList{}
		if err := r.gardenClient.List(ctx, controllerInstallationList); err != nil {
			return reconcile.Result{}, err
		}

		for _, controllerInstallation := range controllerInstallationList.Items {
			if controllerInstallation.Spec.SeedRef.Name == seed.Name {
				return reconcile.Result{}, fmt.Errorf("cannot remove finalizer of Seed %q because still found at least one ControllerInstallation", seed.Name)
			}
		}

		return reconcile.Result{}, controllerutils.PatchRemoveFinalizers(ctx, r.gardenClient, seed, FinalizerName)
	}

	if !controllerutil.ContainsFinalizer(seed, FinalizerName) {
		return reconcile.Result{}, controllerutils.StrategicMergePatchAddFinalizers(ctx, r.gardenClient, seed, FinalizerName)
	}

	return reconcile.Result{}, nil
}

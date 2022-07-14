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

package shoot

import (
	"context"
	"fmt"

	logf "sigs.k8s.io/controller-runtime/pkg/log"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"
)

const statusLabelReconcilerName = "status-label"

func (c *Controller) shootStatusLabelAdd(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		c.log.Error(err, "Couldn't get key for object", "object", obj)
		return
	}
	c.shootStatusLabelQueue.Add(key)
}

func (c *Controller) shootStatusLabelUpdate(oldObj, newObj interface{}) {
	if c.filterShootForShootStatusLabel(newObj) {
		key, err := cache.MetaNamespaceKeyFunc(newObj)
		if err != nil {
			c.log.Error(err, "Couldn't get key for object", "object", newObj)
			return
		}
		c.shootStatusLabelQueue.Add(key)
	}
}

func (c *Controller) filterShootForShootStatusLabel(obj interface{}) bool {
	shoot, ok := obj.(*gardencorev1beta1.Shoot)
	if !ok {
		return false
	}

	status := string(shootpkg.ComputeStatus(shoot.Status.LastOperation, shoot.Status.LastErrors, shoot.Status.Conditions...))
	if currentStatus, ok := shoot.Labels[v1beta1constants.ShootStatus]; !ok || currentStatus != status {
		return true
	}

	return false
}

// NewShootStatusLabelReconciler creates a reconcile.Reconciler that updates a shoot's status label.
func NewShootStatusLabelReconciler(gardenClient client.Client) reconcile.Reconciler {
	return &shootStatusLabelReconciler{
		gardenClient: gardenClient,
	}
}

type shootStatusLabelReconciler struct {
	gardenClient client.Client
}

func (r *shootStatusLabelReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	shoot := &gardencorev1beta1.Shoot{}
	if err := r.gardenClient.Get(ctx, request.NamespacedName, shoot); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	// Compute shoot status
	status := string(shootpkg.ComputeStatus(shoot.Status.LastOperation, shoot.Status.LastErrors, shoot.Status.Conditions...))

	// Update the shoot status label if needed
	if currentStatus, ok := shoot.Labels[v1beta1constants.ShootStatus]; !ok || currentStatus != status {
		log.V(1).Info("Updating shoot status label", "status", status)

		patch := client.MergeFrom(shoot.DeepCopy())
		metav1.SetMetaDataLabel(&shoot.ObjectMeta, v1beta1constants.ShootStatus, status)
		if err := r.gardenClient.Patch(ctx, shoot, patch); err != nil {
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{}, nil
}

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

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/logger"
	shootpkg "github.com/gardener/gardener/pkg/operation/shoot"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func (c *Controller) shootStatusLabelAdd(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		return
	}
	c.shootStatusLabelQueue.Add(key)
}

func (c *Controller) shootStatusLabelUpdate(oldObj, newObj interface{}) {
	if c.filterShootForShootStatusLabel(newObj) {
		key, err := cache.MetaNamespaceKeyFunc(newObj)
		if err != nil {
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
		logger.Logger.Debugf("Shoot %s status label should be updated to %s", kutil.ObjectName(shoot), status)
		return true
	}

	return false
}

// NewShootStatusLabelReconciler creates a reconcile.Reconciler that updates a shoot's status label.
func NewShootStatusLabelReconciler(logger logrus.FieldLogger, gardenClient client.Client) reconcile.Reconciler {
	return &shootStatusLabelReconciler{
		logger:       logger,
		gardenClient: gardenClient,
	}
}

type shootStatusLabelReconciler struct {
	logger       logrus.FieldLogger
	gardenClient client.Client
}

func (r *shootStatusLabelReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	shoot := &gardencorev1beta1.Shoot{}
	if err := r.gardenClient.Get(ctx, request.NamespacedName, shoot); err != nil {
		if apierrors.IsNotFound(err) {
			r.logger.Infof("Object %q is gone, stop reconciling: %v", request.Name, err)
			return reconcile.Result{}, nil
		}
		r.logger.Infof("Unable to retrieve object %q from store: %v", request.Name, err)
		return reconcile.Result{}, err
	}

	// Compute shoot status
	status := string(shootpkg.ComputeStatus(shoot.Status.LastOperation, shoot.Status.LastErrors, shoot.Status.Conditions...))

	// Update the shoot status label if needed
	if currentStatus, ok := shoot.Labels[v1beta1constants.ShootStatus]; !ok || currentStatus != status {
		r.logger.Debugf("Updating shoot %s status label to %s", kutil.ObjectName(shoot), status)
		if err := r.updateShootStatusLabel(ctx, shoot, status); err != nil {
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{}, nil
}

func (r *shootStatusLabelReconciler) updateShootStatusLabel(ctx context.Context, shoot *gardencorev1beta1.Shoot, status string) error {
	patch := client.MergeFrom(shoot.DeepCopy())
	metav1.SetMetaDataLabel(&shoot.ObjectMeta, v1beta1constants.ShootStatus, status)
	return r.gardenClient.Patch(ctx, shoot, patch)
}

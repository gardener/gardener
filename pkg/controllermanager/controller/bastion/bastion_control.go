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

package bastion

import (
	"context"
	"time"

	operationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
	"github.com/gardener/gardener/pkg/logger"

	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func (c *Controller) bastionAdd(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		logger.Logger.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}
	c.bastionQueue.Add(key)
}

func (c *Controller) bastionUpdate(_, newObj interface{}) {
	c.bastionAdd(newObj)
}

func (c *Controller) bastionDelete(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		logger.Logger.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}
	c.bastionQueue.Add(key)
}

// NewBastionReconciler creates a new instance of a reconciler which reconciles Bastions.
func NewBastionReconciler(
	logger logrus.FieldLogger,
	gardenClient client.Client,
	maxLifetime time.Duration,
) reconcile.Reconciler {
	return &reconciler{
		logger:       logger,
		gardenClient: gardenClient,
		maxLifetime:  maxLifetime,
	}
}

type reconciler struct {
	logger       logrus.FieldLogger
	gardenClient client.Client
	maxLifetime  time.Duration
}

// Reconcile reacts to updates on Bastion resources and cleans up expired Bastions.
func (r *reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	logger := r.logger.WithField("bastion", request)

	bastion := &operationsv1alpha1.Bastion{}
	if err := r.gardenClient.Get(ctx, request.NamespacedName, bastion); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}

		logger.Infof("Unable to retrieve object from store: %v", err)
		return reconcile.Result{}, err
	}

	// do not reconcile anymore once the object is marked for deletion
	if bastion.DeletionTimestamp != nil {
		return reconcile.Result{}, nil
	}

	now := time.Now()

	// delete the bastion once it has expired
	if bastion.Status.ExpirationTimestamp != nil && now.After(bastion.Status.ExpirationTimestamp.Time) {
		logger.WithField("expired", bastion.Status.ExpirationTimestamp.Time).Info("Deleting expired bastion")
		return reconcile.Result{}, client.IgnoreNotFound(r.gardenClient.Delete(ctx, bastion))
	}

	// delete the bastion once it has reached its maximum lifetime
	if time.Since(bastion.CreationTimestamp.Time) > r.maxLifetime {
		logger.WithField("created", bastion.CreationTimestamp.Time).Info("Deleting bastion because it reached its maximum lifetime")
		return reconcile.Result{}, client.IgnoreNotFound(r.gardenClient.Delete(ctx, bastion))
	}

	// requeue when the Bastion expires or reaches its lifetime, whichever is sooner
	requeueAfter := time.Until(bastion.CreationTimestamp.Time.Add(r.maxLifetime))
	if bastion.Status.ExpirationTimestamp != nil {
		expiresIn := time.Until(bastion.Status.ExpirationTimestamp.Time)
		if expiresIn < requeueAfter {
			requeueAfter = expiresIn
		}
	}

	return reconcile.Result{
		RequeueAfter: requeueAfter,
	}, nil
}

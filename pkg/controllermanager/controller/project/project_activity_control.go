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

package project

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/cache"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const activityReconcilerName = "activity"

// NewActivityReconciler creates a new instance of a reconciler which reconciles the LastActivityTimestamp of a project, then it calls the stale reconciler.
func NewActivityReconciler(gardenClient client.Client) reconcile.Reconciler {
	return &projectActivityReconciler{
		gardenClient: gardenClient,
	}
}

type projectActivityReconciler struct {
	gardenClient client.Client
}

func (r *projectActivityReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	shoot := &gardencorev1beta1.Shoot{}
	if err := r.gardenClient.Get(ctx, client.ObjectKey{Namespace: request.Namespace, Name: request.Name}, shoot); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	return r.reconcile(ctx, log, shoot)
}

func (r *projectActivityReconciler) reconcile(ctx context.Context, log logr.Logger, obj client.Object) (reconcile.Result, error) {
	project, err := gutil.ProjectForNamespaceFromReader(ctx, r.gardenClient, obj.GetNamespace())
	if err != nil {
		if apierrors.IsNotFound(err) {
			// project is gone, nothing to do here
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("failed to get project for namespace %q: %w", obj.GetNamespace(), err)
	}

	if project.Status.LastActivityTimestamp != nil && obj.GetCreationTimestamp().UTC().Before(project.Status.LastActivityTimestamp.UTC()) {
		// not the newest object in this project, nothing to do
		return reconcile.Result{}, nil
	}

	timestamp := obj.GetCreationTimestamp()
	if project.Status.LastActivityTimestamp == nil {
		shoots := &gardencorev1beta1.ShootList{}
		if err := r.gardenClient.List(ctx, shoots, client.InNamespace(*project.Spec.Namespace)); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to list shoots in project namespace %q: %w", *project.Spec.Namespace, err)
		}
		for _, shoot := range shoots.Items {
			if shoot.GetCreationTimestamp().UTC().After(timestamp.UTC()) {
				timestamp = shoot.GetCreationTimestamp()
			}
		}
	}

	log.Info("Updating Project's lastActivityTimestamp", "lastActivityTimestamp", timestamp)
	if err := updateStatus(ctx, r.gardenClient, project, func() {
		project.Status.LastActivityTimestamp = &timestamp
	}); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed updating Project's lastActivityTimestamp: %w", err)
	}
	return reconcile.Result{}, nil
}

func (c *Controller) updateShootActivity(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		c.log.Error(err, "Couldn't get key for object", "object", obj)
		return
	}
	c.projectShootActivityQueue.Add(key)
}

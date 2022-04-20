// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const plantActivityReconcilerName = "plantActivity"

// NewPlantActivityReconciler creates a new instance of a reconciler which reconciles the LastActivityTimestamp of a project, then it calls the stale reconciler.
func NewPlantActivityReconciler(gardenClient client.Client) reconcile.Reconciler {
	return &projectPlantActivityReconciler{
		gardenClient: gardenClient,
	}
}

type projectPlantActivityReconciler struct {
	gardenClient client.Client
}

func (r *projectPlantActivityReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	plant := &gardencorev1beta1.Plant{}
	if err := r.gardenClient.Get(ctx, client.ObjectKey{Namespace: request.Namespace, Name: request.Name}, plant); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	return r.reconcile(ctx, log, plant)
}

func (r *projectPlantActivityReconciler) reconcile(ctx context.Context, log logr.Logger, obj client.Object) (reconcile.Result, error) {
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
		plants := &gardencorev1beta1.PlantList{}
		if err := r.gardenClient.List(
			ctx,
			plants,
			client.InNamespace(*project.Spec.Namespace),
		); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to list plants in project namespace %q: %w", *project.Spec.Namespace, err)
		}
		for _, plant := range plants.Items {
			if plant.GetCreationTimestamp().UTC().After(timestamp.UTC()) {
				timestamp = plant.GetCreationTimestamp()
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

func (c *Controller) updatePlantActivity(obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		c.log.Error(err, "Couldn't get key for object", "object", obj)
		return
	}
	c.projectPlantActivityQueue.Add(key)
}

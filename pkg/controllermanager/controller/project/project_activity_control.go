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
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const projectActivityReconcilerName = "project-activity"

// NewActivityReconciler creates a new instance of a reconciler which reconciles the LastActivityTimestamp of a project, then it calls the stale reconciler.
func NewActivityReconciler(gardenClient client.Client, clock clock.Clock) reconcile.Reconciler {
	return &projectActivityReconciler{
		gardenClient: gardenClient,
		clock:        clock,
	}
}

type projectActivityReconciler struct {
	gardenClient client.Client
	clock        clock.Clock
}

func (r *projectActivityReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	project := &gardencorev1beta1.Project{}
	if err := r.gardenClient.Get(ctx, request.NamespacedName, project); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	now := r.clock.Now()
	log.Info("Updating Project's lastActivityTimestamp", "lastActivityTimestamp", now)
	if err := updateStatus(ctx, r.gardenClient, project, func() {
		project.Status.LastActivityTimestamp = &metav1.Time{Time: now}
	}); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed updating Project's lastActivityTimestamp: %w", err)
	}

	return reconcile.Result{}, nil
}

func (c *Controller) projectActivityObjectAddDelete(ctx context.Context, obj interface{}, withLabel bool, addFunc bool) {
	objMeta, err := meta.Accessor(obj)
	if err != nil {
		return
	}

	if withLabel {
		// skip queueing if the object(secret or quota) doesn't have the "referred by a secretbinding" label
		if _, hasLabel := objMeta.GetLabels()[v1beta1constants.LabelSecretBindingReference]; !hasLabel {
			return
		}
	}

	if addFunc {
		// If the creationTimestamp of the object is less than 1 hour from current time,
		// skip it. This is to prevent unnecessary reconciliations in case of GCM restart.
		if objMeta.GetCreationTimestamp().Add(time.Hour).UTC().Before(c.clock.Now().UTC()) {
			return
		}
	}

	key := c.getProjectKey(ctx, objMeta.GetNamespace())
	if key == "" {
		return
	}

	c.projectActivityQueue.Add(key)
}

func (c *Controller) projectActivityObjectUpdate(ctx context.Context, oldObj, newObj interface{}, withLabel bool) {
	oldObjMeta, err := meta.Accessor(oldObj)
	if err != nil {
		return
	}
	newObjMeta, err := meta.Accessor(newObj)
	if err != nil {
		return
	}

	if withLabel {
		// skip queueing if the object(secret or quota) doesn't have the "referred by a secretbinding" label
		_, oldObjHasLabel := oldObjMeta.GetLabels()[v1beta1constants.LabelSecretBindingReference]
		_, newObjHasLabel := newObjMeta.GetLabels()[v1beta1constants.LabelSecretBindingReference]

		if !oldObjHasLabel && !newObjHasLabel {
			return
		}
	} else if oldObjMeta.GetGeneration() == newObjMeta.GetGeneration() {
		return
	}

	key := c.getProjectKey(ctx, newObjMeta.GetNamespace())
	if key == "" {
		return
	}

	c.projectActivityQueue.Add(key)
}

func (c *Controller) getProjectKey(ctx context.Context, namespace string) string {
	project, err := gutil.ProjectForNamespaceFromReader(ctx, c.gardenClient, namespace)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// project is gone, nothing to do here
			return ""
		}
		c.log.Error(err, "Failed to get project for namespace", "namespace", namespace)
		return ""
	}

	key, err := cache.MetaNamespaceKeyFunc(project)
	if err != nil {
		c.log.Error(err, "Couldn't get key for object", "object", project)
		return ""
	}

	return key
}
